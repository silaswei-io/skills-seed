# Skills Seed Configuration Reference

[简体中文](CONFIGURATION.md) | [English](CONFIGURATION.EN.md)

The config file lives at `.skills-seed/config.yaml`. `skills-seed init` creates it from the project context. Most paths are relative to the project root or `.skills-seed`; each field below states the relevant base.

## Config Example

### Default Structure

```yaml
project:
  name: "your-project"
  mode: "project"
  language: "go"
  locale: "en-US"
  git_remote: ""
  root_path: ""
  initialized_at: ""

workspace:
  init_children: false
  projects: []
  shared: []
  contracts: []
  infra: []

analysis:
  codegraph:
    enabled: true
    required: false
    command: "codegraph"
    auto_init: true
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

autofix:
  strategy: "patch"
  backup_path: "backups"

output:
  skills_paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"

logging:
  level: "DEBUG"
  logs_path: "logs"
  max_log_files: 30

exclude:
  - ".*"
  - "vendor/**"
  - "node_modules/**"
  - "**/*.pb.go"
  - "**/*.gen.go"
  - "**/mocks/**"
  - "**/testdata/**"
```

## Config Sections

### `project`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `name` | current directory name | Project name, filled during init |
| `mode` | `project` | Init mode: `project` for a single project, `workspace` for a multi-project workspace |
| `language` | `go` | Primary project language, such as `typescript` or `python` |
| `locale` | auto-detect; fallback `zh-CN` | Language for CLI output, config templates, prompts, and skill templates |
| `git_remote` | auto-filled or empty | Git remote URL |
| `root_path` | current project absolute path | Written during init and used to locate the project root |
| `initialized_at` | init time | Initialization time |

#### Notes

1. `mode` is locked after learning or skill generation starts.
2. To choose another mode, run `skills-seed reset --mode project` or `skills-seed reset --mode workspace`.
3. `locale` supports `zh-CN` and `en-US`.

### `workspace`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `init_children` | `false` | Whether `learn current` should initialize child projects missing `.skills-seed` before learning |
| `projects` | `[]` | Child project list; workspace init tries to discover first-level project folders |
| `shared` | `[]` | Shared libraries or shared code paths |
| `contracts` | `[]` | API, IDL, or protocol contract paths |
| `infra` | `[]` | Deployment, operations, or infrastructure paths |

#### `projects` Fields

| Field | Default | Description |
|---|---:|---|
| `id` | normalized directory name | Unique child project id |
| `path` | discovered relative path | Child project path relative to the workspace root |
| `type` | auto-detected | Child role, such as `backend`, `frontend`, `infra`, or `contracts` |
| `language` | auto-detected | Primary child project language |

#### `shared` / `contracts` / `infra` Fields

| Field | Default | Description |
|---|---:|---|
| `path` | none | Path relative to the workspace root |
| `description` | empty | Purpose of the path |

#### Behavior

1. `skills-seed init --workspace` initializes only the workspace root.
2. `skills-seed init --workspace --children` initializes the root and child projects from `workspace.projects` when `.skills-seed` is missing.
3. When `workspace.init_children: true`, `skills-seed learn current` initializes missing child `.skills-seed` directories before learning.
4. Existing child `.skills-seed/config.yaml` files are not overwritten. If a child agent differs from the root, it is reported and preserved.

### `analysis.codegraph`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `enabled` | `true` | Enable CodeGraph structural analysis enhancement |
| `required` | `false` | Fail when CodeGraph is unavailable; `false` warns and falls back to normal file analysis |
| `command` | `codegraph` | CodeGraph command path |
| `auto_init` | `true` | Run `codegraph init -i` automatically when the target project has no `.codegraph` |
| `auto_sync` | `true` | Run `codegraph sync` before analysis when an index exists |
| `max_nodes` | `30` | Maximum symbol nodes passed to `codegraph context` |
| `max_code` | `0` | Maximum code blocks passed to `codegraph context`; `0` means structural summary only |

#### Recommendations

1. Keep the defaults for local development.
2. In CI or strict team environments, set `required: true` if CodeGraph is mandatory.
3. Set `auto_init` or `auto_sync` to `false` if indexing should be controlled manually.

### `agent`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `provider` | `claude` | Current agent provider; matches keys in `commands` and `output.skills_paths` |
| `commands` | `claude: claude`, `codex: codex` | Provider-to-CLI command mapping |
| `timeout` | `1800` | AI request timeout in seconds |
| `allow_user_plugins` | `false` | Whether agents may load user plugins; disabled by default for stable batch runs |
| `parallelism` | `0` | Number of concurrent agents; `0` means automatic |

#### `parallelism` Notes

1. In `project` mode, automatic parallelism is `1`.
2. In `workspace` mode, automatic parallelism is the child project count, capped at `6`.
3. A positive value is used as the explicit concurrency limit.
4. The implementation is real concurrency: child project tasks run through a goroutine worker pool.

#### Switch Agent

```yaml
agent:
  provider: "codex"
  commands:
    claude: "claude"
    codex: "codex"

output:
  skills_paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"
```

You can also set the agent during initialization:

```bash
skills-seed init --mode project --agent codex
skills-seed init --workspace --children --agent codex
```

### `learning`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `max_commits` | `50` | Default maximum number of Git commits analyzed by `learn history` |
| `batch_size` | `5` | Number of commits per AI call when learning history in batches |

#### Command Overrides

```bash
skills-seed learn history --limit 100 --batch-size 10
```

Command flags affect only the current run and do not rewrite the config file.

### `autofix`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `strategy` | `patch` | Auto-fix strategy: `patch`, `backup`, `stash`, or `branch` |
| `backup_path` | `backups` | Backup path relative to `.skills-seed` |

#### Strategies

1. `patch`: generate patch files, recommended by default.
2. `backup`: back up original files before modification.
3. `stash`: apply fixes and save them through Git stash.
4. `branch`: create a new branch for fixes.

### `output`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `skills_paths.claude` | `.claude/skills/skills-seed-skills` | Claude Code skills output directory |
| `skills_paths.codex` | `.agents/skills/skills-seed-skills` | Codex skills output directory |

#### Notes

1. `generate-skills` uses `output.skills_paths` for the current `agent.provider` by default.
2. Use `skills-seed generate-skills --output <path>` to override the output directory for one run.
3. For a custom provider, add both `agent.commands.<provider>` and `output.skills_paths.<provider>`.

### `logging`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `level` | `DEBUG` | Log level: `DEBUG`, `INFO`, `WARN`, or `ERROR` |
| `logs_path` | `logs` | Log directory relative to `.skills-seed` |
| `max_log_files` | `30` | Maximum retained log files; older files are cleaned up automatically |

### `exclude`

#### Defaults

| Pattern | Description |
|---|---|
| `.*` | Dot-prefixed files and directories, such as `.github`, `.cursor`, `.codegraph`, `.env` |
| `vendor/**` | Go dependency directory |
| `node_modules/**` | Node.js dependency directory |
| `**/*.pb.go` | Protobuf generated files |
| `**/*.gen.go` | Generated code files |
| `**/mocks/**` | Mock test files |
| `**/testdata/**` | Test data directories |

#### Notes

1. `exclude` uses glob-style patterns, not regular expressions.
2. Exclusion rules affect learning and analysis.
3. Generated skill directories are also excluded by default, including configured `output.skills_paths`, `.claude/skills/**`, and `.agents/skills/**`.
