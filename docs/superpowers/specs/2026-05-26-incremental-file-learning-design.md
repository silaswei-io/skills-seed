# Incremental File Learning Design

## Background

`learn history` already avoids repeated analysis by tracking analyzed commit hashes through `CommitAnalysisTracker`. `learn current` does not have an equivalent file-level tracker. It sends the current codebase to `AnalyzeCodebaseFullWithOptions`, then relies on `SavePatterns` and `PatternRepository.FindSimilar` to merge duplicate patterns after the AI call.

That means unchanged files can be analyzed repeatedly, profile refreshes can repeat work, and generated skills files can be reintroduced into later learning runs if the output directory is inside the project.

## Goals

- Track analyzed normal files by content hash and run `learn current` incrementally.
- Apply the same incremental decision to both pattern extraction and project profile refresh.
- Keep workspace projects isolated so the same relative file path in different child projects does not collide.
- Avoid learning from generated skills output and project memory.
- Reduce duplicate patterns by giving current-code learning the same known-pattern context used by history learning.
- Update user-facing documentation for the new behavior and generated-skill exclusions.

## Non-Goals

- Rebuild pattern storage around exact source-file ownership.
- Delete or rewrite existing patterns when a source file is deleted.
- Add vector search or complex semantic similarity for pattern matching.
- Change `learn history` commit filtering semantics.

## Recommended Approach

Add a file analysis tracker backed by the existing project memory store. Before `learn current` calls the AI, it computes fingerprints for learnable files and compares them with the previous run. Only added, modified, or deleted paths drive the current run. If there are no file changes, both pattern extraction and profile refresh are skipped.

The implementation should keep the existing `FocusPaths` mechanism as the boundary between command orchestration and analyzer behavior. Changed files become focus paths for both:

- `AnalyzerService.AnalyzeCodebaseFullWithOptions`
- `AnalyzerService.AnalyzeProjectFullWithOptions`

For profile refresh, the command should pass the existing profile when available so the AI returns a complete updated profile while focusing on changed paths.

## Data Model

Add a domain model similar to:

```go
type FileAnalysisRecord struct {
    ProjectID      string `json:"project_id,omitempty"`
    ScopePath      string `json:"scope_path,omitempty"`
    Path           string `json:"path"`
    Hash           string `json:"hash"`
    HashAlgorithm  string `json:"hash_algorithm"`
    Size           int64  `json:"size"`
    ModTime        string `json:"mod_time"`
    Source         string `json:"source"`
    LastAnalyzedAt string `json:"last_analyzed_at"`
}
```

Use `md5` as the hash algorithm to match the requested behavior. The tracker key should include scope:

```text
<projectID>\x00<scopePath>\x00<relativePath>
```

For single-project mode, `projectID` and `scopePath` can be empty. For workspace mode, use the child project ID and configured child path.

## Repository Interface

Add a small repository interface instead of expanding pattern storage responsibilities:

```go
type FileAnalysisTracker interface {
    GetAnalyzedFile(ctx context.Context, scope FileAnalysisScope, path string) (*FileAnalysisRecord, error)
    ListAnalyzedFiles(ctx context.Context, scope FileAnalysisScope) ([]FileAnalysisRecord, error)
    SaveAnalyzedFiles(ctx context.Context, records []FileAnalysisRecord) error
    DeleteAnalyzedFiles(ctx context.Context, scope FileAnalysisScope, paths []string) error
}
```

The BoltDB implementation can live next to the existing pattern repository and use a dedicated bucket. It should not store file contents.

## Incremental Flow

### Single Project

1. Resolve project root, project name, language, focus paths, and existing profile as today.
2. Build the learnable file set.
3. Apply built-in excludes, configured excludes, and generated-skill excludes.
4. Compute md5 for each learnable file.
5. Compare with tracker records:
   - new path: changed
   - same path, different md5: changed
   - tracker path missing from current file set: deleted
   - same path and same md5: unchanged
6. If no added, modified, or deleted paths exist:
   - skip `AnalyzeCodebaseFullWithOptions`
   - skip `AnalyzeProjectFullWithOptions`
   - keep runtime state updates conservative; do not mark new file records
7. If changes exist:
   - pass changed and deleted relative paths as focus paths
   - call pattern extraction
   - save patterns
   - call project profile refresh with existing profile
   - save profile
   - update tracker only after successful pattern and profile persistence

