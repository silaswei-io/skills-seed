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
  engine: "claude"
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

skills:
  target: "claude"
  paths:
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
  - "dist/**"
  - "build/**"
  - "out/**"
  - "target/**"
  - "coverage/**"
  - ".cache/**"
  - "tmp/**"
  - "temp/**"
  - "*.log"
  - "*.tmp"
  - "*.bak"
  - "*.swp"
  - "*.zip"
  - "*.tar"
  - "*.tar.gz"
  - "*.tgz"
  - "*.rar"
  - "*.7z"
  - "*.png"
  - "*.jpg"
  - "*.jpeg"
  - "*.gif"
  - "*.webp"
  - "*.ico"
  - "*.pdf"
  - "*.mp4"
  - "*.mov"
```

## Config Sections

### `project`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `name` | current directory name | Project name, filled during init |
| `mode` | `project` | Init mode: `project` for a single project, `workspace` for a multi-project workspace |
| `language` | `go` | Primary project language, such as `typescript` or `python` |
| `locale` | `zh-CN` | Language for CLI output, config templates, prompts, and skill templates |
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

1. `skills-seed init --workspace` initializes the root and the child projects detected at that time.
2. For child projects added or copied into the workspace later, run `skills-seed add .` to detect all children or `skills-seed add <child>` for specific children.
3. Existing child `.skills-seed/config.yaml` files are not overwritten. If a child agent differs from the root, it is reported and preserved.
4. If a child has a `.skills-seed` directory but no `config.yaml`, the command fails instead of overwriting partial state.

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
| `engine` | `claude` | Agent engine used for analysis, learning, and generation summaries; matches keys in `commands` |
| `commands` | `claude: claude`, `codex: codex` | Engine-to-CLI command mapping |
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

You can also set the agent during initialization:

```bash
skills-seed init --mode project --agent codex
skills-seed init --workspace --agent codex
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

### `.skills-seed/prompts/`

`.skills-seed/prompts/` is not a `config.yaml` field, but it is created by `skills-seed init` as editable runtime prompt fragments for the project. Use it for persistent project notes, workspace constraints, and user instructions.

Common paths:

| Path | Purpose |
|---|---|
| `.skills-seed/prompts/project/project-profile.md` | Project facts merged into related prompts |
| `.skills-seed/prompts/project/common.md` | Common project constraints merged into related prompts |
| `.skills-seed/prompts/project/<prompt-id>.md` | Optional project-level fragment for one prompt |
| `.skills-seed/prompts/workspace/<prompt-id>.md` | Workspace-level fragment, for example `skill-workspace-profile.md` |
| `.skills-seed/prompts/instructions/<prompt-id>.md` | User instructions appended to one prompt |

These files are merged with built-in prompts; they do not replace built-in prompts. Skills Seed appends a built-in final output contract after the merged fragments to protect the JSON / Markdown format expected by parsers.

`--context` and `--context-file` are one-time command flags. They affect only the current `learn current` or `generate-skills` run and are not written to `.skills-seed/prompts/`. Put long-lived rules in `prompts/instructions/<prompt-id>.md`; use `--context` or `--context-file` for temporary guidance.

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

### `skills`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `target` | `agent.engine` | Generated Skills target type; can differ from `agent.engine` |
| `paths.claude` | `.claude/skills/skills-seed-skills` | Claude Code skills output directory |
| `paths.codex` | `.agents/skills/skills-seed-skills` | Codex skills output directory |

#### Notes

1. `generate-skills` uses `skills.paths` for the current `skills.target` by default.
2. Use `skills-seed generate-skills --output <path>` to override the output directory for one run.
3. For a custom engine or target, add `agent.commands.<engine>` and `skills.paths.<target>` respectively.

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
| `vendor/**` | Common dependency directory |
| `node_modules/**` | Common dependency directory |
| `dist/**` | Common build output directory |
| `build/**` | Common build output directory |
| `out/**` | Common output directory |
| `target/**` | Common build output directory |
| `coverage/**` | Coverage report directory |
| `.cache/**` | Cache directory |
| `tmp/**` | Temporary directory |
| `temp/**` | Temporary directory |
| `*.log` | Log files |
| `*.tmp` | Temporary files |
| `*.bak` | Backup files |
| `*.swp` | Editor swap files |
| `*.zip` / `*.tar` / `*.tar.gz` / `*.tgz` / `*.rar` / `*.7z` | Archives |
| `*.png` / `*.jpg` / `*.jpeg` / `*.gif` / `*.webp` / `*.ico` | Image assets |
| `*.pdf` | Document outputs |
| `*.mp4` / `*.mov` | Video assets |

#### Notes

1. `exclude` uses glob-style patterns, not regular expressions. Patterns without `/` (e.g., `*.log`) match against both the file basename and the full path.
2. Exclusion rules affect learning and analysis.
3. Generated skill directories are also excluded by default, including configured `skills.paths`, `.claude/skills/**`, and `.agents/skills/**`.
