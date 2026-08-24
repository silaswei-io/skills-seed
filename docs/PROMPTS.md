# Prompt Templates

Runtime prompt templates live under `embedfs/templates/prompts/loader/`. They are rendered by `internal/prompts.Loader` and sent through both Claude and Codex agent implementations unless noted otherwise.

`embedfs/templates/prompts/append/knowledge-goal-contract.txt.tmpl` is prepended to every runtime prompt that can create long-lived project or workspace knowledge. It is a compact, stable projection of `docs/ULTIMATE_GOAL.md`: future-agent decision value, authority boundaries, evidence requirements, scope, and risk must survive every learning stage. Stage templates remain responsible for task-specific discovery and output rules.

The runtime goal contract has a single source of truth under `embedfs/templates/prompts/append/`; `docs/ULTIMATE_GOAL.md` is an acceptance and design document and is never loaded as prompt text. Project-specific facts are passed through typed runtime inputs instead of allowing the model to generate an unconstrained replacement prompt.

`embedfs/templates/prompts/append/output-contract-guard.txt.tmpl` is not called as a standalone task prompt. `Loader.Render` / `RenderForRuntimeTask` append it to every runtime prompt, after any project context fragments, so final JSON shape, escaping, stable output, and language rules are enforced consistently.

For knowledge-producing calls, the runtime also writes current user Rules and Workflows into a typed `maintained-guidance.json` input and prepends a shared guidance contract. Rules constrain source inference and command semantics; Workflows remain user-owned procedures and cannot become source-learned patterns or authority rules. Persistent files under `.skills-seed/context/` are a separate non-authoritative background layer for terminology and facts unavailable in source.

Files under `embedfs/templates/prompts/append/` are reusable mandatory fragments selected by prompt name in `internal/prompts`; their directory name is historical and does not imply that every fragment is placed at the end. The knowledge goal is prepended to knowledge-producing prompts, while the output-contract guard is appended globally. Current-code candidate normalization is coordinated by `internal/service/patternnorm`, with AI limited to semantic merge proposals.

| Template | Main production callers | Scenario |
|---|---|---|
| `learning-pack-plan` | `CodexAgent.PlanLearningAgenda`, `ClaudeAgent.PlanLearningAgenda` | Split current candidate files into self-contained evidence packs in one independent runtime call. |
| `learning-pack-analyze` | `CodexAgent.AnalyzeCurrentCodebaseBatch`, `ClaudeAgent.AnalyzeCurrentCodebaseBatch` | Initial current-code learning for one or more evidence packs in an isolated runtime call. |
| `learning-delta-pack-analyze` | `CodexAgent.AnalyzeCurrentDeltaBatch`, `ClaudeAgent.AnalyzeCurrentDeltaBatch` | Diff-anchored incremental learning in an isolated runtime call; output must be triggered by changed hunks. |
| `learning-pattern-normalize` | `CodexAgent.NormalizePatterns`, `ClaudeAgent.NormalizePatterns` | Propose semantic merges between new candidates and related stored patterns. Local normalization owns source-ID validation, ownership resolution, recovery, and storage. |
| `learning-profile-refresh` | `CodexAgent.RefreshProjectProfile`, `ClaudeAgent.RefreshProjectProfile` | Sync the complete project profile in a bounded profile-only runtime call when current analysis recommends it. |
| `learning-authority-extract` | `CodexAgent.ExtractAuthority`, `ClaudeAgent.ExtractAuthority` | Extract explicit constraints from the deterministic authority section catalog without mixing them into project-map analysis. |
| `learning-knowledge-review` | `CodexAgent.ReviewKnowledge`, `ClaudeAgent.ReviewKnowledge` | Independently accept, narrow, or reject every source-learning candidate before semantic normalization; the runtime constrains each response to the current candidate IDs and exact one-to-one decision count. |
| `core-workspace-profile` | `CodexAgent.AnalyzeWorkspaceProfile`, `ClaudeAgent.AnalyzeWorkspaceProfile` | Learn workspace-level project relationships and routing facts. |
| `core-workspace-spec` | `CodexAgent.AnalyzeWorkspaceSpec`, `ClaudeAgent.AnalyzeWorkspaceSpec` | Generate workspace-level executable development constraints. |
| `core-user-pattern` | `CodexAgent.UserDefinePattern`, `ClaudeAgent.UserDefinePattern` | Convert user-provided pattern descriptions into structured pattern output. |

Planning and learning stages retain read-only repository tools because their prompts intentionally reference runtime candidate lists, structural context, diffs, and repository paths. Cross-stage memory is explicit runtime data, not command-wide hidden conversation state.

## Redundancy Status

No loader prompt template is currently unused. Every file under `embedfs/templates/prompts/loader/` has a production render path in the agent layer.

Current-code learning uses independent runtime calls. Deterministic local preparation sends every in-scope path into planning; planning must account for every path through a focus or an explicit skip receipt. Each evidence pack is analyzed in its own call, and diff learning uses isolated diff-pack calls. Source candidates then pass an independent review that keeps each agenda focus together, runs focuses serially, and checkpoints each completed focus. Review output is constrained to the exact candidate ID set and decision count for that focus; local review application still rejects duplicates, omissions, and unknown IDs. AI proposes semantic merges only after all reviewed candidates are reassembled in agenda order; local normalization validates ownership, derives reviewed flags from source IDs, recovers missing coverage, and writes results.

Planning also assigns each evidence focus explicit attributes, risk signals, and `standard`, `careful`, or `critical` analysis depth. This depth controls evidence scrutiny in isolated pack analysis and remains separate from pattern admission. Analysis emits only controlled, evidence-based knowledge flags; independent review may correct them, while runtime code validates and propagates them without inferring semantics from names or paths.

Project-map refresh and authority extraction are separate calls with separate contracts. The profile stores navigation facts such as architecture, modules, and dependencies; it does not own reusable capability entries or authoritative rules. Authority extraction consumes a deterministic file/section catalog and returns one coverage receipt per section. The runtime validates that receipt and combines the extracted constraints with the profile only when building deterministic Skill projections. There is no persisted derived `project-spec.json` fact store.

Testing, deployment, and acceptance procedures are not learned from repository source. Their commands, steps, and ordering come from user-maintained Workflows, which are copied into generated Skills and routed by `SKILL.md`. Explicit command permissions or prohibitions from authoritative project files remain project safety rules.