### Workspace

Run the same flow inside each child project task. Each project computes fingerprints relative to its own root but stores records under its workspace scope. Child projects with no file changes should skip their AI calls independently.

Workspace-level profile/spec generation is not part of this feature. It can continue to use the existing workspace config and saved child profiles.

## Learnable File Selection

Start from tracked project files, preferably `git ls-files`, because it avoids build artifacts and untracked local noise. If a future non-git mode is added, it can use a filesystem walker with the same exclusions.

Exclude at minimum:

- `.git/**`
- `.skills-seed/**`
- `vendor/**`
- `node_modules/**`
- generated files already excluded by config, such as `**/*.pb.go` and `**/*.gen.go`
- configured `exclude` entries
- provider-specific skills output path from `output.skills_paths`
- fallback provider skills locations:
  - `.claude/skills/**`
  - `.agents/skills/**`

The generated-skills exclusion is required to prevent a feedback loop where generated `SKILL.md` and reference files become input for future learning.

## Pattern Deduplication

Keep the current save-time merge behavior and add an analysis-time hint:

- Add `KnownPatternsJSON` and `KnownPatternsCount` to `AnalyzeCurrentCodebaseRequest`.
- Marshal known patterns using the same reduced representation used by history learning.
- Update `init-skills` prompts so current-code learning sees existing pattern names, categories, rules, confidence, source, and business method metadata.
- Instruct the agent to avoid re-emitting an existing rule under a new name. If a changed file adds material evidence to an existing rule, the output should reuse the existing concept or provide a narrowly scoped improvement.

Save-time duplicate handling should still run because AI output is not guaranteed. A small improvement can normalize names before `IsSimilar`, but large semantic matching is out of scope for this change.

## Generated Skills Interaction

`generate skills` should not write tracker records. Instead, learning should exclude generated locations. This keeps generation and learning loosely coupled and avoids surprising state mutations during generation.

For workspace mode, child project skills paths can come from either the root config or a child `.skills-seed/config.yaml`. The exclusion builder should resolve all known configured paths for the project being scanned where that config is available.

## Error Handling

- If hashing a file fails, log a warning and skip that file for the run.
- If tracker reads fail, fail the incremental preparation step rather than falling back to full analysis silently.
- If tracker writes fail after AI work succeeds, return an error so the user knows the next run may repeat analysis.
- If pattern extraction succeeds but profile refresh fails, do not update file records. The next run should retry the full changed set.
- If there are deleted files only, skip pattern extraction but still refresh the profile with deleted paths as focus paths when a profile exists.

## User Output

Add concise CLI messages for:

- number of changed files detected
- number of unchanged files skipped
- no-change skip
- generated skills paths excluded

Diagnostics should include counts for added, modified, deleted, unchanged, and skipped files.

## Documentation Updates

Update at least:

- `README.md`
- `README.en.md`
- `docs/project-generation-guide.md`
- `docs/project-generation-guide.en.md`
- config templates under `embedfs/templates/config/`

The docs should explain:

- `learn current` is file-hash incremental after the first successful run.
- Both pattern extraction and profile refresh are skipped when no learnable files changed.
- Generated skills directories are excluded from learning by default.
- Updating source files inside a workspace child project only re-learns that child project.
- Clearing project memory or deleting the tracker state forces a future full current-code learning pass.

## Test Plan

- Unit-test md5 fingerprint creation and path normalization.
- Unit-test file tracker save/list/delete behavior in BoltDB.
- Unit-test generated-skills exclusion from configured provider paths and fallback paths.
- Unit-test single-project `learn current` skips AI calls when hashes are unchanged.
- Unit-test single-project `learn current` calls both pattern extraction and profile refresh when one file changes.
- Unit-test deleted-file-only runs skip pattern extraction but refresh profile.
- Unit-test workspace scopes do not collide for equal relative paths in different child projects.
- Unit-test current-code prompt data includes known pattern JSON.
- Regression-test existing history learning commit filtering.

## Implementation Decisions

- Do not add a `learn current --full` flag in the initial implementation. Users can force a full current-code learning pass by clearing project memory or deleting the file tracker state.
- Keep generated-skill exclusion built in for the initial implementation. Document the behavior instead of adding new config.
