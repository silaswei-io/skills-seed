# Skills Seed

**Learn project conventions from a codebase and generate local skills for Claude Code / Codex.**

[简体中文](README.md) | [English](README.en.md)

Skills Seed analyzes current code, Git history, and project structure, turns existing team practices into local knowledge assets, then renders them to `.claude/skills` or `.agents/skills` for the current `agent.provider`. Data is stored locally under `.skills-seed` by default.

## Features

- Learn patterns, business methods, utilities, and best practices from the current codebase
- Learn incrementally from Git history and skip already analyzed commits
- Generate `project-profile.json` and `project-spec.json`
- Generate Claude Code / Codex skills with `SKILL.md` and `references/`
- Support single-project mode and multi-project workspace mode
- Generate a workspace root skill plus child-project skills so AI agents can route by path
- Support `check`, interactive fixes, and a pre-commit hook
- Support Chinese and English templates, prompts, config, and terminal output

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
- An available AI Agent CLI: default is `claude`; `codex` can be selected in config

## Quick Start: Single Project

```bash
cd your-project
skills-seed init --mode project --locale en-US
skills-seed learn current
skills-seed generate-skills
```

The default provider is `claude`, so output is:

```text
your-project/
├── .skills-seed/
│   ├── config.yaml
│   ├── memory/
│   │   ├── project.db
│   │   ├── project-profile.json
│   │   └── project-spec.json
│   └── logs/
└── .claude/skills/skills-seed-skills/
```

After changing `agent.provider` to `codex` in `.skills-seed/config.yaml`, output goes to `.agents/skills/...`.

## Quick Start: Workspace

Use workspace mode when one Git repository contains multiple child projects such as `frontend/`, `backend/`, `gateway/`, and `deploy/`.

```bash
cd your-workspace
skills-seed init --mode workspace --locale en-US
# Or: skills-seed init --workspace
```

Initialization scans only first-level folders under the workspace root and identifies child projects by common markers such as `package.json`, `go.mod`, `pyproject.toml`, `Cargo.toml`, `pom.xml`, `build.gradle`, `composer.json`, `Gemfile`, `Chart.yaml`, `Dockerfile`, and `openapi.yaml`. Review and adjust `.skills-seed/config.yaml`:

```yaml
project:
  mode: "workspace"

workspace:
  child_skill_policy: "skip_existing" # skip_existing | overwrite | root_only
  projects:
    - {id: "frontend", path: "frontend", type: "frontend", language: "typescript"}
    - {id: "backend", path: "backend", type: "backend", language: "go"}
  shared:
    - {path: "pkg"}
  contracts:
    - {path: "proto"}
  infra:
    - {path: "deploy"}

agent:
  parallelism: 0   # 0 means automatic: project=1, workspace=project count with a cap
```

Then run:

```bash
skills-seed learn current
skills-seed generate-skills
```

Workspace mode generates:

- Root skill for the current provider: workspace routing, cross-project rules, and impact radius
- Child-project skills: project spec, profile, and patterns for each project. If a child has its own `.skills-seed/config.yaml`, its provider and output path come from that child config
- `.skills-seed/memory/workspace-profile.json`
- `.skills-seed/memory/projects/<project-id>/project-profile.json`
- `.skills-seed/memory/projects/<project-id>/project-spec.json`

If a child project has its own `.skills-seed/config.yaml`, the workspace reads that child config's `agent.provider` and `output.skills_paths` to resolve the child skill path; otherwise it uses the workspace root config.

The default `child_skill_policy: "skip_existing"` keeps an existing `SKILL.md` at the path resolved from the child project's effective config; only the root workspace skill is generated/refreshed. Override it in config or for one run:

```bash
skills-seed generate-skills --overwrite # overwrite child-project skills at their resolved config paths
skills-seed generate-skills --root-only # generate only the root workspace skill
```

## Daily Commands

### Learn

