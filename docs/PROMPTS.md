# Prompt Templates

Runtime prompt templates live under `embedfs/templates/prompts/loader/`. They are rendered by `internal/prompts.Loader` and sent through both Claude and Codex agent implementations unless noted otherwise.

`embedfs/templates/prompts/append/output-contract-guard.txt.tmpl` is not called as a standalone task prompt. `Loader.Render` / `RenderForRuntimeTask` append it to every runtime prompt, after any project context fragments, so final JSON shape, escaping, stable output, and language rules are enforced consistently.

Files under `embedfs/templates/prompts/append/` are reusable fragments selected by prompt name in `internal/prompts`. At present only the shared output-contract guard is appended globally; current-code candidate normalization is coordinated by `internal/service/patternnorm`, with AI limited to merge proposals.

| Template | Main production callers | Scenario |
|---|---|---|
| `learning-candidate-select` | `CodexAgent.SelectLearningCandidates`, `ClaudeAgent.SelectLearningCandidates` | Narrow large current-code candidate file sets before evidence-pack planning. The output contract requires selected paths to come from the candidate list and preserve required paths. |
| `learning-pack-plan` | `CodexAgent.PlanLearningAgenda`, `ClaudeAgent.PlanLearningAgenda` | Split current candidate files into self-contained evidence packs in one independent runtime call. |
| `learning-pack-analyze` | `CodexAgent.AnalyzeCurrentCodebaseBatch`, `ClaudeAgent.AnalyzeCurrentCodebaseBatch` | Initial current-code learning for one or more evidence packs in an isolated runtime call. |
| `learning-delta-pack-analyze` | `CodexAgent.AnalyzeCurrentDeltaBatch`, `ClaudeAgent.AnalyzeCurrentDeltaBatch` | Diff-anchored incremental learning in an isolated runtime call; output must be triggered by changed hunks. |
| `learning-pattern-normalize` | `CodexAgent.NormalizePatterns`, `ClaudeAgent.NormalizePatterns` | Optimize current-code candidate pattern merging by returning source ownership and canonicalization proposals; local normalization remains authoritative for validation and storage. |
| `learning-profile-refresh` | `CodexAgent.RefreshProjectProfile`, `ClaudeAgent.RefreshProjectProfile` | Sync the complete project profile in a bounded profile-only runtime call when current analysis recommends it. |
| `core-workspace-profile` | `CodexAgent.AnalyzeWorkspaceProfile`, `ClaudeAgent.AnalyzeWorkspaceProfile` | Learn workspace-level project relationships and routing facts. |
| `core-workspace-spec` | `CodexAgent.AnalyzeWorkspaceSpec`, `ClaudeAgent.AnalyzeWorkspaceSpec` | Generate workspace-level executable development constraints. |
| `core-user-pattern` | `CodexAgent.UserDefinePattern`, `ClaudeAgent.UserDefinePattern` | Convert user-provided pattern descriptions into structured pattern output. |
| `core-workflow-optimize` | `CodexAgent.OptimizeWorkflow`, `ClaudeAgent.OptimizeWorkflow` | Normalize a user workflow description into maintainable Markdown while preserving task-specific structure. |

Planning and learning stages retain read-only repository tools because their prompts intentionally reference runtime candidate lists, structural context, diffs, and repository paths. Cross-stage memory is explicit runtime data, not command-wide hidden conversation state.

## Redundancy Status

No loader prompt template is currently unused. Every file under `embedfs/templates/prompts/loader/` has a production render path in the agent layer.

Current-code learning uses independent runtime calls: planning creates self-contained evidence packs, each pack batch is analyzed in its own call, diff learning uses isolated diff-pack calls, and candidate patterns pass through AI merge optimization before local validation and storage.
