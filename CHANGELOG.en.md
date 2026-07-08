# Changelog

[简体中文](CHANGELOG.md) | [English](CHANGELOG.en.md)

## [v0.13.2]

### Fixes

- Cleaned up unused functions, ineffective appends, simplifiable struct conversions, and redundant formatting calls reported by `staticcheck`, allowing `staticcheck ./...` to run as an effective quality gate.
- Migrated bbolt lock-timeout checks to the new `go.etcd.io/bbolt/errors` error variables instead of the deprecated `bolt.ErrTimeout`.
- Removed unused test helpers and legacy render context types to reduce noise for future refactors.

## [v0.13.1]

### Changes

- Split `learn current` / `sync` file selection into clear stages: local filtering, gotree structural indexing, gotree file confirmation, AI file selection, and AI analysis-unit selection. Console progress now shows only important stages, while detailed metrics live in logs and the unified summary.
- AI file selection for large repositories now uses a gotreesitter structural index to produce high-value entry candidates instead of sending oversized file trees to the Agent, reducing large swings in analyzed file counts across runs.
- Runtime AI JSON output contracts are now generated from Go DTOs, removing hand-written JSON field descriptions that could drift from parser structures.
- Moved init-generated long-lived context templates to `embedfs/templates/seed/context/`, keeping runtime prompts, seed templates, and generated Skills templates under clearer ownership boundaries.

### Fixes

- Fixed JSON field contract drift across learning, profile, generation, and workspace prompts, including validation-command evidence, profile deltas, location fields, and batched unit output.
- Fixed resumable analysis-plan console summaries to report source files, local-filtered plan inputs, files sent to AI selection, files finally selected by AI, pending analysis files, and analysis units.
- Kept `jsonrepair-go` as the generic JSON repair layer while removing extra patch-style compatibility parsing; malformed output is now addressed at the prompt/contract source.

## [v0.13.0]

### Changes

- Consolidated runtime AI prompt templates into English-only source templates and removed separate `.en-US` runtime variants, reducing drift between Chinese and English prompt copies.
- Clarified that `skills.locale` controls AI learning output, generated Skills, and persisted natural-language content; runtime prompts now use a shared output contract to require the target language while preserving code identifiers, paths, commands, and config keys.
- Removed deprecated workspace runtime prompt template paths. Workspace profile/spec learning continues through the loader prompt system, keeping long-lived context templates separate from runtime analysis templates.
- Updated the README, command docs, configuration docs, and template README with core features, `skills.locale` semantics, and the single-source runtime prompt rule.

### Fixes

- Fixed prompt loader fallback when Skills language is omitted: it now uses the built-in Skills locale instead of the tool UI locale, preventing a Chinese UI from accidentally changing AI output language.
- The appended output-contract template is now rendered with target-language rules, keeping final JSON constraints after user context while enforcing the configured output language.

## [v0.12.2]

### Changes

- Renamed the long-lived `.skills-seed/context/` files to `background.md`, `constraints.md`, `terminology.md`, and `workspace.md`, and updated the README, command docs, configuration docs, and context templates.
- Init now removes deprecated `project.md`, `rules.md`, `glossary.md`, and the old `.skills-seed/prompts/` directory so legacy prompt scaffolding no longer participates in runtime context composition.
- Generated output now softens weak-evidence or local patterns into verification hints and emits coverage warnings for thin business groups, reducing the chance that a single observation becomes a hard rule.
- Tightened command architecture boundaries by adding `internal/service/syncflow` for the learn-then-generate sync use case; `sync` and `workspace` commands no longer depend sideways on other command packages.

### Fixes

- Reference generation now removes missing code snippets and treats likely non-project-relative references more conservatively, avoiding accidental removal of URLs, external identifiers, or descriptive text.
- Workspace context templates no longer write runtime placeholders or analysis-goal text, keeping them focused on long-lived editable project context.

## [v0.12.1]

### Changes

- Consolidated long-lived AI context created by `init` under `.skills-seed/context/`, adding `project.md`, `rules.md`, `glossary.md`, and workspace context files; runtime prompt rendering now merges the user-authored context fragments from that directory.
- Added candidate-admission budgets and current-change profiling to `learn current`, limiting new stored patterns by analysis unit, run, micro/minor change size, confidence, and routeable evidence to reduce fragmented rules from small edits.
- Added lifecycle fields to patterns, and made check, curate, generate, and statistics consume active patterns by default, leaving a clear boundary for stale, superseded, and deprecated pattern maintenance.
- Updated the README, command docs, configuration docs, and default config templates for the new project context directory and `learning.current.budget` settings.

### Fixes

- Fixed JSON-contract drift between learning/project-profile prompts and the Go data model by removing the outdated `business_method.layer` example and tightening string, array, object, and numeric type requirements for business methods, profile deltas, evidence locations, and validation commands.
- Deprecated prompt scaffold cleanup now preserves files with existing user-authored content, avoiding accidental removal of real project guidance during the context-directory migration.

## [v0.12.0]

### Changes

- Refactored the Skills generation architecture with an `internal/skillgen` generation plan and renderer, separating plan building, template rendering, and file writing from the generator service.
- Added `internal/knowledge` layers for policy decisions, business routing, validation command selection, and rule-claim view models; generation now consumes stable knowledge views instead of mixing classification, routing, and presentation rules inside template adapters.
- Upgraded pattern references from importance layers to rule claims, rendering each learned pattern as an actionable claim with evidence count, scope, locations, and usage guidance so weak observations are less likely to be treated as hard constraints.
- Consolidated generated text into i18n keys for workflows, categories, related references, validation matrix entries, rule claims, and renderer errors, reducing hardcoded user-visible strings in generation code.

### Fixes

- Agent invocation and result-parse failures now include unified diagnostics with agent, operation, attempt, output/error lengths, short previews, and content/raw/stderr/manifest archive paths, making it clear whether the CLI call failed or the Agent returned non-JSON output.

## [v0.11.6]

### Changes

- Tightened current-code and history-learning prompts with a Candidate Admission Gate and Facts Are Not Patterns rules, requiring direct source evidence, project specificity, actionability, routeability, and conservative confidence before emitting a pattern.
- Reframed the business coverage matrix as a missed-candidate check instead of a force-output rule; low-frequency, local, or fact-only candidates are now dropped first instead of filling space with weak evidence.
- Optimized learning read-scope guidance so Agents expand only to directly related files that can prove a candidate rule, avoiding broader reads just to strengthen weak candidates.

## [v0.11.5]

### Changes

- Moved the HTML overview page to `docs/html/skills-seed.html` and updated the README entry links to the new location.
- Split the HTML overview page into standalone CSS and JS, added light/dark theme switching and Chinese/English switching, and made README links open the matching GitHub README.

### Fixes

- Fixed the generated Skills structure examples in the docs by removing the nonexistent `testing.md` entry and listing the currently generated pattern categories.

## [v0.11.4]

### Changes

- `init` now adapts to repository state: first runs still open the setup flow, while later interactive runs offer inspecting the current configuration, reinitializing, or canceling instead of repeating first-run prompts or silently overwriting config.
- `sync` now distinguishes first sync, normal incremental sync, and unfinished resumable state: missing generated Skills shows a first-context flow, existing output offers current-state sync or restart, and unfinished state still offers resume or restart.

### Fixes

- Reinitialization now reuses the reset backup flow while preserving the Agent, Skills target, parallelism, analysis depth, and split-scope choices selected during the interactive reset.

## [v0.11.3]

### Changes

