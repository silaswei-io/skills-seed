# Skills Seed Configuration Reference

[简体中文](CONFIGURATION.md) | [English](CONFIGURATION.EN.md)

The config file lives at `.skills-seed/config.yaml`. `skills-seed init` creates it from the project context. Most paths are relative to the project root or `.skills-seed`; each field below states the relevant base.

## 0.6.1 Config Structure

0.6.1 keeps the clean 0.6.0 config structure and does not keep compatibility with old fields:

- Top-level `project` was renamed to `profile`. It describes the project or workspace that owns the config file; it is not the `project` run mode.
- `workspace` now keeps only `projects`; user-written `shared`, `contracts`, and `infra` fields were removed.
- Workspace shared libraries, contracts, and infrastructure impact are analyzed into workspace profile/spec during `learn current` from repository evidence, child project profiles, and one-shot user context. They are not read from config, and generation only consumes learned artifacts.
- Workspace root `profile.language` is empty by default because a workspace can contain child projects in multiple languages.

## Config Example

### Default Structure

```yaml
profile:
  name: "your-project"
  mode: "project"
  language: "go"
  locale: "en-US"
  git_remote: ""
  root_path: ""
  initialized_at: ""

workspace:
  projects: []

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
  retry:
    max_retries: 3
    initial_interval: 15
    max_interval: 120

learning:
  max_commits: 50
  batch_size: 5

autofix:
  strategy: "patch"
  backup_path: "backups"

skills:
  target: "claude"
  locale: "en-US"
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

### `profile`

`profile` describes the project or workspace that owns this config file; it is not the `project` run mode

#### Fields

| Field | Default | Description |
|---|---:|---|
| `name` | current directory name | Project name, filled during init |
| `mode` | `project` | Init mode: `project` for a single project, `workspace` for a multi-project workspace |
| `language` | `go` | Primary project language, such as `typescript` or `python` |
| `locale` | `zh-CN` | Language for tool output and config templates |
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

#### `projects` Fields

| Field | Default | Description |
|---|---:|---|
| `id` | normalized directory name | Unique child project id |
| `path` | discovered relative path | Child project path relative to the workspace root |
| `type` | auto-detected | Child project type, such as `backend`, `frontend`, `library`, `infra`, or `contracts` |
| `language` | auto-detected | Primary child project language |

#### Behavior

1. `skills-seed init --workspace` initializes the root and the child projects detected at that time.
2. For child projects added or copied into the workspace later, run `skills-seed workspace add .` to detect all children or `skills-seed workspace add <child>` for specific children.
3. Existing child `.skills-seed/config.yaml` files are not overwritten. If a child agent differs from the root, it is reported and preserved.
4. If a child has a `.skills-seed` directory but no `config.yaml`, the command fails instead of overwriting partial state.
5. Only first-level directories under the workspace root that have their own `.git` are recognized as child projects.
6. Markers such as `go.mod`, `package.json`, install scripts, Helm charts, and Terraform files classify `type` and `language`; they no longer decide whether a directory is a project.

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
| `retry.max_retries` | `3` | Maximum retry attempts for retryable errors; `0` uses the default `3` |
| `retry.initial_interval` | `15` | Initial retry wait in seconds; `0` uses the default `15` |
| `retry.max_interval` | `120` | Maximum exponential-backoff wait in seconds; `0` uses the default `120` |

#### `parallelism` Notes

1. In `project` mode, automatic parallelism is `1`.
2. In `workspace` mode, automatic parallelism is the child project count, capped at `6`.
3. A positive value is used as the explicit concurrency limit.
4. The implementation is real concurrency: child project tasks run through a goroutine worker pool.

#### `retry` Notes

1. Retry currently applies to retryable Agent CLI errors such as 429 / 529 / overloaded.
2. Wait time starts at `initial_interval`, doubles after each retry, and is capped by `max_interval`.
3. Long-running steps such as `learn current` update the active progress line with the agent error, failed call duration, and backoff wait; when the next call starts, the line switches to `attempt N`.

#### Switch Agent

```yaml
agent:
  engine: "claude"
  commands:
    claude: "claude"
    codex: "codex"

skills:
  target: "codex"
  locale: "en-US"
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

`--context` and `--context-file` are one-time learning flags. They affect only the current `learn current` run, are not written to `.skills-seed/prompts/`, and are not passed to `generate skills`. Put long-lived rules in `prompts/instructions/<prompt-id>.md`; use `learn current --context` or `learn current --context-file` for temporary guidance.

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
| `locale` | `en-US` | Generated Skills, AI prompts, and natural-language content persisted into Skills |
| `paths.claude` | `.claude/skills/skills-seed-skills` | Claude Code skills output directory |
| `paths.codex` | `.agents/skills/skills-seed-skills` | Codex skills output directory |

#### Notes

1. `generate skills` uses `skills.paths` for the current `skills.target` by default.
2. Use `skills-seed generate skills --output <path>` to override the output directory for one run.
3. `skills.locale` supports `zh-CN` and `en-US` and defaults to English; `profile.locale` no longer controls AI prompt or Skills content language.
4. For a custom engine or target, add `agent.commands.<engine>` and `skills.paths.<target>` respectively.

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
