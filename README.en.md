<div align="center">

# Skills Seed

**Learn project conventions from a codebase and generate local skills for Claude Code / Codex.**

[简体中文](README.md) · [English](README.en.md)

`Claude Code` · `Codex` · `Project Skills` · `Workspace`

[Quick Start](#quick-start) · [Agent Support](#agent-support) · [Command Reference](docs/COMMANDS.EN.md) · [Configuration Reference](docs/CONFIGURATION.EN.md)

</div>

Skills Seed is built for existing projects. It reads current code, Git history, and project structure, then turns real team practices into local knowledge: naming, error handling, directory layout, business methods, utilities, testing habits, and API conventions.

Learning data is stored locally under `.skills-seed` by default. Generated skills are rendered to `.claude/skills` or `.agents/skills` for the current `agent.provider`, so your AI assistant can load project-specific guidance without relying on a remote knowledge base.

## Capabilities

Skills Seed focuses on helping an AI assistant understand how this project should be changed:

1. Learn project conventions from current code, including patterns, business methods, utilities, and best practices.
2. Learn incrementally from Git history while skipping commits that were already analyzed.
3. Generate project profiles and project specs so the assistant understands module roles, dependencies, business boundaries, and change constraints.
4. Generate Claude Code / Codex skills with `SKILL.md`, project overviews, specs, patterns, and examples.
5. Support workspace roots where child projects learn and generate independently while the root skill handles routing and cross-project impact.
6. Support `check` and pre-commit hooks to apply learned rules to staged files or all Git-tracked files.

## Workflow

```text
init -> learn current / learn history -> generate-skills -> check
```

`init` creates `.skills-seed` and the default config. `learn` extracts project rules from code or commit history. `generate-skills` renders profiles and patterns into skills for the current Agent. `check` uses those rules to review future changes.

## Agent Support

Skills Seed currently includes two built-in providers:

- `claude`: the default Agent. Skills are generated to `.claude/skills/skills-seed-skills` for Claude Code.
- `codex`: skills are generated to `.agents/skills/skills-seed-skills` for Codex.

Choose the Agent during initialization:

```bash
skills-seed init --mode project --agent codex --locale en-US
skills-seed init --workspace --children --agent codex --locale en-US
```

`--agent` writes `agent.provider` and ensures matching entries exist in `agent.commands` and `output.skills_paths`. When initializing workspace children, newly created child projects inherit the root Agent config; existing child configs are preserved.

## Install

```bash
go install github.com/silaswei-io/skills-seed/cmd/skills-seed@latest
skills-seed --help
```

If the command is not found, add `$GOPATH/bin` or `$GOBIN` to `PATH`.

Build from source:

```bash
git clone https://github.com/silaswei-io/skills-seed.git
cd skills-seed
go build -o skills-seed ./cmd/skills-seed
```

## Requirements

- Go 1.25.6+
- A Git repository
- An available AI Agent CLI: default `claude`; use `--agent codex` to switch to Codex

## Quick Start

Single project:

```bash
cd your-project
skills-seed init --mode project --agent codex --locale en-US
skills-seed learn current
skills-seed generate-skills
```

Workspace:

```bash
cd your-workspace
skills-seed init --workspace --children --agent codex --locale en-US
skills-seed learn current
skills-seed generate-skills
```

Common commands:

```bash
skills-seed check
skills-seed profile show
skills-seed patterns merge --dry-run
skills-seed hook install
```

## Defaults

- Init mode is `project`.
- Default Agent is `claude`.
- Default data directory is `.skills-seed`.
- Claude output is `.claude/skills/skills-seed-skills`.
- Codex output is `.agents/skills/skills-seed-skills`.
- Workspace init scans only first-level child projects.
- `workspace.init_children` defaults to `false`; use `--children` or enable the config to initialize missing child projects.
- `agent.parallelism` defaults to `0`, meaning automatic concurrency: project=1, workspace=child project count capped at 6.

Existing child `.skills-seed/config.yaml` files are not overwritten. If a child project uses a different agent from the workspace root, it is reported and preserved.

## Documentation

- [Command Reference](docs/COMMANDS.EN.md)
- [Configuration Reference](docs/CONFIGURATION.EN.md)
- [Changelog](CHANGELOG.en.md)

## Development

```bash
go test ./...
go vet ./...
go build -o skills-seed ./cmd/skills-seed
```

## License

[MIT](LICENSE)