- Replaced the in-tree JSON repair implementation with `github.com/silaswei-io/jsonrepair-go`, and added a structured fallback that can recover JSON from text when direct unmarshalling fails.
- Unified one-shot context input across `learn current`, `patterns add/update`, and `sync` with repeatable `--context-path`, supporting files or directories. The old single-file `--context-file` and `patterns --files` semantics are now folded into context paths.
- Narrowed `sync` to learning current code and generating skills when needed. User-pattern creation is now handled by `patterns add/update`, which also records changelog entries.
- Added a local stable policy for AI file selection: explicit focus paths are force-kept, and overly narrow AI recommendations on large candidate sets are deterministically filled to a minimum budget to reduce coverage swings across runs.
- Generated business-method indexes now separate business entries from supporting entries, trigger hints use business-method signals for authorization, state-transition, and persistence scenarios, and validation command selection is weighted by change area.
- Updated the README, command/configuration docs, and added an HTML overview page for `--context-path`, the stable file-selection policy, and the new `sync` / `patterns` split.

### Fixes

- Pattern and business-method payload parsing now tolerates string code locations, string line numbers, and string confidence values, reducing failures from minor Agent output type drift.
- Resumed `learn current` progress now starts from the completed unit count instead of miscounting pending units as completed.

## [v0.11.2]

### Changes

- Added `learning.current.max_units_per_call` to control how many analysis units may be grouped into one AI call. The default is `1`, keeping per-unit calls by default to reduce oversized-output parse failures and cross-unit conclusion bleed.
- `learn current` can now analyze multiple units in one batch while reusing the same codebase snapshot context; progress, pattern storage, file-fingerprint commits, and profile-delta merging still land per unit.
- Generated skills now use stricter core-rule layering: only patterns with enough confidence, frequency, and evidence become strong constraints, while low-frequency or local evidence is rendered as reference guidance instead of a hard standard.

### Fixes

- JSON extraction now recognizes top-level `units` in batch analysis results, preventing valid batch responses from failing because the outer key was not detected.
- Current-code learning prompts now harden the JSON type contract for profile-layer fields, requiring `layers` to be an object array instead of a string and reducing parse failures from `profile_delta.layers` type drift.

## [v0.11.1]

### Changes

- `learn current` now supports in-project analysis-unit concurrency through `learning.current.parallelism`; in workspace mode, root `agent.parallelism` controls child-project concurrency while `learning.current.parallelism` controls unit concurrency inside each child project.
- Interactive init now prioritizes the common path: by default it asks only for tool language, initialization type, Agent, total parallelism, and the execution plan; analysis depth, split scope, Skills language, and Skills type are grouped under optional advanced settings.
- Init now accepts total Agent parallelism, automatically writes the derived project-unit or workspace project/unit concurrency settings, and shows the final config in the summary.
- Added `learning.current.scope` with `domain`, `flow`, and `module` to guide analysis-unit planning by business domain, workflow, or module/plugin granularity.
- Concurrent analysis progress now shows the current progress and running units, and workspace learning output shows both child-project concurrency and per-project unit concurrency.

### Fixes

- Resume state now includes `learning.current.scope`, so changing analysis depth or split scope no longer reuses stale analysis-unit plans.
- Agent JSON repair now tolerates invalid numeric line ranges such as `"line": 29-43`, and learning prompts now require evidence line numbers to be a single integer.
- Failed concurrent units no longer refresh the final progress line to `running []`, preserving the last useful running-unit context.

## [v0.11.0]

### Changes

- Added `learning.current.mode` with `fast`, `normal`, and `deep` strategies. The selected mode now participates in resume-state fingerprints so runs do not reuse stale analysis units from another mode.
- Simplified and abstracted current-code learning prompts while preserving key quality constraints for business subdomains, coverage matrices, candidate filtering, business expansion directions, and utils misclassification prevention.
- Generated project skills now include related-reference routing, business-pattern importance layers, change-scope validation matrices, and module-grouped entry method indexes.
- Entry method indexes now add receiver, module, or path context for generic entry names such as `Run`, `main`, `Start`, and `Init`, improving readability and navigation precision.
- Reference generation now validates evidence, business-entry, and module paths against the project root before rendering, reducing incorrect navigation links.

## [v0.10.7]

### Changes

- Changed `patterns add` to require `--context` for the natural-language pattern description, with `--files` accepting related files or directories. `sync --context` is now the unified path for adding a user pattern and regenerating skills, replacing the old `sync --add` semantics.
- Added `patterns update <pattern-id> --context <request>` so an Agent can regenerate structured pattern content while preserving the original pattern ID, creation time, and workspace ownership.
- `patterns show` now sorts overview output by latest update by default and supports `--sort updated|score|hits|category` for quality, hit-count, or category-oriented inspection.
- Reorganized `internal/agent/parser`: JSON extraction and repair now live under the `jsonrepair` subpackage, while result parsing, payload conversion, and workspace parsing are grouped by responsibility.

### Fixes

- Strengthened Agent JSON output repair for trailing commas, comments, single-quoted strings, Python-style `True`/`False`/`None`, and missing commas between object fields or array values.
- Made business-method parsing tolerate `prerequisites` and `returns` returned as string arrays, preventing valid JSON with minor field-type drift from interrupting `learn current`.

## [v0.10.6]

### Changes

- Added a unified interactive command entrypoint. Running `init` without flags now opens a terminal selection flow for tool language, project mode, Skills language, Agent type, and Skills type; running `sync` without flags now prompts to resume unfinished state or restart when applicable.
- Updated the `init` startup banner to show CLI version and embedded prompt-template short hash, removing the `open-source` tag while keeping version metadata in English.
- Reused the shared terminal selector across `check`, `hook`, `init`, and `sync`, reducing duplicated command UI logic.
- Extracted a reusable console step runner for `learn current`, unifying stage details, Agent retry status, and failure labels.

### Fixes

- Added `sync --resume`, `sync --restart`, and `sync --no-interactive` so users can explicitly continue unfinished learning state, clear command state before rerunning, or disable prompts in scripts.
- Made project-profile JSON parsing tolerate list fields returned either as strings or arrays, reducing failures from minor Agent output drift.

## [v0.10.5]

### Changes

- `learn current` unit analysis no longer injects the existing pattern store into every unit prompt, preventing prompt context from growing with stored pattern count. Deduplication remains handled by local deterministic post-storage merging, with `patterns compact --ai` still available for explicit semantic compaction.
- Improved learning progress output in both single-project and workspace modes. Progress lines now show concrete sub-actions inside the current stage and use the full planned unit count, for example `Analyze current codebase · unit 6/7 · NTLS API Gateway`.
- Generated `project-overview.md` now uses a more conservative project overview summary instead of promoting unit-scoped profile summaries into whole-project facts.

### Fixes

- Strengthened Agent JSON output repair for raw newlines/control characters inside strings, bare object keys, and array items missing an object-start marker, reducing parse failures during long-context learning runs.

## [v0.10.4]

### Fixes

- Fixed deterministic pattern merging potentially emitting duplicate curated pattern IDs when a candidate reused an existing pattern ID, preventing `learn current` from falling back after structural validation.
- Fixed malformed historical pattern stores with duplicate IDs from producing structurally invalid local merge results; deterministic merging now keeps its accepted set unique by ID internally.

### Documentation

- Updated the README and configuration guide to describe local deterministic pre-storage merging and same-ID deduplication.

## [v0.10.3]

### Changes

- Agent output archives still use `.md`, but valid JSON final content is now formatted as a readable fenced `json` block for easier inspection under `runtime/agent-outputs`.
- Strengthened the final output contract for JSON prompts so the Agent must fix invalid JSON internally before returning the final object.

### Fixes

