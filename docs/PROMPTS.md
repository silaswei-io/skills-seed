# Prompt Templates

Runtime prompt templates live under `embedfs/templates/prompts/loader/`. They are rendered by `internal/prompts/loader.Loader` and sent through both Claude and Codex agent implementations unless noted otherwise.

`embedfs/templates/prompts/append/output-contract-guard.txt.tmpl` is not called as a standalone task prompt. `Loader.Render` / `RenderForRuntimeTask` append it to every runtime prompt, after any project context fragments, so final JSON shape, escaping, stable output, and language rules are enforced consistently.

Other files under `embedfs/templates/prompts/append/` are reusable fragments selected by prompt name in `internal/prompts/loader`. For example, `pattern-abstraction-rules` is appended to pattern-learning and curation prompts so abstraction level and stable naming rules stay consistent, while `pattern-evidence-rules` centralizes code-evidence snippet constraints.

| Template | Main production callers | Scenario |
|---|---|---|
| `analysis-plan` | `CodexAgent.PlanAnalysisUnits`, `ClaudeAgent.PlanAnalysisUnits` | Split current learn/sync candidate files into analysis units before current-code learning. |
| `file-select` | `CodexAgent.SelectFiles`, `ClaudeAgent.SelectFiles` | AI relevant-file selection before current-code learning; local validation still enforces candidate boundaries and focus paths. |
| `fix-generate` | `CodexAgent.GenerateFixes`, `ClaudeAgent.GenerateFixes` | Generate fixes for check/analyze findings. |
| `learn-analyze` | `CodexAgent.AnalyzeCode`, `ClaudeAgent.AnalyzeCode` | Analyze files for `check`-style issue detection. |
| `learn-batch` | `CodexAgent.LearnFromCommit`, `CodexAgent.BatchLearnFromCommits`, Claude equivalents | Learn pattern candidates from git commit history batches. |
| `pattern-curate` | `CodexAgent.CuratePatterns`, `ClaudeAgent.CuratePatterns` | Decide canonical text and real source IDs for current-code candidates; source-owned fields are hydrated locally before storage. |
| `pattern-learn-current` | `CodexAgent.AnalyzeCurrentCodebase`, `ClaudeAgent.AnalyzeCurrentCodebase` | Learn patterns from one current-code analysis unit. |
| `pattern-learn-current-batch` | `CodexAgent.AnalyzeCurrentCodebaseBatch`, `ClaudeAgent.AnalyzeCurrentCodebaseBatch` | Learn patterns from one or more current-code analysis units in a single agent call. |
| `project-profile` | `CodexAgent.AnalyzeProject`, `ClaudeAgent.AnalyzeProject` | Build or refresh a project profile from structure, focused paths, and source evidence. |
| `skill-workspace-profile` | `CodexAgent.AnalyzeWorkspaceProfile`, `ClaudeAgent.AnalyzeWorkspaceProfile` | Learn workspace-level project relationships and routing facts. |
| `skill-workspace-spec` | `CodexAgent.AnalyzeWorkspaceSpec`, `ClaudeAgent.AnalyzeWorkspaceSpec` | Generate workspace-level executable development constraints. |
| `user-define-pattern` | `CodexAgent.UserDefinePattern`, `ClaudeAgent.UserDefinePattern` | Convert user-provided pattern descriptions into structured pattern output. |
| `workflow-optimize` | `CodexAgent.OptimizeWorkflow`, `ClaudeAgent.OptimizeWorkflow` | Normalize a user workflow description into structured workflow steps. |

File selection, analysis-unit planning, and pattern curation are prompt-only tasks because their prompts already contain all required inputs. Claude runs them with repository tools disabled, preventing multi-turn repository reads from inflating token usage. Current-code analysis and project profiling still retain read-only source tools because their prompts intentionally reference repository files.

## Redundancy Status

No loader prompt template is currently unused. Every file under `embedfs/templates/prompts/loader/` has a production render path in the agent layer.

The templates most worth future compression are `pattern-learn-current` and `learn-batch`; both still contain long learning-quality rules and verbose JSON schemas. Smaller prompts such as `workflow-optimize`, `fix-generate`, `learn-analyze`, `analysis-plan`, and workspace prompts are already compact enough that compression has lower value and higher risk.
