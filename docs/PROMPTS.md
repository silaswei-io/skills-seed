# Prompt Templates

Runtime prompt templates live under `embedfs/templates/prompts/loader/`. They are rendered by `internal/prompts.Loader` and sent through both Claude and Codex agent implementations unless noted otherwise.

`embedfs/templates/prompts/append/output-contract-guard.txt.tmpl` is not called as a standalone task prompt. `Loader.Render` / `RenderForRuntimeTask` append it to every runtime prompt, after any project context fragments, so final JSON shape, escaping, stable output, and language rules are enforced consistently.

Other files under `embedfs/templates/prompts/append/` are reusable fragments selected by prompt name in `internal/prompts`. `pattern-curation-rules` and `pattern-abstraction-rules` are appended to `learning-global-curate` so duplicate handling, scoped confidence, source ownership, anti-generalization rules, abstraction level, and stable naming rules stay consistent.

| Template | Main production callers | Scenario |
|---|---|---|
| `learning-conversation-start` | `CodexAgent.StartLearningSession`, `ClaudeAgent.StartLearningSession` | Start one short stage conversation. Runtime stage names distinguish planning, pack analysis, delta pack analysis, profile sync, and global curation. |
| `learning-candidate-select` | `LearningSession.SelectLearningCandidates` | Narrow large current-code candidate file sets before evidence-pack planning. The output contract requires selected paths to come from the candidate list and preserve required paths. |
| `learning-pack-plan` | `LearningSession.PlanLearningAgenda` | Split current candidate files into self-contained evidence packs inside the planning conversation. |
| `learning-pack-analyze` | `LearningSession.AnalyzeCurrentCodebaseBatch` | Initial current-code learning for one or more evidence packs in an isolated pack-analysis conversation. |
| `learning-delta-pack-analyze` | `LearningSession.AnalyzeCurrentDeltaBatch` | Diff-anchored incremental learning in an isolated delta-pack conversation; output must be triggered by changed hunks. |
| `learning-profile-refresh` | `LearningSession.RefreshProjectProfile` | Sync the complete project profile in a bounded profile-only conversation when current analysis recommends it. |
| `learning-global-curate` | `LearningSession.CuratePatterns` | Curate this run's pack-level patterns against existing patterns in a separate global curation conversation before local hydration and storage. |
| `core-workspace-profile` | `CodexAgent.AnalyzeWorkspaceProfile`, `ClaudeAgent.AnalyzeWorkspaceProfile` | Learn workspace-level project relationships and routing facts. |
| `core-workspace-spec` | `CodexAgent.AnalyzeWorkspaceSpec`, `ClaudeAgent.AnalyzeWorkspaceSpec` | Generate workspace-level executable development constraints. |
| `core-user-pattern` | `CodexAgent.UserDefinePattern`, `ClaudeAgent.UserDefinePattern` | Convert user-provided pattern descriptions into structured pattern output. |
| `core-workflow-optimize` | `CodexAgent.OptimizeWorkflow`, `ClaudeAgent.OptimizeWorkflow` | Normalize a user workflow description into structured workflow steps. |

Planning and learning stages retain read-only repository tools because their prompts intentionally reference runtime candidate lists, structural context, diffs, and repository paths. Cross-stage memory is explicit runtime data, not a command-wide hidden conversation.

## Redundancy Status

No loader prompt template is currently unused. Every file under `embedfs/templates/prompts/loader/` has a production render path in the agent layer.

Current-code learning uses segmented short conversations: planning creates self-contained evidence packs, each pack is analyzed in its own conversation, diff learning uses isolated diff-pack conversations, and final curation runs in a separate global curation conversation.