- Relaxed runtime filename semantic-part length so labels such as `pattern-learn-current-unit-auth-admin-login` are no longer truncated into incomplete names like `...auth-admi`.

## [v0.10.2]

### Changes

- Refactored `learn current` into a business analysis unit plan plus unit-level learning flow: the Agent first plans units by business capability, then each unit is analyzed and immediately persists patterns and file fingerprints so failed runs can continue from already stored results.
- Added `.skills-seed/cache/commands/<command>/state.json` as deletable, rebuildable command resume state. Authoritative learning facts remain in `store/project.db`, while `runtime` only stores prompts, Agent outputs, logs, and other disposable diagnostics.
- Clarified `.skills-seed` directory semantics: `store/documents` stores persistent readable documents such as profiles, specs, state, and changelog; `cache` stores rebuildable caches; `runtime` stores disposable run artifacts.
- Split and renamed current-learning prompts so project initialization, project profile analysis, current-code business learning, and business-unit planning each do one job. Prompts and skill templates remain template/i18n driven instead of hard-coded in code.
- Improved generated project skills so business pattern overviews, business methods, touchpoint indexes, and pattern details focus more on how requesters describe business needs instead of only code structure.

### Fixes

- Fixed AI file-selection skipped files still being included in business-unit planning input during current learning.
- Fixed project profile deltas in unit-level learning risking retention of only the last unit's delta; successful unit deltas are now merged across the run.
- Fixed later units in the same run using a stale known-patterns snapshot after earlier units had already saved new patterns.

### Documentation

- Updated the README, command reference, and configuration guide for the new `store/cache/runtime` boundary, current-learning plan cache, and prompt/skill generation responsibilities.

## [v0.9.19]

### Changes

- Generated project `SKILL.md` workflow descriptions now summarize the Agent-organized workflow body instead of echoing the user's original notes.
- Project skill entry files no longer embed maintenance commands such as `skills-seed learn history` or `skills-seed generate skills`, keeping the skill focused on the learned project itself; configuration-controlled generated notices are unchanged.

### Fixes

- Workflow output context, script, and summary-heading text now comes from i18n messages, avoiding hard-coded Chinese or English headings during workflow description extraction.

## [v0.9.18]

### Changes

- When `--name` is omitted, new workflows now get a readable ID from the Agent-generated English title. Repeated titles receive a numbered suffix instead of implicitly updating an existing workflow.

### Documentation

- Updated the Chinese and English README and configuration guide to clarify that workflow storage directories use `<id>`, generated from the Agent's English title slug when no explicit name is provided.

## [v0.9.17]

### Changes

- Removed the skills dirty-state mechanism. `sync` now always enters `generate skills` after learning or adding a pattern, and explicit generation continues to fully rebuild from the current profile, patterns, and workflows without maintaining or clearing dirty targets.
- Changed user workflow update semantics: same-name workflows merge and deduplicate by default, and only `--overwrite` fully replaces one. The old `--append` flag was removed.
- When `--name` is omitted, new workflows now get a stable ID based on the Agent title and current input. Repeated inputs receive a numbered suffix instead of implicitly updating an existing workflow.
- Changed the workflow prompt from an organizer to an inferrer. It can infer development, testing, validation, code generation, and release workflows from explicitly provided goals, constraints, background, paths, or rough notes.

### Fixes

- Fixed `learn current` skipping profile refresh when files were unchanged but the project profile was missing, which could leave workspace child skills generation unable to recover after a missing-profile failure.

### Documentation

- Updated the Chinese and English README, command reference, and configuration guide to remove outdated skills dirty, `--append`, and sync generation-skip descriptions.

## [v0.9.16]

### Changes

- Refactored skills generation so an explicit `generate skills` run no longer skips through generation-input fingerprints or dirty state. It now deletes the old skills-seed generated output directory and fully rebuilds from the current profile, patterns, and workflows; `--force` was removed.
- Changed pattern storage and `patterns compact` to use local deterministic merging by default. Candidates are processed one by one and the higher-quality pattern is kept as the primary entry; Agent semantic merging is used only with `patterns compact --ai`.
- Removed automatic workflow extraction. Workflows are now added only through explicit user input with `skills-seed workflow --context ...`; workspace roots can use `--child` to save a workflow into a child project.
- Skills generation no longer calls the Agent for a summary. It generates directly from curated patterns, profiles, and workflows, reducing generation latency and nondeterminism.

### Documentation

- Updated the Chinese and English README, command reference, and configuration guide to document full regeneration semantics, explicit AI pattern compaction, and user workflow storage/generation.

## [v0.9.15]

### Changes

- Added `skills-seed log`, which prints learned and generated changes in a git-log-like format. Change entries are stored in `.skills-seed/memory/change-log.json`, keeping user-facing summaries separate from diagnostic logs.
- Changed the Git pre-commit hook flow. Installed hooks no longer force checks or learning by default; interactive terminals now show a menu for "sync and generate skills", "learn only", or "skip this time". Non-interactive environments skip directly so IDEs, scripts, and Git automation are not blocked.

### Documentation

- Updated the command reference in both languages for `skills-seed log` and the new hook menu behavior.

## [v0.9.14]

### Documentation

- Improved the command reference opening with a linked command overview and common workflows so readers can jump to commands by usage scenario.
- Added the missing `skills-seed preview` command documentation and documented the `generate skills --no-references` flag.

### Maintenance

- Added a command index generated from the Cobra command tree, plus tests that verify the generated sections in both command-reference documents stay aligned with the CLI implementation.

## [v0.9.13]

### Changes

- Split runtime prompt templates from `embedfs/templates/prompts/common` into the `loader` directory, and moved appended final-output contracts into a separate `append` directory so normal prompt templates and appended constraints stay separate.
- Centralized the final output contract for all JSON runtime prompts. The loader now appends stronger mandatory JSON rules requiring the final response to be one directly parseable JSON object and to pass an internal JSON self-check before responding.

### Fixes

- Fixed historical snapshot files matched by `.gitignore` or `exclude` still appearing as deleted diffs in AI input. Snapshots still preserve the full current state, but diffs sent for AI analysis now follow the current file-filtering policy.
- Fixed unclear pattern-curation warnings when `dropped` referenced non-candidate IDs. These invalid dropped entries are now ignored with clearer informational logging instead of making the run look like a full learning failure.

## [v0.9.12]

### Changes

- Added the `exclude.gitignore` setting, enabled by default. Learning, preview, project-structure summaries, sample-file collection, and structural pre-scan now apply Git ignore rules in addition to `exclude.paths`, and the setting can be disabled when ignored source files should still be analyzed.
- Changed the previous top-level `exclude` list to `exclude.paths` and moved the Git ignore switch under `exclude.gitignore`, removing the standalone `file_filter` block. Legacy config shapes now fail during parsing instead of being silently migrated.

## [v0.9.11]

### Changes

- Added the global `file_filter.apply_git_ignore` setting, enabled by default. Learning, preview, project-structure summaries, sample-file collection, and structural pre-scan now apply Git ignore rules in addition to `exclude`, and the setting can be disabled when ignored source files should still be analyzed.

## [v0.9.10]

### Fixes

- Improved workspace `learn current` / `sync` output for parallel child-project learning: each child still keeps its own progress bar and step count, child names and step columns are aligned, and detailed child start/skip/completion logs no longer interrupt the progress panel in unchanged runs.

## [v0.9.9]

### Fixes

- Fixed workspace `learn current` / `sync` potentially re-analyzing the workspace profile and workspace spec when child repos were unchanged but the CLI version or prompt templates changed. Workspace relationship learning now skips based on relationship facts and this run's one-shot context, and migrates legacy fingerprints when existing artifacts still match.

## [v0.9.8]

