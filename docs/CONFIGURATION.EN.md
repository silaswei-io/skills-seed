# Skills Seed Configuration Reference

[简体中文](CONFIGURATION.md) | [English](CONFIGURATION.EN.md)

The config file lives at `.skills-seed/config.yaml`. `skills-seed init` creates it from the project context. Most paths are relative to the project root or `.skills-seed`; each field below states the relevant base.

## 0.8.x Config Structure

0.8.x keeps the 0.7.x config shape and continues to avoid compatibility with old fields:

- Top-level `project` was renamed to `profile`. It describes the project or workspace that owns the config file; it is not the `project` run mode.
- `workspace` now keeps only `projects`; user-written `shared`, `contracts`, and `infra` fields were removed.
- Workspace shared libraries, contracts, and infrastructure impact are analyzed into workspace profile/spec during `learn current` from repository evidence, child project profiles, and one-shot user context. They are not read from config, and generation only consumes learned artifacts.
- Workspace root `profile.language` is empty by default because a workspace can contain child projects in multiple languages.
- `analysis.codegraph` was removed. Structural pre-scan is now configured through `learning.current.structural`, uses embedded tree-sitter, and does not require an external CodeGraph command or index.

## Config Example

### Default Structure

```yaml
profile:
  name: "your-project"
  mode: "project"
  language: ""
  locale: "en-US"
  git_remote: ""
  root_path: ""
  initialized_at: ""

workspace:
  projects: []

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
  current:
    mode: "normal"
    scope: "flow"
    parallelism: 1
    select_relevant_files: true
    select_relevant_files_min_candidates: 200
    structural:
      enabled: true
      max_symbols: 30
      max_file_size: 512
  history:
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
  logs_path: "runtime/logs"
  max_log_files: 30

exclude:
  gitignore: true
  paths:
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
| `language` | auto-detected or empty | Primary project language; left empty when init cannot detect it |
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
| `type` | auto-detected | Child project type, such as application, library, shared component, infrastructure, or contract project |
| `language` | auto-detected | Primary child project language |

#### Behavior

1. `skills-seed init --workspace` initializes the root and the child projects detected at that time.
2. For child projects added or copied into the workspace later, run `skills-seed workspace add .` to detect all children or `skills-seed workspace add <child>` for specific children.
3. Existing child `.skills-seed/config.yaml` files are not overwritten. If a child agent differs from the root, it is reported and preserved.
4. If a child has a `.skills-seed` directory but no `config.yaml`, the command fails instead of overwriting partial state.
5. Only first-level directories under the workspace root that have their own `.git` are recognized as child projects.
6. Markers such as `go.mod`, `package.json`, install scripts, Helm charts, and Terraform files classify `type` and `language`; they no longer decide whether a directory is a project.

### `learning.current`

`learning.current` controls the file scope and structural context used by `learn current`.

#### Fields

| Field | Default | Description |
|---|---:|---|
| `mode` | `normal` | Analysis depth: `fast` is quicker and merges nearby capabilities, `normal` balances quality and speed, `deep` is more thorough and keeps more business boundaries |
| `scope` | `flow` | Unit split scope: `domain` groups by business domain, `flow` splits by workflow/resource action, `module` allows finer module/plugin/API splits |
| `parallelism` | `1` | In-project analysis-unit parallelism; used by ordinary projects and workspace child projects, `1` means serial |
| `select_relevant_files` | `true` | Select the most relevant files from the candidate file tree before AI analysis to reduce noisy inputs |
| `select_relevant_files_min_candidates` | `200` | Only call AI file selection when the candidate count reaches this threshold; smaller projects use local filtering to avoid an extra AI call |
| `structural.enabled` | `true` | Enable structural context; even when enabled, it only runs when focus, diff, sample, or entry files are available |
| `structural.max_symbols` | `30` | Maximum symbols emitted into structural context |
| `structural.max_file_size` | `512` | Per-source-file size limit in KB; larger files are skipped |

#### `structural`

Lightweight structural pre-scan based on embedded tree-sitter. It provides symbols, imports, entry points, and module clues without depending on an external command or maintaining an index.

Starting in 0.7.1, structural pre-scan, `learn current`, and `preview` share the same file-selection policy: source files, build config, and dependency config are included by default, while documents, generated outputs, paths matched by global `exclude`, and generated Skills output directories are skipped.

Starting in 0.9.0, project-structure summaries, sample-file collection, and structural pre-scan all use the same configured file-selection policy. Except for built-in safety boundaries such as `.git`, `.skills-seed`, and configured generated-skills output directories, analyzer no longer keeps extra directory-name keywords. Put dependency, build-output, or project-specific directories in `exclude` when they should be skipped.

Starting in 0.9.1, `select_relevant_files` is enabled by default. When the locally filtered candidate count reaches `select_relevant_files_min_candidates`, `learn current` asks AI to select the most relevant files from the candidate file tree and change metadata before deeper analysis.

Starting in 0.9.11, file selection also applies Git ignore rules by default. Starting in 0.9.12, the Git ignore switch lives at `exclude.gitignore`. Set it to `false` when files ignored by `.gitignore` should still be analyzed. Starting in 0.9.13, snapshots still preserve the full current state, but diffs sent to AI are filtered by `exclude.paths` and `exclude.gitignore`, preventing ignored files from entering analysis as deleted diffs.

#### Recommendations

1. Most projects should keep the defaults; structural context still does not run without bounded inputs.
2. Set `select_relevant_files` to `false` when relevant-file selection is not needed.
3. Raise `select_relevant_files_min_candidates` for small projects to skip AI file selection, or lower it for large projects to narrow scope earlier.
4. Set `structural.enabled` to `false` when structural context is not needed.
5. Lower `structural.max_file_size` for large repositories to avoid generated files, bundles, or unusually large files.
6. Structural context only consumes bounded seed inputs and does not scan the whole repository when no seed exists.

### Prompt Runtime Debugging

Prompt fragments are still read from `.skills-seed/prompts/`, but starting in 0.7.1 rendering filters default metadata, empty scaffolding, and unfilled placeholder text. Only user-authored constraints are kept.

Rendered prompts are saved by default under `.skills-seed/runtime/rendered-prompts/` with a neighboring `.manifest.json`. The manifest records whether built-in, project profile, project fragment, workspace fragment, user instruction, and output-contract fragments were merged, plus raw and final lengths, so you can inspect the exact context sent to the Agent. Starting in 0.9.13, the final output contract is appended from a separate append template and forces JSON prompts to return exactly one parseable JSON object.

Starting in 0.10.5, `learn current` unit analysis no longer writes the existing pattern store into every unit prompt. To inspect stored patterns, read the local pattern store or use `patterns show` / `patterns stats`. In addition to the final output contract, parsing continues to apply programmatic JSON repair for issues such as raw newlines/control characters inside strings, bare object keys, and array items missing an object-start marker. Starting in 0.10.7, repair also covers trailing commas, comments, single-quoted strings, Python-style literals, and missing commas between object fields or array values.

Starting in 0.11.0, `learning.current.mode` can be set to `fast`, `normal`, or `deep` to choose between learning speed and pattern coverage quality; the mode is included in resume-state fingerprints. Generated skills now render related-reference routing, importance layers, validation matrices, grouped entry indexes, and path-validated source evidence.

Starting in 0.11.1, `learning.current.scope` can be set to `domain`, `flow`, or `module` to guide analysis-unit splitting by business domain, workflow, or module/plugin granularity, and it participates in resume-state fingerprints together with `mode`. Model-output parsing also repairs evidence line range expressions, normalizing invalid JSON such as `"line": 29-43` to a single line number.

The interactive init prompt asks for total Agent parallelism and writes concrete config fields automatically. In project mode it writes `learning.current.parallelism`; in workspace mode it splits the value across root `agent.parallelism` (child-project parallelism) and `learning.current.parallelism` (analysis-unit parallelism inside each child project), keeping their product within the total.

Starting in 0.8.0, Agent outputs are saved separately under `.skills-seed/runtime/agent-outputs/` by default, including final content, raw CLI output, stderr, and a manifest. Runtime logs keep only lengths and archive paths, and no longer include model reply previews or raw stdout/stderr. Starting in 0.10.3, valid JSON final content is formatted as a readable fenced `json` block inside the `.md` archive.

Starting in 0.9.6, debug records under `.skills-seed/runtime` use the `YYYYMMDD-HHMMSS.NNNNNNNNN-<kind>-<name>` filename prefix. `rendered-prompts/` and their matching `agent-outputs/` share the same date-time ID and semantic name; Agent output files only add the Agent name, making each prompt/output pair easy to correlate. Starting in 0.10.3, valid JSON output is formatted as a readable fenced `json` block inside the `.md` archive.

Starting in 0.9.0, the pattern store renders the `pattern-curate` prompt before storage so AI can deduplicate, consolidate, drop, and self-check candidate patterns against related historical patterns. Starting in 0.10.4, default pre-storage curation uses local deterministic merging and keeps its internal accepted set unique by pattern ID; candidates that reuse existing IDs, or historical stores that already contain duplicate IDs, are collapsed into one higher-quality pattern before writing. `generate skills` no longer runs pattern merging, so generation prompts only summarize and produce artifacts.

The current version no longer maintains skills dirty state. `sync` generates skills immediately after learning or adding a pattern, and explicit `skills-seed generate skills` deletes the old skills-seed generated output directory and fully rebuilds it.

### Generated Notice

The skills-seed generated footer in Skills templates is now controlled by an internal default and is omitted by default, reducing generated-content feedback into later learning. To inspect artifact provenance, use the `generated-by` metadata header or runtime logs.

### `agent`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `engine` | `claude` | Agent engine used for analysis, learning, and generation summaries; matches keys in `commands` |
| `commands` | `claude: claude`, `codex: codex` | Engine-to-CLI command mapping |
| `timeout` | `1800` | AI request timeout in seconds |
| `allow_user_plugins` | `false` | Whether agents may load user plugins; disabled by default for stable batch runs |
| `parallelism` | `0` | Workspace child project parallelism in workspace root configs; it does not control project-unit parallelism, `0` means automatic |
| `retry.max_retries` | `3` | Maximum retry attempts for retryable errors; `0` uses the default `3` |
| `retry.initial_interval` | `15` | Initial retry wait in seconds; `0` uses the default `15` |
| `retry.max_interval` | `120` | Maximum exponential-backoff wait in seconds; `0` uses the default `120` |

#### `parallelism` Notes

1. In `project` mode, `agent.parallelism` does not control in-project unit concurrency; use `learning.current.parallelism`.
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

### Workflow Resources

User workflows are not stored in `config.yaml` and are not part of `profile.mode`. The command sends explicitly provided goals, constraints, background, or paths to the current Agent, infers a standard workflow from them, saves the inferred body to `.skills-seed/workflows/<id>/WORKFLOW.md`, and stores original notes plus metadata in `metadata.yaml` in the same directory:

```bash
skills-seed workflow --context "Check environment variables and build artifacts before release, then run smoke tests after deployment"
```

When `--name` is omitted, the Agent generates an English workflow title from `--context` and uses its slug as `<id>`; repeated titles receive a numbered suffix. `--context` can be a goal, constraint, background note, path, or rough description; the Agent infers a standard workflow from that explicit input. Existing same-name workflows are merged and deduplicated by default; use `--overwrite` to replace one completely.

When skills are generated, workflows are written to output `workflows/`, and matching script directories are copied to `scripts/workflows/<id>/`.

### `learning.history`

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

### `.skills-seed` Layout

`.skills-seed/store/` is persistent data and should not be deleted. `.skills-seed/cache/` is rebuildable cache. `.skills-seed/runtime/` contains logs, rendered prompts, Agent outputs, and temporary inputs; it can be deleted when you do not need troubleshooting artifacts.

| Path | Purpose |
|---|---|
| `.skills-seed/store/project.db` | Indexed data such as patterns, hit stats, file fingerprints, and reviews |
| `.skills-seed/store/documents/` | Readable JSON documents such as profiles, specs, state, and changelog |
| `.skills-seed/cache/snapshots/` | Rebuildable file snapshot cache |
| `.skills-seed/cache/commands/<command>/state.json` | Resumable state for an unfinished command, such as `learn-current` or `sync`; safe to delete, then that command detects changes and plans again |
| `.skills-seed/runtime/logs/` | Runtime logs |
| `.skills-seed/runtime/rendered-prompts/` | Rendered prompts and manifests |
| `.skills-seed/runtime/agent-outputs/` | Archived Agent outputs |

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
| `logs_path` | `runtime/logs` | Log directory relative to `.skills-seed` |
| `max_log_files` | `30` | Maximum retained log files; older files are cleaned up automatically |

### `exclude`

`exclude` controls global file boundaries shared by learning, preview, project-structure summaries, sample-file collection, and structural pre-scan.

| Field | Default | Description |
|---|---:|---|
| `gitignore` | `true` | Exclude files matched by Git ignore rules, including `.gitignore`, `.git/info/exclude`, and the global Git ignore file |
| `paths` | See below | Relative paths or globs to exclude |

When `gitignore` is disabled, file selection still applies built-in safety boundaries, generated Skills output directories, and `exclude.paths`, but source files ignored by Git are no longer skipped just because of Git ignore rules.

#### Defaults

| Pattern | Description |
|---|---|
| `.*` | Dot-prefixed files and directories, such as `.github`, `.cursor`, `.env` |
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

1. `exclude.paths` uses glob-style patterns, not regular expressions. Patterns without `/` (e.g., `*.log`) match against both the file basename and the full path.
2. Exclusion rules affect learning, preview, project-structure summaries, sample-file collection, and structural pre-scan; `exclude.gitignore` is also applied by default.
3. Generated skill directories are also excluded by default, including configured `skills.paths`, `.claude/skills/**`, and `.agents/skills/**`.
