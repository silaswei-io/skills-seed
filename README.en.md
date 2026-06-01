<div align="center">

# Skills Seed

**Make AI Agents remember your project rules.**

[![CI](https://img.shields.io/github/actions/workflow/status/silaswei-io/skills-seed/ci.yml?branch=main&label=ci&logo=github&style=flat-square)](https://github.com/silaswei-io/skills-seed/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/silaswei-io/skills-seed?style=flat-square)](https://github.com/silaswei-io/skills-seed/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/silaswei-io/skills-seed?style=flat-square)](go.mod)
[![License](https://img.shields.io/github/license/silaswei-io/skills-seed?style=flat-square)](LICENSE)

[简体中文](README.md) · [English](README.en.md)

`Claude Code` · `Codex` · `Local Skills` · `Workspace` · `Code Review`

[Quick Start](#quick-start) · [Output Preview](#output-preview) · [Design Principles](#design-principles) · [Workspace](#workspace) · [Command Reference](docs/COMMANDS.EN.md)

</div>

Skills Seed is built for existing projects. It reads current code, Git history, project structure, and recorded check hits, then turns real team practices into local knowledge: naming, error handling, directory layout, business methods, utilities, testing habits, and API conventions.

It solves a specific problem: when an AI Agent enters a project for the first time, it usually does not know how this project should be changed. Skills Seed extracts those implicit rules from real code and turns them into local skills that can be loaded, refreshed, and checked.

What you get is not a generic project summary. It is Agent working context generated for the current project:

- Which directories own which capabilities, and where to look first for a change.
- Which business methods, utilities, error-handling patterns, and test habits are already established.
- Which child project in a workspace should handle a request, and how cross-project changes should be inspected.
- Which rules repeatedly appear in `check` or review and should be prioritized in generated skills.

All data is local by default. The generated skills type is selected by `skills.target`; the Agent CLI used for analysis, learning, and summaries is selected by `agent.engine`. That means you can use `claude` for analysis and summarization while producing `codex` skills.

## Why Use It

| Problem | What Skills Seed Does |
|---|---|
| You need to explain the same project structure to AI repeatedly | Generates reusable skills from code, Git history, and project profiles |
| Team conventions live in code, reviews, and individual memory | Extracts patterns, business methods, utilities, and test habits |
| A multi-project workspace makes context selection unclear | Root skill handles routing; child skills stay independent |
| Generated skills could feed back into later learning | Excludes `.skills-seed`, skills outputs, and build artifacts by default |
| You do not know whether generated rules are useful | Records pattern hits from `check`; `patterns stats` shows quality and usage |

## Use Cases

- You want an AI Agent to work on an existing business system without re-explaining the project structure and constraints every time.
- Your team has stable conventions for naming, layout, error handling, business methods, and tests, and you want those conventions available to the assistant.
- Your workspace contains multiple independent Git child projects, and you want child projects to learn and generate independently while the root skill handles routing and cross-project impact.
- You want `check` / pre-commit hooks to apply learned rules to future changes and record which rules are actually hit.
- You want to import local review comments and see which recurring review issues are already covered by learned patterns.

## Output Preview

After `generate-skills`, the default output looks like this:

```text
.agents/skills/skills-seed-skills/
├── SKILL.md
├── agents/
│   └── openai.yaml
└── references/
    ├── project-overview.md
    ├── project-spec.md
    ├── business-methods.md
    ├── modules.md
    └── patterns/
        ├── business.md
        ├── error.md
        └── testing.md
```

`SKILL.md` is the Agent entry point. `references/` keeps the fuller project profile, specs, and pattern details. The Agent can read references when it needs depth, instead of loading every detail into the entry document.

## Workflow

```text
init -> learn current / learn history -> generate-skills -> check
```

| Stage | Command | Output |
|---|---|---|
| Initialize | `skills-seed init` | `.skills-seed/config.yaml`, local database, default prompts |
| Learn current code | `skills-seed learn current` | patterns, business methods, utilities, project profile |
| Learn history | `skills-seed learn history` | long-lived rules extracted from Git evolution |
| Generate skills | `skills-seed generate-skills` | `SKILL.md`, project overview, specs, pattern references |
| Check later changes | `skills-seed check` | issues, fix suggestions, and pattern hits based on learned rules |

`generate-skills` ranks learned patterns by quality: rules with higher effective score, more check hits, and higher confidence are favored, reducing generic or duplicated rules in the final skills.

## Quick Start

### Single Project

```bash
cd your-project
skills-seed init --mode project --agent codex --locale en-US
skills-seed learn current
skills-seed generate-skills
test -f .agents/skills/skills-seed-skills/SKILL.md
```

For Codex, the default generated skill is:

```text
.agents/skills/skills-seed-skills/SKILL.md
```

For Claude Code, the default generated skill is:

```text
.claude/skills/skills-seed-skills/SKILL.md
```

### Workspace

Workspace mode is for a root directory that contains multiple independent Git child projects. During initialization, Skills Seed scans first-level directories, detects child projects, and initializes `.skills-seed` for the children found at that time.

```bash
cd your-workspace
skills-seed init --workspace --agent codex --locale en-US
skills-seed learn current
skills-seed generate-skills
test -f .agents/skills/skills-seed-skills/SKILL.md
```

If a new project is copied into the workspace root later, use `add` to sync config and initialize the child repo:

```bash
skills-seed add .
skills-seed add backend frontend
```

The workspace root coordinates routing and cross-project relationships only. Child projects use their own `.skills-seed` directories to learn, generate, and store patterns independently. Existing child `.skills-seed/config.yaml` files are never overwritten; if a child uses a different agent from the root, it is reported and preserved.

## Design Principles

- **Local-first**: learned data, config, and generated output stay in the current repository by default.
- **Existing-code-first**: real code, Git history, and check hits are the source of truth, instead of requiring users to hand-write a large project guide.
- **Agent-agnostic**: `agent.engine` and `skills.target` are separate, so one Agent can analyze while another Agent's skill format is generated.
- **Workspace-aware**: root workspaces handle routing and cross-project relationships; child repos learn and generate independently.
- **Feedback-driven**: `check` and review hits feed back into pattern quality, making useful rules more likely to reach the final skills.

## Agent And Skills Target

`agent.engine` selects the Agent CLI used for analysis, learning, and generation summaries. `skills.target` selects which Agent skill format to generate.

For example, use Claude for analysis and summarization while generating Codex skills:

```yaml
agent:
  engine: "claude"
  commands:
    claude: "claude"
    codex: "codex"

skills:
  target: "codex"
  paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"
```

Built-in targets:

| Name | Purpose | Default Output |
|---|---|---|
| `claude` | Claude Code skills | `.claude/skills/skills-seed-skills` |
| `codex` | Codex skills | `.agents/skills/skills-seed-skills` |

## Common Commands

| Command | Description |
|---|---|
| `skills-seed init` | Initialize a single project or workspace root |
| `skills-seed add .` | Auto-detect and add all child projects from a workspace root |
| `skills-seed add <child...>` | Add specific child projects from a workspace root |
| `skills-seed learn current` | Incrementally learn rules and profile from current code |
| `skills-seed learn history` | Learn long-lived rules from Git history |
| `skills-seed generate-skills` | Generate skills for the current `skills.target` |
| `skills-seed check` | Check staged files or Git-tracked files |
| `skills-seed patterns stats` | Show pattern quality, hit counts, and recent hits |
| `skills-seed review import --from-file` | Import local review comments |
| `skills-seed hook install` | Install the local pre-commit hook |

See [Command Reference](docs/COMMANDS.EN.md) for all flags and forms.

## Local And Safety Boundaries

- Project code is not uploaded to a remote knowledge base by default; learned data is written to `.skills-seed` in the current repository.
- `check` and `generate-skills` call the configured Agent CLI, so network behavior depends on the `claude` / `codex` CLI you use.
- Generated skills directories, `.git/**`, `.skills-seed/**`, and common build outputs are excluded by default so generated content does not feed back into later learning.
- A handwritten `SKILL.md` without a `generated-by: skills-seed` marker is not overwritten by default.

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
- An available AI Agent CLI: default `claude`; switch with `--agent codex` or `agent.engine`

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

---

<div align="center">

Released under the [MIT License](LICENSE).

</div>