### Changes

- Expanded `skills-seed patterns show <pattern-id>` detail output with good/bad examples, quality metrics, merge/generated state, workspace ownership, business-method fields, code-location history, and language-agnostic symbol snapshots.
- Added pattern-level evidence locations; when a learned pattern has no business method binding, the `patterns show` overview falls back to evidence locations instead of showing empty location state/current location.
- Optimized workspace `skills-seed sync`: when all child projects and workspace relationship artifacts produce no skills dirty target, sync skips `generate skills` after learning completes.

### Documentation

- Updated README and the command reference to clarify that `patterns show` without arguments prints the overview, passing a pattern ID prints the full detail view, and unchanged `sync` runs skip generation.

## [v0.9.7]

### Changes

- Expanded `skills-seed patterns show <pattern-id>` detail output with good/bad examples, quality metrics, merge/generated state, workspace ownership, business-method fields, code-location history, and language-agnostic symbol snapshots.

### Documentation

- Updated README and the command reference to clarify that `patterns show` without arguments prints the overview, while passing a pattern ID prints the full detail view for one pattern.

## [v0.9.6]

### Changes

- Unified debug record filename prefixes under `.skills-seed/memory/runtime` as `YYYYMMDD-HHMMSS.NNNNNNNNN-<kind>-<name>`, so rendered prompts, Agent output archives, and runtime temporary input directories can be located by time order.

### Maintenance

- Added the shared `runtimefiles` naming helper to centralize safe runtime filename parts and timestamp prefixes instead of keeping separate naming logic in prompt, Agent, and workspace flows.

## [v0.9.5]

### Fixes

- Fixed retryable HTTP status detection still treating standalone `429` / `503` / `529` numbers in normal output as rate limits.
- Fixed Codex multi-part `agent_message` merging still returning only the last message segment.
- Fixed `ExtractJSON` potentially parsing a non-JSON code block before the later JSON result in multi-language project output.

### Maintenance

- Backfilled the English `CHANGELOG.en.md` entry for `v0.9.4`.
- Removed scache-specific analysis reports from the repository root so release content stays aligned with the all-language, all-project tool positioning.

## [v0.9.4]

### Fixes

- Fixed inconsistent path normalization between `SaveAnalyzedFiles` and `DeleteAnalyzedFiles`, which could leave stale incremental-learning fingerprints.
- Fixed `ExtractJSON` failing to extract nested JSON from code blocks by replacing non-greedy regex extraction with brace-counting.
- Fixed `TruncString` truncating by byte and splitting multi-byte characters; log truncation now uses rune boundaries.
- Fixed retry detection so normal output containing numbers like "429" is not treated as rate limiting.
- Unified Claude/Codex error handling and rate-limit messages.
- Fixed `repairUnescapedQuotesInStrings` incorrectly splitting on commas inside string content.
- Fixed Codex final-content extraction losing earlier message events when output is split across multiple agent messages.
- Fixed Codex `LearnFromCommit` not setting `LearnedAt`.
- Fixed `safeRelativePath` incompletely blocking traversal paths such as `foo/..`.

## [v0.9.3]

### Fixes

- Fixed direct Pattern saves writing non-canonical category buckets when categories include uppercase letters or surrounding whitespace; saves and category queries now normalize categories first.
- Fixed stale same-ID Pattern copies left in legacy non-canonical category buckets after saving again, which could duplicate counts; deleting a Pattern now removes all historical category copies.
- Fixed similar Pattern lookup missing existing canonical-category patterns when the input category uses compatible aliases, different casing, or whitespace.
- Fixed `patterns compact --category` missing canonical categories when the input uses different casing, whitespace, or compatible category aliases.
- Fixed the `learn current` project-init prompt JSON example using explanatory text as the `category` value, reducing the chance that models copy an invalid category string.

## [v0.9.2]

### Changes

- Centralized the pattern-category contract in the domain layer so prompts, curator validation, and storage paths share the same allowed category list.
- Updated Chinese and English prompts for `learn history`, `learn current`, `patterns add`, and `pattern-curate` to show the shared allowed categories.

### Fixes

- Fixed pattern curation fallback when AI outputs the compatible `security` category; it is now normalized to `utils`.
- Fixed misleading curator logs so validation failures are reported separately from parse failures.

## [v0.9.1]

### Features

- Added AI relevant-file selection for `learn current`, narrowing large candidate file sets from the file tree and change metadata before analysis.
- Added `skills-seed patterns delete` to remove patterns by ID and synchronize linked child-project patterns from a workspace root.
- Added skills dirty state and `generate skills --force`; generation can now skip unchanged targets and regenerate only skills affected by learning, pattern, or workspace relationship changes.
- Added a stronger AI JSON repair pipeline for common model-output issues such as duplicated object starts, invalid escapes, unescaped quotes inside strings, and missing closing containers.

### Changes

- Reorganized learning config into `learning.current` and `learning.history`: structural context moved from `analysis.structural` to `learning.current.structural`, and history defaults moved under `learning.history`.
- `learn current --profile auto` now refreshes project profiles only when missing or when this run actually writes or updates patterns.
- Workspace relationship analysis skips unchanged inputs when artifacts already exist and marks only affected workspace or child skills dirty.
- Generated skills and references now include validation commands and tighter module and business-method evidence guidance.

### Fixes

- Removed the root `completion` command and deleted its command-reference sections.
- Fixed Chinese locale help text for `help`, `preview`, `review`, and `patterns show/stats` so command descriptions, flags, and table headers no longer fall back to English.
- Fixed the English README root example to use `skills-seed workspace add .`.

## [v0.9.0]

### Features

- Added the `curator` service as the single pre-storage boundary for candidate patterns: it retrieves related historical patterns, asks AI to deduplicate/consolidate/drop candidates, validates the result server-side, then writes to the database.
- Added the `pattern-curate` prompt, requiring pre-storage validation for candidate coverage, duplicate-rule consolidation, code-evidence provenance, summary consistency, and low-quality candidate drops.
- Added the explicit maintenance command `skills-seed patterns compact` for curating the existing pattern store; supports `--category` and `--dry-run`.

### Changes

- `learn current`, `learn history`, `learn staged/commit`, and `patterns add` now produce candidate patterns; all add/update/merge/drop decisions happen inside the curator storage boundary.
- `generate skills` is now read-only with respect to the pattern store. It no longer merges or repairs patterns; it only reads stored project profiles, workspace profile/spec data, and patterns to generate skills.
- `sync` now runs `learn current -> generate skills` or `patterns add -> generate skills`; pattern curation happens while learning or adding patterns stores candidates.
- Project-structure summaries, sample-file collection, and structural pre-scan now share the configured file-selection policy. Apart from built-in safety boundaries and `exclude`, analyzer no longer maintains extra directory-name keyword rules.
- Skills templates and generated references were tightened toward language-agnostic, evidence-driven wording so the generator does not synthesize hardcoded project guidance.

### Breaking Changes

- Removed `skills-seed generate skills --merge`.
- Removed the old `skills-seed patterns merge` command; use `skills-seed patterns compact` instead.
- Removed the old `internal/service/merger`, `pattern-merge` prompt, and `MergePatterns*` Agent API.

## [v0.8.1]

### Features

- Business pattern references now use an index + domain-detail structure: `business.md` keeps only reading guidance and detail links, while full rules and code evidence are written to `references/patterns/business/*.md` to avoid oversized single files.
- Business pattern domains are grouped from code location, scope, and stable directory names; rules without stable ownership fall back to `other`, avoiding project-specific business keywords in the generic generator.
- Generated main skills and project specs now link references conditionally based on the files actually generated, preventing broken links in sparse projects or `--no-references` output.

