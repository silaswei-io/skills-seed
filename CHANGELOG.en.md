# Changelog

[简体中文](CHANGELOG.md) | [English](CHANGELOG.en.md)

## [v0.0.4]

### Features

- Limit workspace initialization discovery to first-level directories and expand common project marker support
- Generate the root workspace skill and child-project skills for the current `agent.provider`
- Make root workspace routing point only to child skills for the current provider, so Claude generation no longer also writes Codex output
- Generate provider metadata for the root workspace skill, including standard `agents/openai.yaml` for Codex output
- Add `workspace.child_skill_policy` with `skip_existing`, `overwrite`, and `root_only` for child-project skill generation
- Add `generate-skills --overwrite` and `--root-only` to override the workspace child-skill policy for one run
- Treat child projects with `.skills-seed/config.yaml` as independently initialized, so the workspace root does not generate or overwrite their child skills

### Templates

- Expand the workspace root skill with a workspace map, impact-radius decisions, cross-project order, default special-path detection, and parallel write boundaries
- Expand `workspace-overview.md` and `cross-project-rules.md` so they provide default detection rules even when contracts/shared/infra paths are not configured
- Mark independently initialized child projects in the root workspace skill and overview, with instructions to use the child project's own `.skills-seed/config.yaml`

### Experience

- Keep template comments and double-quoted style when saving workspace config, avoiding full-file YAML marshal fallback
- Skip existing child-project skills for the current provider by default while still generating the root workspace skill
- Add i18n text for CLI flag help and workspace generation logs
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
