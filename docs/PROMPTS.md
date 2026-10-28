# Prompt Templates

Runtime prompt templates live under `embedfs/templates/prompts/loader/`. They are rendered by `internal/prompts.Loader` and sent through both Claude and Codex agent implementations unless noted otherwise.

`embedfs/templates/prompts/append/knowledge-goal-contract.txt.tmpl` is prepended to every runtime prompt that can create long-lived project or workspace knowledge. It is a compact, stable projection of `docs/ULTIMATE_GOAL.md`: future-agent decision value, authority boundaries, evidence requirements, scope, and risk must survive every learning stage. Stage templates remain responsible for task-specific discovery and output rules.

The runtime goal contract has a single source of truth under `embedfs/templates/prompts/append/`; `docs/ULTIMATE_GOAL.md` is an acceptance and design document and is never loaded as prompt text. Project-specific facts are passed through typed runtime inputs instead of allowing the model to generate an unconstrained replacement prompt.

`embedfs/templates/prompts/append/output-contract-guard.txt.tmpl` is not called as a standalone task prompt. `Loader.Render` / `RenderForRuntimeTask` append it to every runtime prompt, after any project context fragments, so final JSON shape, escaping, stable output, and language rules are enforced consistently.

For knowledge-producing calls, the runtime also writes current user Rules and Workflows into a typed `maintained-guidance.json` input and prepends a shared guidance contract. Rules constrain source inference and command semantics; Workflows remain user-owned procedures and cannot become source-learned patterns or authority rules. Persistent files under `.skills-seed/context/` are a separate non-authoritative background layer for terminology and facts unavailable in source.

Files under `embedfs/templates/prompts/append/` are reusable mandatory fragments selected by prompt name in `internal/prompts`; their directory name is historical and does not imply that every fragment is placed at the end. The knowledge goal is prepended to knowledge-producing prompts, while the output-contract guard is appended globally. Current-code candidate normalization is coordinated by `internal/service/patternnorm`, with AI limited to semantic merge proposals.

| Template | Main production callers | Scenario |
|---|---|---|
| `learning-semantic-screen` | `CodexAgent.ScreenSemanticGaps`, `ClaudeAgent.ScreenSemanticGaps` | Compact structural pre-screen when local candidates require multiple evidence batches; excludes only clearly low-value entries and runs within the shared Agent concurrency budget. |
| `learning-supplement` | Shared current-code analysis, delta analysis and independent review | Default local-first path: bounded inline evidence and request data; no repository tools. |
| `learning-pack-analyze` | `CodexAgent.AnalyzeCurrentCodebaseBatch`, `ClaudeAgent.AnalyzeCurrentCodebaseBatch` | Initial current-code learning for one or more evidence packs in an isolated runtime call. |
| `learning-delta-pack-analyze` | `CodexAgent.AnalyzeCurrentDeltaBatch`, `ClaudeAgent.AnalyzeCurrentDeltaBatch` | Diff-anchored incremental learning in an isolated runtime call; output must be triggered by changed hunks. |
| `learning-pattern-normalize` | `CodexAgent.NormalizePatterns`, `ClaudeAgent.NormalizePatterns` | Propose semantic merges between new candidates and related stored patterns. Local normalization owns source-ID validation, ownership resolution, recovery, and storage. |
| | `learning-authority-extract` | `CodexAgent.ExtractAuthority`, `ClaudeAgent.ExtractAuthority` | Extract explicit constraints from the deterministic authority section catalog without mixing them into project-map analysis. |
| `learning-knowledge-review` | `CodexAgent.ReviewKnowledge`, `ClaudeAgent.ReviewKnowledge` | Independently accept, narrow, or reject every source-learning candidate before semantic normalization; the runtime constrains each response to the current candidate IDs and exact one-to-one decision count. |
| `core-workspace-profile` | `CodexAgent.AnalyzeWorkspaceProfile`, `ClaudeAgent.AnalyzeWorkspaceProfile` | Learn workspace-level project relationships and routing facts. |
| `core-workspace-spec` | `CodexAgent.AnalyzeWorkspaceSpec`, `ClaudeAgent.AnalyzeWorkspaceSpec` | Generate workspace-level executable development constraints. |
| `core-user-pattern` | `CodexAgent.UserDefinePattern`, `ClaudeAgent.UserDefinePattern` | Convert user-provided pattern descriptions into structured pattern output. |

Automatic supplements disable repository tools and supply complete bounded evidence inline. Claude uses low effort for these calls; independent review remains mandatory before candidate admission. Other explicit learning tasks may retain read-only repository tools for their referenced inputs. Cross-stage memory is explicit runtime data, not command-wide hidden conversation state.

## Redundancy Status

No loader prompt template is currently unused. Every file under `embedfs/templates/prompts/loader/` has a production render path in the agent layer.

Current-code learning is local-first. CodeGraph indexes files, symbols, calls, and impact locally; deterministic selection keeps only changed and high-value business gaps. Related gaps share one evidence call until the evidence byte budget is reached, so the number of questions per call grows with the project and is never fixed at one. AI receives only the bounded evidence packet, then an independent review accepts or rejects business candidates. Local normalization validates source ownership and writes results; generic implementation details are discarded.

The local selector prioritizes changed evidence of accepted knowledge, then callable entries on actual CodeGraph call paths. It keeps independent public capabilities and collapses downstream candidates reached from a reliable entry while retaining their structural navigation. When several full evidence batches remain, a compact pre-screen can exclude clearly generic entries; ambiguous entries still proceed to source analysis and independent review.

Selection and prompts must remain repository-independent: no project-name allowlists, preferred framework paths, or domain-specific exceptions. Persistent project facts and terminology belong in `.skills-seed/context/`; temporary priorities belong in `--context`. Both reach supplement calls without changing the shared policy. Context guides interpretation and cannot establish a source claim without evidence or replace user-maintained Rules and Workflows.

CodeGraph stores local navigation facts such as files, symbols, modules, and dependencies; it does not own reusable business entries or authoritative rules. Rule and Workflow files remain user-maintained inputs and are projected deterministically. There is no persisted derived `project-spec.json` fact store.

Testing, deployment, and acceptance procedures are not learned from repository source. Their commands, steps, and ordering come from user-maintained Workflows, which are copied into generated Skills and routed by `SKILL.md`. Explicit command permissions or prohibitions from authoritative project files remain project safety rules.