### Changes

- Project-init, incremental learning, and pattern-merge prompts now require `good_example` to be copied from read source as a complete semantic snippet, forbidding synthesized or rewritten “good examples”.
- Skills templates now label examples as “Code Evidence” instead of “Good Example” to reduce the chance that models treat examples as freely generated code.
- Project specs no longer cap business rules; all executable business rules are retained, with reference splitting controlling context size.

### Fixes

- Fixed `GenerateSkillsWithOptions` dropping its options. `SkipReferences` now actually skips reference file generation.
- Fixed unchanged-input generation checks only validating `business.md` and ignoring business detail files, preventing false skips when detail files are missing.
- Fixed remaining `skills-seed generate-skills` template references, standardizing on `skills-seed generate skills`.

## [v0.8.0]

### Features

- Agent call outputs are now archived separately under `.skills-seed/memory/runtime/agent-outputs/`, including final content, raw CLI output, stderr, and a manifest for model-output debugging without polluting runtime logs.
- Business-method code locations now use structured `code_location` metadata throughout, preserving current location, historical location, status, and language-agnostic symbol snapshots. Generated business-method references show location status.

### Changes

- Runtime logs no longer include Agent reply previews, raw stdout/stderr, or JSON snippets. They now keep lengths and runtime archive paths only.
- Initial project learning and pattern-merge prompt examples now use `code_location.current_location`; example JSON is fenced as documentation while the actual model response is still required to be unfenced JSON.
- Generated project skills and references are more compact: entry skills guide the Agent to read only the minimum relevant references, project specs focus on executable rules, and project overviews avoid repeated structure dumps.
- Project profiles clean unusable business methods before saving; a business method must have both a name and a displayable location to be retained.

### Fixes

- Fixed `learn current` failing immediately when Agent output only missed trailing JSON container closers. The parser now conservatively restores missing `}` / `]` closers, while still rejecting unterminated strings and truly invalid JSON.

## [v0.7.4]

### Fixes

- Improved the error message when the project database is locked. When BoltDB cannot acquire the `.skills-seed/memory/project.db` lock before its timeout, the CLI now explains that another `skills-seed` command may be using the database and suggests waiting or checking for a stale process.

## [v0.7.3]

### Features

- Added `skills-seed patterns show` to inspect pattern timestamps, source, code-location status, and language-agnostic symbol snapshots from the DB; supports single-record details and JSON output.

### Changes

- Pattern, file-analysis fingerprint, pattern-hit, review-comment, and analyzed-commit records now maintain `created_at/updated_at`.
- Business-method code locations now include structured DB metadata for historical location, current location, status, change kinds, and language-agnostic symbol snapshots. Generated docs prefer the current location while retaining historical location and status.

### Fixes

- Fixed `learn current` committing file-analysis fingerprints when pattern persistence fails, preventing unsuccessfully learned files from being skipped by later incremental learning.

## [v0.7.2]

### Fixes

- Fixed project-profile JSON parsing when `AnalyzeProject` output contains an occasional duplicated object-start fragment inside object arrays, covering malformed model output such as `{"{"name": ...`.
- Fixed `AnalyzeProject` parse failures being converted into a successful `unknown/parse failed` fallback profile. Parse failures now return errors so existing valid profiles are not overwritten.
- Fixed misleading `learn current` output that could report “project profile saved” even when the saved profile was only a parse-failure placeholder.

### Documentation

- Updated README and changelog notes for 0.7.2 project-profile JSON recovery and parse-failure protection.

## [v0.7.1]

### Features

- Prompt rendering now strips default scaffolding and generated metadata, merging only user-authored project constraints, workspace constraints, and instruction fragments into Agent input.
- Rendered prompts are saved under the runtime directory with a manifest that records whether each fragment was included, plus raw and final lengths, making prompt-context debugging easier.
- `learn current` file selection, excludes, incremental fingerprints, and commit bookkeeping moved into the `fileanalysis` service so analysis, preview, and learning share one policy.

### Changes

- Project prompt templates now default to empty comment guidance, preventing generic bootstrap text from being repeatedly appended as user constraints.
- Structural analysis and sample selection now default to source, build config, and dependency config files while continuing to skip documents, generated outputs, and generated Skills directories.
- The skills-seed generated footer in Skills templates is now configuration-controlled and omitted by default.
- Default config templates, source comments, and constant documentation now use Chinese-first wording with mixed English where technical names are clearer, such as Agent, Skills, CLI, and tree-sitter.

### Documentation

- Updated README, configuration reference, and changelog for 0.7.1 prompt merge cleanup, runtime debug manifests, unified file-selection policy, and comment/documentation wording.

## [v0.7.0]

### Breaking Changes

- Removed CodeGraph integration and the `analysis.codegraph` config section. Old fields are not kept for compatibility.
- Structural analysis is now configured through `analysis.structural`, with only `enabled`, `max_symbols`, and `max_file_size`.
- Renamed `max_nodes` to `max_symbols` to make the meaning explicit: the maximum number of symbols emitted into structural context.

### Features

- Added a lightweight embedded tree-sitter structural pre-scan that extracts symbols, imports, entry points, and module clues without an external command or local index.
- Structural pre-scan only runs when bounded inputs such as focus paths, diffs, samples, or entry files exist, avoiding unbounded whole-repository scans.
- Current-code learning now handles added, modified, and deleted file states. After analysis, snapshots are replaced within the learned scope so the next run can compute incremental diffs from a clean snapshot.

### Documentation

- Updated README, command reference, and configuration reference for the 0.7.0 embedded structural pre-scan, `analysis.structural` config, and CodeGraph removal.

## [v0.6.4]

### Features

- Added `generate skills --no-references` flag to skip reference document generation (`references/` directory); SKILL.md and Agent metadata are always generated.

### Changes

- Refactored Generator into a pure orchestration layer:
  - Extracted `SkillWriter` (`writer.go`) for all template rendering and file I/O.
  - Moved pure functions to the domain layer: `CleanProjectProfile`, `RankPatternsForGeneration`, `NewProjectSpecFromProfile`, etc.
  - Split workspace generation into a standalone sub-package `internal/service/workspace/`, fully decoupled from single-project generation.
- Reduced `GeneratorService` dependencies from 10 to 5 (`patternRepo`, `profileRepo`, `agent`, `configRepo`, `writer`).

## [v0.6.3]

### Features

- Added `--skills-locale` so tool output/config template language can be separated from generated Skills and prompt language.

### Changes

- Added `skills.locale` to config. Generated Skills now default to English, while `profile.locale` continues to control CLI output and config templates.
- Agent prompts, project prompts, Skills templates, and workspace generation now consistently read the Skills language setting instead of inheriting the tool UI language.

### Documentation

- Updated command and configuration references for the different responsibilities of `--locale` and `--skills-locale`.

## [v0.6.2]

### Fixes

- Fixed repeated workspace root relationship analysis and skill generation when inputs had not changed. Skills Seed now records input md5 values and skips unchanged work when outputs are complete.
- Fixed mismatches between actual CLI help and the command reference by removing the obsolete `generate skills --context` example and correcting flag descriptions for `sync --context`, `patterns add --files`, and related commands.

### Changes

- Fast skipped/completed workspace child steps now share a global `200ms` pause instead of scattered fixed waits, reducing idle terminal time for unchanged runs.

### Documentation

- Updated the command reference for workspace root relationship-analysis and `generate skills` input-md5 skip behavior.
- Synchronized the command reference with actual CLI help for `init` / `reset` defaults, `learn history --batch-size` default source, repeated `patterns add --files`, and the scope of `sync --context`.

