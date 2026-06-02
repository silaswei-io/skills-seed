# Changelog

[简体中文](CHANGELOG.md) | [English](CHANGELOG.en.md)

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
- `skills-seed view`

### Distribution

- Add GitHub Actions CI for formatting, module consistency, `go vet`, and unit tests
- Add a simple GitHub Actions release workflow based on native `go build` commands
- Publish Linux, macOS, and Windows archives for x86_64 / arm64 where supported
- Include checksums and release notes in GitHub Releases