```bash
# Learn from current code and create or refresh the project profile when needed
skills-seed learn current

# Learn only from a focused directory, without refreshing the profile
skills-seed learn current --focus internal/service --profile skip

# Focused learning with incremental profile refresh from the existing profile
skills-seed learn current --focus internal/service --profile refresh

# Learn from Git history; already learned commits are skipped
skills-seed learn history --limit=50
skills-seed learn history --since=30d
```

`--profile` values:

- `auto`: default. Refreshes for first/full learning; skips narrow changes when possible
- `skip`: learn patterns only
- `refresh`: refresh the profile from the current input

After the first successful `learn current`, Skills Seed records md5 fingerprints for analyzed files. Later runs compare those fingerprints first: when no learnable files changed, both pattern learning and project profile refresh are skipped; when files changed, only added, modified, or deleted paths drive incremental learning. Workspace mode scopes records per child project, so one child project's change does not re-learn the others.

Generated skills directories are excluded from learning by default, including configured `output.skills_paths`, `.claude/skills/**`, and `.agents/skills/**`. This prevents generated `SKILL.md` and `references/` files from feeding back into future learning.

`learn current` prints token usage after the learning log. In workspace mode, token usage is printed at the end of each child-project log block so concurrent learning does not interleave it with other projects.

### Profile And Spec

```bash
skills-seed profile show
skills-seed profile refresh
```

`profile refresh` rebuilds the project profile only. `project-spec.json` is generated by `generate-skills` from the profile and learned patterns.

### Generate Skills

```bash
skills-seed generate-skills

# Merge similar patterns explicitly when needed
skills-seed patterns merge
skills-seed generate-skills

# Temporarily override output path
skills-seed generate-skills --output .agents/skills/my-project

# workspace: overwrite existing child-project skills
skills-seed generate-skills --overwrite

# workspace: refresh only the root workspace skill
skills-seed generate-skills --root-only
```

Generated content:

```text
SKILL.md
agents/
references/
  project-overview.md
  project-spec.md
  patterns/*.md
  examples/*.md
```

### Check And Hook

```bash
# Check staged files by default
skills-seed check

# Check all Git-tracked files
skills-seed check --all

# Install pre-commit hook
skills-seed hook install
```

## Initialization Mode And Locking

Choose one mode at initialization:

```bash
skills-seed init --mode project
skills-seed init --mode workspace
```

After learning or skill generation starts, `project.mode` is locked and cannot be switched directly between `project` and `workspace`. To reinitialize:

```bash
skills-seed reset --mode workspace
```

`reset` backs up the old `.skills-seed` to `.skills-seed.backup/<timestamp>`.

## Configuration

Config file: `.skills-seed/config.yaml`. Common fields:

```yaml
project:
  name: "your-project"
  mode: "project"      # project or workspace
  language: "go"
  locale: "en-US"

analysis:
  codegraph:
    enabled: true       # Enable structural analysis by default; warn and fall back if codegraph is missing
    required: false     # true fails when CodeGraph is unavailable
    command: "codegraph"
    auto_init: false    # Run codegraph init -i automatically when the target project has no .codegraph
    auto_sync: true
    max_nodes: 30
    max_code: 0

agent:
  provider: "claude"
  commands:
    claude: "claude"
    codex: "codex"
  timeout: 1800
  allow_user_plugins: false
  parallelism: 0

learning:
  max_commits: 50
  batch_size: 5

output:
  skills_paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"

logging:
  level: "DEBUG"
  logs_path: "logs"
  max_log_files: 30
```

`analysis.codegraph.enabled` defaults to `true`. If `codegraph` is not installed, or the target project has no `.codegraph/` index, `required: false` makes `skills-seed` print a warning and continue with normal file-based analysis. Teams that require CodeGraph in CI can set `required` to `true`.

## Documentation

- [Project Architecture](docs/project-architecture.en.md)
- [Generation Pipeline](docs/project-generation-guide.en.md)
- [Changelog](CHANGELOG.en.md)

## Development

```bash
go test ./...
go vet ./...
go build -o skills-seed ./cmd/skills-seed
```

## License

[MIT](LICENSE)