## [v0.6.1]

### Fixes

- Fixed workspace learning writing only the `workspace.projects` config skeleton to `workspace-profile.json`, which prevented root workspace skills from inheriting child project profiles and one-shot learning context.
- Fixed workspace child learning/generation Agent calls that could still execute from the root workspace path; Agent calls now resolve their working directory from the active child `.skills-seed`.
- Fixed the boundary where generation could accept one-shot user context and include it in skill summaries; `generate skills` no longer accepts `--context` / `--context-file` and only consumes profile/spec/patterns already learned.
- Fixed missing terminal progress during root workspace profile/spec analysis after child learning completed, which made long workspace analysis look stuck.
- Tightened skill output path validation so workspace roots and child projects cannot write generated skills outside their corresponding project root.

### Changes

- `learn current --context` / `--context-file` remain one-shot learning inputs. Workspace learning passes them into workspace profile/spec analysis, while prompts explicitly forbid copying the original text or long paraphrases into persisted artifacts.
- Workspace root learning now reads each child project's learned `project-profile.json` summary, frameworks, and key modules before generating and saving richer `workspace-profile.json` and `workspace-spec.json`.
- Workspace profile/spec merge logic moved into `internal/workspace`, so learning and generation share the same fallback routing and merge rules.

### Documentation

- Updated README, command reference, and configuration reference for the 0.6.1 one-shot context boundary, workspace learning artifact flow, and removal of context flags from `generate skills`.

## [v0.6.0]

### Breaking Changes

- Renamed the top-level config key from `project` to `profile`. `profile` describes the project or workspace that owns the config file and avoids confusion with `profile.mode: "project"`.
- Removed `workspace.shared`, `workspace.contracts`, and `workspace.infra` from user config. Workspace shared paths, contract paths, and infrastructure paths are now learned/analyzed into workspace profile/spec from repository evidence and user context instead of being hand-written by users.
- Workspace child discovery is now limited to first-level directories that have their own `.git`. Files such as `go.mod`, `package.json`, install scripts, Helm charts, or Terraform files only classify project type and language; they no longer decide whether a child project exists.

### Features

- Workspace initialization leaves root `profile.language` empty by default, supporting workspaces that contain multiple languages.
- `init` now fills `profile.git_remote` from the current repository's `origin` remote.
- Shell installer/base repositories can be classified as `type: "infra"` and `language: "shell"`, for example independent Git children containing `install.sh`, `_install.sh`, or `install.ini`.

### Experience

- Default `config.yaml` now uses large module comment headers and field-level preceding comments, keeps blank lines between modules, and avoids sentence-ending punctuation in comments.
- `workspace.projects` is now the only user-facing workspace config field, reducing confusion between project/profile/workspace/shared/infra concepts.
- Saving an old config rewrites it to the new structure and removes deprecated workspace path fields.

### Documentation

- Updated README, command reference, and configuration reference for the 0.6.0 config structure, workspace child boundary rule, and removed path config fields.

## [v0.5.0]

### Breaking Changes

- `skills-seed add` migrated to `skills-seed workspace add`
- `skills-seed generate-skills` split into `skills-seed generate skills`
- Removed the legacy `internal/command/add` package; logic unified under `internal/command/workspace`

### Features

- Added `skills-seed patterns add <description>`: define coding patterns in natural language; AI generates structured patterns
- Added `skills-seed sync` one-step command:
  - `sync` = learn current → patterns merge → generate skills
  - `sync --add <description>` = patterns add → patterns merge → generate skills
- Added `skills-seed generate` parent command with `generate skills` subcommand, reserving room for future generation types
- Added `skills-seed workspace` parent command with `workspace add` subcommand for cleaner command structure
- AI agents now support exponential backoff retry for 429 / 529 / overloaded errors; retry count and interval configurable in `config.yaml` under `agent.retry`; the active progress line distinguishes normal, waiting, and retrying states, showing the agent error, failed call duration, and backoff wait
- Added `UserPatternDefiner` agent interface for user-defined pattern generation
- Added user-defined pattern prompt templates (`user-define-pattern`) in both Chinese and English
- User-defined patterns are automatically tagged with `source: user_defined`

### Changes

- Updated command routing table: `generate/skills`, `sync`, `workspace/add`, `patterns/add` require project runtime
- Removed unreachable code in `commandNeedsProjectRuntime`

### Documentation

- Updated README, command reference, and configuration reference for the 0.5.0 command structure, `patterns add`, `sync`, and `agent.retry`

## [v0.4.4]

### Improvements

- Improve runtime prompt JSON output constraints by removing markdown code fences from examples, reducing the chance that agents return fenced JSON that parser-facing calls cannot consume.
- Tighten file-reading scope in prompts related to `learn current`, `learn history`, `generate-skills`, and `check` fix generation: prompts now prioritize target files, changed files, CodeGraph structural context, and directly related call relationships instead of encouraging whole-repository scans.
- Improve project initialization analysis prompts: fixed framework, ORM, and logging catalogs were removed, so the agent extracts only the technology stack actually used by the project instead of being biased by examples.

### Changes

- `fix-generate` now parses and uses `summary` and `warnings`; generated fixes shown through `check` can surface manual-review warnings when a file cannot be safely rewritten in full.
- `skill-project-summary` now passes `key_insights` and `improvement_suggestions` into generated project `SKILL.md` files, making summary-stage insights and improvement suggestions visible to agents.
- `pattern-merge` now preserves good examples, bad examples, and business method data on merged patterns, so generated skills can still use those fields after merging.

### Fixes

- Fix the misspelled `concurrency` category in the Chinese `skill-project-summary` prompt.

## [v0.4.3]

### Fixes

- Fix Windows generating English config by default when `--locale` is omitted; the implicit default is now consistently Chinese.
- Fix root project `init` not auto-detecting frontend/Node project language and leaving config as `go`.
- Improve Windows path compatibility by supporting `~\path` expansion and avoiding Unix `tree` arguments on Windows.

### Release

- Add a Windows arm64 release asset.

## [v0.4.2]

### Fixes

- Fix `skills-seed help` hanging on Windows in uninitialized directories because parent-directory traversal did not stop at drive roots.
- Fix project-independent commands such as `help`, `--version`, `completion`, `init`, and `hook` being unavailable while project learning holds the database/runtime; `reset` still requires the project runtime guard to avoid resetting `.skills-seed` during learning.
- Fix `skills-seed reset help` being treated as a real `reset` execution; commands that do not accept positional arguments now reject extra arguments before running business logic.

## [v0.4.1]

### Changes

- Clarify `.skills-seed/prompts/` semantics: user files are now merged with built-in prompts as project context, workspace constraints, and user instructions instead of acting as full prompt replacements.
- Move user instructions to `.skills-seed/prompts/instructions/<prompt-id>.md`, and use `.skills-seed/prompts/project/<prompt-id>.md` for project-level prompt fragments.
- Rename initialized workspace prompt files to canonical runtime prompt IDs: `skill-workspace-profile.md` and `skill-workspace-spec.md`.
- Change the default `project-profile.md` content to fact-style `Not recorded` placeholders, avoiding task instructions such as "describe" or "analyze" inside runtime prompt context.
- Add built-in `output-contract-guard` prompt templates appended after user instructions to protect JSON / Markdown output formats.

### Documentation

- Add README / README.en coverage for prompt merging and one-time `--context` / `--context-file` guidance.
- Update command and configuration references with `.skills-seed/prompts/` directory purposes, merge order, final output contract behavior, and the difference between one-time guidance flags and persistent instructions.

## [v0.4.0]

### Fixes

