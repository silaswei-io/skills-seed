<div align="center">

# Skills Seed

**Make AI Agents understand project rules before changing code.**

[![CI](https://img.shields.io/github/actions/workflow/status/silaswei-io/skills-seed/ci.yml?branch=main&label=ci&logo=github&style=flat-square)](https://github.com/silaswei-io/skills-seed/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/silaswei-io/skills-seed?style=flat-square)](https://github.com/silaswei-io/skills-seed/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/silaswei-io/skills-seed?style=flat-square)](go.mod)
[![License](https://img.shields.io/github/license/silaswei-io/skills-seed?style=flat-square)](LICENSE)

[简体中文](README.md) · [English](README.en.md)

[Wiki](docs/wiki/en/Home.md) · [Getting Started](docs/wiki/en/Getting-Started.md) · [Command Reference](docs/COMMANDS.EN.md) · [Configuration Reference](docs/CONFIGURATION.EN.md)

</div>

Skills Seed is for existing repositories. It turns user-maintained project constraints, current source, and verifiable project knowledge into local Skills, so Claude Code, Codex, and similar Agents understand rules, ownership, reusable capabilities, and impact boundaries before editing.

It is not a remote knowledge base, and it does not replace code review, tests, or owner judgment. Learned results, user rules, runtime archives, and generated output stay local to the project by default, where they can be reviewed, refreshed, and committed.

## Quick Start

Run this at a Git project root:

```bash
cd your-project
skills-seed init
skills-seed sync
```

Use `skills-seed init --skills-name team-guide` to choose a custom Skill name during initialization; omit it to derive the name from the project.

First download the binary for the current system and architecture from the [latest release](https://github.com/silaswei-io/skills-seed/releases/latest) and place it on `PATH`. `init` selects project mode, analysis Agent, and output target. `sync` learns current code and refreshes generated Skills. Later, `skills-seed update` downloads and verifies an official release without requiring Go. See the [Getting Started Wiki page](docs/wiki/en/Getting-Started.md) for prerequisites, verification, and choosing between project and workspace mode.

## What It Provides

- **Explicit rules first**: Teams keep constraints and command boundaries that source code cannot derive as Rules.
- **Evidence-backed project knowledge**: Current source, structure, and change evidence produce module maps, capability entries, and reusable patterns.
- **Modular Skills**: Agents read a concise entry, then load references, rules, and workflows only when needed.
- **Continuous sync**: Incremental learning, focus review, and recoverable state refresh knowledge as the repository evolves.
- **Workspace routing**: Roots handle cross-project relations while children keep independent knowledge and generated output.

## Documentation

| Need | Read |
|---|---|
| First setup, daily sync, recovery, and troubleshooting | [Wiki](docs/wiki/en/Home.md) |
| All commands and flags | [Command Reference](docs/COMMANDS.EN.md) |
| Configuration fields, defaults, and runtime directories | [Configuration Reference](docs/CONFIGURATION.EN.md) |
| A real generated result | [Medusa Demo Case Study](docs/MEDUSA_DEMO_CASE.EN.md) |
| Product acceptance boundary and maintainer constraints | [Ultimate Goal](docs/ULTIMATE_GOAL.md) |
| Version changes | [Changelog](CHANGELOG.en.md) |
| Contributing | [Contributing Guide](CONTRIBUTING.en.md) |

See the [Wiki source guide](docs/wiki/README.md) for the ownership and maintenance contract of each document type.

## Development

```bash
go test ./...
go vet ./...
staticcheck ./...
go build ./cmd/skills-seed
```

---

<div align="center">

Released under the [MIT License](LICENSE).

</div>
