# Skills Generation Pipeline

[简体中文](project-generation-guide.md)

## Overview

Skills Seed generation has two stages:

1. Learning creates internal knowledge assets
2. Generation renders those assets into AI skills

```text
Code / Git history
  -> Pattern
  -> ProjectProfile
  -> ProjectSpec
  -> SKILL.md + references/
```

## Core Artifacts

### Internal Knowledge Assets

```text
.skills-seed/memory/project.db
.skills-seed/memory/project-profile.json
.skills-seed/memory/project-spec.json
```

Workspace mode also has:

```text
.skills-seed/memory/workspace-profile.json
.skills-seed/memory/workspace-spec.json
```

### Generated Skills

```text
<skills-output>/
  SKILL.md
  agents/
  references/
    project-overview.md
    project-spec.md
    patterns/*.md
    examples/*.md
```

Workspace mode generates a root skill and one skill for each child project.

## Initialization

```bash
skills-seed init --mode project
skills-seed init --mode workspace
```

Initialization creates:

```text
.skills-seed/
  config.yaml
  memory/
  logs/
  prompts/
```

`project.mode` determines the learning and generation path. It cannot be switched directly after learning or generation starts.

## Learn From Current Code

```bash
skills-seed learn current
```

Project mode flow:

```text
runLearnCurrent
  -> Resolve project root, language, and focus paths
  -> Compare md5 fingerprints for normal project files and build incremental focus paths
  -> AnalyzerService.AnalyzeCodebaseFullWithOptions
  -> Agent.AnalyzeCurrentCodebase
  -> Save patterns
  -> Create or refresh project-profile.json according to --profile
  -> Print follow-up steps and token usage
```

After a successful run, `learn current` stores md5 fingerprints for normal project files in `.skills-seed/memory/project.db`. The next run compares fingerprints first:

- no added, modified, or deleted learnable files: skip pattern learning and profile refresh
- added or modified files: use only those files as incremental focus paths
- deleted files only: skip pattern learning and refresh the profile from the existing profile

Generated skills output directories are excluded by default and do not need to be listed manually in `exclude`.

`--profile`:

- `auto`: default. Refreshes for first/full learning; skips narrow learning when possible
- `skip`: learn patterns only
- `refresh`: refresh the profile

Examples:

```bash
skills-seed learn current
skills-seed learn current --focus internal/service --profile skip
skills-seed learn current --focus internal/domain --profile refresh
```

`learn current` does not write `SKILL.md` directly. Run:

```bash
skills-seed generate-skills
```

In project mode, token usage is the final learning log line. In workspace mode, child projects are learned concurrently, and each child project's token usage is printed after that child project's completion log with the child project name.

## Learn From Git History

```bash
skills-seed learn history --limit=50
skills-seed learn history --since=30d
```

Flow:

```text
LearnerService.Learn
  -> GitRepository.GetCommits
  -> CommitAnalysisTracker.IsCommitAnalyzed
  -> Filter already learned commits
  -> Agent.BatchLearnFromCommits
  -> PatternRepo.FindSimilar / Save
  -> CommitAnalysisTracker.MarkCommitAnalyzed
```

Analyzed commits are stored in BoltDB metadata as `analyzed_commits`. Later history learning skips them by commit hash.

If an AI call fails for a batch, that batch is not marked as analyzed and will be retried later.

## Project Profile

The project profile is the durable project facts input:

```text
.skills-seed/memory/project-profile.json
```

Created or refreshed by:

```bash
skills-seed learn current
skills-seed profile refresh
```

`profile refresh` only refreshes the profile. It does not learn patterns.

The profile contains:

- Project summary
- Architecture
- Key modules
- Dependencies and data flow
- Common utilities
- Business methods
- Config and framework patterns

## Project Spec

The project spec is the development contract that AI should read before editing code:

```text
.skills-seed/memory/project-spec.json
references/project-spec.md
```

`generate-skills` builds the spec from the project profile and learned patterns. It includes:

- Module and layer boundaries
- Pattern rules
- Config and framework rules
- Change touchpoints
- Workspace child-project scope

Workspace child projects keep their own `project-spec.json` and `references/project-spec.md` in each child repo's `.skills-seed`.

## Generate Skills

```bash
skills-seed generate-skills
```

Project mode flow:

```text
GeneratorService.GenerateSkills
  -> Resolve output.skills_paths[agent.provider]
  -> Read patterns
  -> Read project-profile.json
  -> In generation.mode=template, build summaries directly from learned data
  -> In generation.mode=ai, call Agent.GenerateSkillsSummary for summary merging
  -> Generate project-spec.json
  -> Render SKILL.md
  -> Render references/
```

If there are no patterns, generation is skipped. If the project profile is missing, the command asks you to run `learn current` or `profile refresh` first.

`generation.mode` defaults to `template`, so generation does not make an extra AI call. The learning stage has already produced patterns, the project profile, and the project spec, making template mode better for stable batch generation and workspace child repos. Set this when you want AI to compress or polish a large pattern set before rendering:

```yaml
generation:
  mode: "ai"
```

## Workspace Generation

Workspace mode flow:

```text
skills-seed generate-skills
  -> Enter each independent Git child repo listed in workspace.projects
  -> Generate each child skill using that child's own .skills-seed/config.yaml
  -> If a child target SKILL.md has no generated-by marker, treat it as manual and skip overwrite
  -> Return to the workspace root and generate the root skill
  -> Read child provider, output.skills_paths, and generated skill summary
  -> Generate workspace-overview.md and cross-project-rules.md
```

Root skill responsibilities:

- Decide which child project a change belongs to
- Tell AI agents to load the relevant child skill
- Describe shared, contract, and infra paths
- Describe cross-project change order and risk

Child skill responsibilities:

- Describe project architecture
- Describe project boundaries and rules
- Provide project patterns and examples

Child skills are generated from each child repo's own config and data. You can run `skills-seed learn current` and `skills-seed generate-skills` inside a child repo, or run `skills-seed generate-skills` from the workspace root to orchestrate every child first. Child profiles, patterns, md5 file fingerprints, and skills stay in each child's own `.skills-seed`; the workspace root only reads child config and generated skill content at the end to build routing.

## Template Selection

Skills templates are resolved by provider first, then common fallback:

```text
embedfs/templates/skills/<provider>/
embedfs/templates/skills/common/
```

Workspace root templates live in:

```text
embedfs/templates/skills/common/workspace/
```

Prompt templates:

```text
embedfs/templates/prompts/common/
embedfs/templates/prompts/project/
embedfs/templates/prompts/workspace/
```

All templates are included in hash calculation. Generated files include the program version and template hash.

## Output Path

Default output path:

```text
output.skills_paths[agent.provider]
```

Workspace mode also uses only the current `agent.provider` output path; the root skill is written under the workspace root, and child skills are written under each child project.

Example:

```yaml
agent:
  provider: "codex"

output:
  skills_paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"
```

Temporary override:

```bash
skills-seed generate-skills --output .agents/skills/my-project
```

Relative paths are resolved from the project root.

## Pattern Merge

Merge similar patterns explicitly:

```bash
skills-seed patterns merge
skills-seed generate-skills
```

`generate-skills --merge` remains compatible, but `patterns merge` is preferred.

## Recommended Flows

### New Single Project

```bash
skills-seed init --mode project
skills-seed learn current
skills-seed generate-skills
```

### Project With Existing Git History

```bash
skills-seed init --mode project
skills-seed learn history --limit=50
skills-seed profile refresh
skills-seed patterns merge
skills-seed generate-skills
```

### Workspace

```bash
skills-seed init --mode workspace
# Review workspace.projects in .skills-seed/config.yaml
skills-seed learn current
skills-seed generate-skills
```

### Daily Iteration

```bash
skills-seed learn current --focus <path> --profile skip
skills-seed learn history --since=7d
skills-seed generate-skills
skills-seed check
```