- Improve workspace `generate-skills` progress output: the first line now shows aggregate child-project completion and each child line shows its own 5-step detailed progress, avoiding the old `1/1 Writing skill files` display and root/child progress overlap.
- Make fast progress steps visibly animate so short steps still provide stable spinner and elapsed-time feedback.
- Fix JSON parsing failures caused by invalid escaped characters in Agent output, and centralize JSON file read/write handling.

### Experience

- Improve `.skills-seed/config.yaml` comment layout with clearer block comments and less inline-comment noise.
- Reuse workspace child-project progress naming between `learn` and `generate-skills` for consistent output.

## [v0.3.0]

### Breaking Changes

- Rename config from `agent.provider` / `output.skills_paths` to `agent.engine` / `skills.target` / `skills.paths`, clearly separating the Agent CLI used for analysis, learning, and summaries from the generated skills target format
- Remove `workspace.init_children` and the `init --children` / `init children` semantics; workspace initialization now initializes the child projects detected at that time

### Features

- Add `skills-seed add .` to auto-detect and add all current child projects from a workspace root, initializing child repositories that are missing `.skills-seed`
- Add `skills-seed add <child...>` to add specific child projects by ID or path; targets such as `./frontend`, `frontend/`, and `frontend\` are normalized to the same child
- Make `add` initialize child repositories before syncing root `workspace.projects`; failed child initialization no longer pollutes the root config
- Make workspace initialization initialize detected child projects by default. Newly created children inherit root Agent and Skills config, while existing child configs are preserved
- Make `generate-skills` resolve its default output path from `skills.paths` using `skills.target`, allowing `claude` to run generation summaries while producing `codex` skills

### Documentation

- Rewrite README / README.en as proper project entry points covering positioning, workflow, workspace behavior, `add`, and the Agent engine vs skills target split
- Update command reference, configuration reference, CLI help, and prompt text to remove old `provider`, `output.skills_paths`, and `init children` wording

## [v0.2.0]

### Changes

- Flip template locale convention: Chinese templates no longer carry the `.zh-CN` suffix (e.g., `learn-analyze.txt.tmpl`), while English templates are explicitly suffixed `.en-US` (e.g., `learn-analyze.en-US.txt.tmpl`); `zh-CN` is now the default locale for all template loading
- Unify all prompt and skills template names to `domain-feature` kebab-case, replacing the previous snake_case / mixed naming:
  `analyze` → `learn-analyze`, `batch-learn` → `learn-batch`, `generate_fixes` → `fix-generate`, `generate_skills_summary` → `skill-project-summary`, `merge-patterns` → `pattern-merge`, `project-analysis` → `project-analyze`, `init-skills` → `skill-project-init`, `workspace-profile` → `skill-workspace-profile`, `workspace-spec` → `skill-workspace-spec`, `skill` → `project-skill`, `workspace/SKILL` → `workspace-skill`
- Introduce a centralized skills template catalog system: `TemplateEntry` declaratively maps template IDs to paths and provider allowlists, replacing the previous `fs.WalkDir` dynamic scan

### Features

- Extract `DefaultExcludePatterns()` as a standalone function; full static exclusion rules are now written to the config file during initialization
- Expand default exclude patterns from 7 to 31 entries, covering common build outputs (`dist`, `build`, `out`, `target`), temporary files (`*.tmp`, `*.bak`, `*.swp`), archives (`*.zip`, `*.tar.gz`), images, and video assets
- Add basename glob matching in file filter: patterns without `/` (e.g., `*.log`) now match against both the file basename and the full path

### Documentation

- Update the `exclude` defaults table in the configuration reference to reflect the expanded exclusion rules

## [v0.1.0]

### Fixes

- Fix `skills-seed init --workspace --children` leaving the root `.skills-seed` behind when child project initialization fails, preventing the next retry from incorrectly reporting that initialization already completed
- Improve terminal output ordering: active steps keep the progress title visible first, regular logs and token details are printed after the step completes, and workspace child generation token details keep their child-project scope

## [v0.0.9]

### Features

- Add pattern quality metrics that are recalculated when patterns are saved or merged, including specificity, evidence count, generic penalty, and effective score
- Record `check` issues with `PatternID` as pattern hits, preserving whether each learned rule is actually used by later checks
- Add `skills-seed patterns stats` to show pattern category, specificity, confidence, effective score, hit count, and last hit time
- Add `skills-seed review import --from-file` and `skills-seed review stats` to import local review comments and measure prevention against existing pattern hits by file and line window

### Experience

- Include quality metrics in known-pattern snapshots so later learning can account for existing rule quality and avoid amplifying generic rules
- Sort pattern stats by hit count and effective score, making high-value rules and never-hit rules easier to identify
- `generate-skills` now ranks patterns by `EffectiveScore*0.6 + normalized(HitCount)*0.3 + Confidence*0.1`, passing quality metrics and hit stats to the Agent so project-specific rules with actual usage are prioritized
- Review stats use a default `±3` line matching window and show total comments, prevented comments, missed comments, and matched pattern counts

### Documentation

- Update README and command references for pattern quality metrics, `patterns stats`, review comment import/statistics, and `generate-skills` quality ranking

## [v0.0.8]

### Documentation

- Simplify README and move the full command reference to `docs/COMMANDS.md` / `docs/COMMANDS.EN.md`
- Add configuration reference documents at `docs/CONFIGURATION.md` / `docs/CONFIGURATION.EN.md`, covering config fields, defaults, path semantics, and related behavior
- Organize the command reference by top-level command with standard sections such as command overview, command forms, flags, subcommand flags, and notes
- Document every command, subcommand, and flag, including `help`, `completion`, `--context`, `--profile`, workspace, hook, patterns, and profile usage
- Rework README as an overview page focused on capabilities, workflow, Agent support, and quick start, with detailed command and configuration references linked at the end
- Add a centered README header that highlights positioning, language links, supported Agents, and key documentation entry points

### Experience

- Add `Long`, `Example`, and flag help for all business subcommands so `skills-seed <command> --help` shows complete usage
- Shorten the root command help intro to reduce noise in `skills-seed help`
- Add bilingual help coverage tests to prevent missing help text or unresolved i18n keys in new commands
- Change successful `init` output to print the relative `.skills-seed` path on the success line and show the README URL for the current version tag
- Remove optional follow-up command hints from `init` and `learn current` output
- Add `--agent` to `init` so project and workspace initialization can directly set providers such as `claude` or `codex`
- Add `skills-seed init --workspace --children` so workspace initialization can also initialize child projects from `workspace.projects` when `.skills-seed` is missing
- Add `workspace.init_children`, default `false`; when enabled, `learn current` initializes missing child `.skills-seed` directories before learning
- Newly initialized workspace child projects inherit root `agent.provider`, `agent.commands`, and `output.skills_paths`; children that already use a different agent are reported and preserved

## [v0.0.7]

### Changes

- Make `generate-skills` always call the current Agent for summary merging and remove the `generation.mode` config field
- Enable CodeGraph `auto_init` by default so missing indexes are initialized automatically
- Simplify default `config.yaml` section headings for easier reading
- Fix workspace child-project task cancellation and share child container validation between learn/generate
- Consolidate learn-current excludes: built-in code now only protects `.git/**`, `.skills-seed/**`, `.claude/**`, and `.agents/**`; optional tool/project artifacts live in default `exclude`
- Use the glob-style `.*` default `exclude` to skip dot-prefixed files/directories at any depth, covering `.github`, `.cursor`, `.codegraph`, `.env`, and similar local/tool artifacts
- Translate English explanatory comments in source code and the Chinese config template while preserving required identifiers, command names, and English templates

## [v0.0.6]

### Features

- Add `--context` / `--context-file` to `learn current` and `generate-skills` for one-shot user guidance during a single run
- Make workspace root skill generation run extra workspace fact/profile and development-rule analysis when `generation.mode = ai` or runtime context is provided, then merge the result into root skill references
- Add structured workspace analysis for project responsibilities, frameworks/runtimes, child-project dependencies, impact routes, workspace-specific rules, change order, and parallel-agent boundaries
- Add Claude and Codex Agent support for workspace profile/spec analysis, parsed into `WorkspaceProfile` / `WorkspaceSpec`

### Templates

- Simplify the workspace root `SKILL.md` so it focuses on routing, child-skill selection, and cross-project rule decisions
- Expand `workspace-overview.md` with user-provided guidance, AI-analyzed workspace facts, dependencies, impact routes, responsibilities, and framework information
- Expand `cross-project-rules.md` with workspace-specific rules, routes, change order, multi-skill loading cases, and parallel-agent constraints
- Update learning, profile, and generation prompts to read large inputs and one-shot user guidance through file paths

### Experience

- Write large Agent inputs to temporary files under `.skills-seed/memory/runtime`, reducing rendered prompt size
- Mask root-level one-shot context while generating child-project skills from a workspace root, preventing workspace guidance from leaking into child skills
- Require an available Agent when runtime context is provided, even in the default template generation mode, so the context can affect generated output

### Documentation

- Remove outdated project architecture, generation pipeline, and incremental-learning design/plan documents

## [v0.0.5]

### Features

- Add md5-based incremental file learning to `learn current`, recording fingerprints after successful current-code learning
- Skip both pattern learning and project profile refresh when no learnable files changed
- Make workspace-root `learn current` enter each independent Git child repo and run incremental learning through that child's own `.skills-seed`
- Keep workspace-root storage limited to the workspace profile and cross-project relationship artifacts; child patterns and file fingerprints stay in child repos
- Refresh the existing profile for deleted-file-only runs without running pattern extraction
- Add `generation.mode` for `generate-skills`; default `template` avoids an extra AI call, while `ai` keeps pre-render summary merging
- Make workspace-root `generate-skills` enter each independent Git child repo first, generate child skills from each child's own `.skills-seed`, then generate the root workspace skill

### Experience

- Exclude configured skills output paths plus `.claude/skills/**` and `.agents/skills/**` by default to prevent generated content from feeding back into learning
- Pass existing pattern summaries into current-code learning prompts to reduce duplicate rules under new names
- Add incremental file statistics and generated-skills exclusion messages to learning output
- Do not overwrite an existing manual `SKILL.md` without a `generated-by: skills-seed` marker; workspace generation skips that child skill and still regenerates the root skill

### Documentation

- Update README, generation pipeline docs, and config templates for md5 incremental learning, workspace/child-repo decoupling, generation mode config, and generated-skills exclusion

## [v0.0.4]

### Features

- Limit workspace initialization discovery to first-level directories and expand common project marker support
- Generate the root workspace skill for the current `agent.provider`; child-project skills are generated by child repos themselves
- Make root workspace routing point to independently generated child skill paths, avoiding root writes into child output directories
- Generate provider metadata for the root workspace skill, including standard `agents/openai.yaml` for Codex output
- Treat child projects with `.skills-seed/config.yaml` as independently initialized, so the workspace root does not generate or overwrite their child skills

### Templates

- Expand the workspace root skill with a workspace map, impact-radius decisions, cross-project order, default special-path detection, and parallel write boundaries
- Expand `workspace-overview.md` and `cross-project-rules.md` so they provide default detection rules even when contracts/shared/infra paths are not configured
- Mark independently initialized child projects in the root workspace skill and overview, with instructions to use the child project's own `.skills-seed/config.yaml`

### Experience

- Keep template comments and double-quoted style when saving workspace config, avoiding full-file YAML marshal fallback
- Keep workspace root generation limited to the root workspace skill, avoiding overwrites of child repo agent configuration
- Align workspace child-project learning logs with single-project mode, including child start, analysis result, saved patterns, saved profile, and skip reasons
- Defer workspace child-project token usage output to the end of that child-project log block and include the child project name
- Make `learn current` token usage the final learning log line in project mode, and print workspace token usage after each child project's completion log to avoid concurrent log ordering drift

### Documentation

- Rewrite the README structure with single-project and workspace quick starts, mode locking, configuration, and common commands
- Update `docs/` architecture and generation pipeline documents, including matching English documents

## [v0.0.3]

### Features

- Add `skills-seed init --mode workspace` / `--workspace` for multi-project workspace initialization
- Add `skills-seed reset --mode ...` with automatic `.skills-seed` backup before reinitialization
- Add `project.mode`, `workspace.projects`, and `agent.parallelism` configuration
- Support concurrent child-project learning in workspace mode, with `project_id`, `scope_path`, and `workspace_role` written to learned patterns
- Generate root `.claude/.agents` workspace skills plus child-project `.claude/.agents` skills in workspace mode
- Generate project-level `project-spec.json` and `references/project-spec.md`, including independent specs for workspace child projects

### Templates

- Add `embedfs/templates/prompts/common/workspace-*` common workspace prompts
- Add `embedfs/templates/prompts/workspace/*` workspace initialization prompt templates
- Add `embedfs/templates/skills/common/workspace/*` root workspace skill and reference templates
- Expand workspace common prompts with strict JSON output, routing rules, impact radius, cross-project change order, and parallel-agent constraints
- Normalize top-level section comments in config templates so every module title is wrapped with `# ========================================`
- Keep child projects on existing `embedfs/templates/prompts/project/` and project skill templates, with generated content linking to `references/project-spec.md`

### Compatibility

- Lock the initialization mode after learning or skill generation starts to prevent project/workspace data shape drift

### Experience

- Adjust Agent token-usage console output order so it no longer interrupts the active progress step completion line

## [v0.0.2]

### Features

- Support incremental project profile refresh with `learn current --focus ... --profile refresh` using the existing project profile and focused paths
- Update the project profile analysis prompt to preserve unchanged modules, utilities, business methods, dependencies, and architecture details from the existing profile
- Add diagnostic logs for incremental project profile refresh so users can confirm whether the incremental path was used

### Documentation

- Add README examples for focused learning, local pattern learning, and project profile refresh commands
- Normalize Chinese and English Markdown documentation and Go comment style

### Experience

- Change init completion follow-up wording to optional next steps

## [v0.0.1]

Initial public release of Skills Seed

### Features

- Learn project-specific coding patterns from the current working tree or Git history
- Generate Claude Code, Codex, and common skill documentation from learned patterns
- Check staged code against learned rules and report actionable issues
- Provide interactive and automated patch-based fixes for detected issues
- Maintain local pattern, profile, memory, and log data under `.skills-seed`
- Support Chinese and English prompts, generated skills, configuration templates, and active UI messages
- Generate project profiles, module references, common utility references, and business-method references
- Support configurable output paths for Claude and Codex skill directories
- Track AI token usage during agent interactions
- Provide Git hook integration for pre-commit checks

### CLI Commands

- `skills-seed init`
- `skills-seed learn current`
- `skills-seed learn history`
- `skills-seed check`
- `skills-seed generate-skills`
- `skills-seed patterns merge`
- `skills-seed profile refresh`
- `skills-seed hook install pre-commit`
- `skills-seed patterns show`

### Distribution

- Add GitHub Actions CI for formatting, module consistency, `go vet`, and unit tests
- Add a simple GitHub Actions release workflow based on native `go build` commands
- Publish Linux, macOS, and Windows archives for x86_64 / arm64 where supported
- Include checksums and release notes in GitHub Releases
