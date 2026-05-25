# Changelog

[简体中文](CHANGELOG.md) | [English](CHANGELOG.en.md)

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
