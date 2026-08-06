# Skills Seed Configuration Reference

[简体中文](CONFIGURATION.md) | [English](CONFIGURATION.EN.md)

The config file lives at `.skills-seed/config.yaml`. `skills-seed init` creates it from the project context. Most paths are relative to the project root or `.skills-seed`; each field below states the relevant base.

## 0.8.x Config Structure

0.8.x keeps the 0.7.x config shape and continues to avoid compatibility with old fields:

- Top-level `project` was renamed to `profile`. It describes the project or workspace that owns the config file; it is not the `project` run mode.
- `workspace` now keeps only `projects`; user-written `shared`, `contracts`, and `infra` fields were removed.
- Workspace shared libraries, contracts, and infrastructure impact are analyzed into workspace profile/spec during `learn current` from repository evidence, child project profiles, and one-shot user context. They are not read from config, and generation only consumes learned artifacts.
- Workspace root `profile.language` is empty by default because a workspace can contain child projects in multiple languages.
- `analysis.codegraph` was removed. Structural context and symbol verification are now configured through `learning.current.structural`; the default `provider: auto` prefers CodeGraph and falls back to embedded tree-sitter when CodeGraph is unavailable.

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
  model: ""
  retry:
    max_retries: 3
    initial_interval: 15
    max_interval: 120

learning:
  current:
    mode: "normal"
    scope: "flow"
    max_focuses_per_call: 1
    structural:
      enabled: true
      provider: "auto"
      max_symbols: 30
      max_file_size: 512
skills:
  target: "claude"
  locale: "en-US"
  paths:
    claude: ".claude/skills/<project-name>-dev"
    codex: ".agents/skills/<project-name>-dev"

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
| `locale` | `zh-CN` | Language for tool output, config templates, and seed context templates |
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
| `mode` | `normal` | Learning strategy: `normal` balances quality and speed; `fast` keeps compact directly evidenced patterns; `deep` keeps more source-backed local business/code patterns |
| `scope` | `flow` | Focus planning lens: `flow` is the default stable choice for workflows/resource actions; `domain` favors long-lived business responsibilities; `module` favors module/plugin/contract boundaries. It guides learning perspective, not a fixed taxonomy or count boundary |
| `max_focuses_per_call` | `1` | Maximum evidence focuses per AI call; `1` disables batching to reduce oversized outputs, parse failures, and cross-focus conclusion bleed |
| `select_relevant_files` | `true` | Enable AI candidate narrowing for large candidate sets before agenda planning; when disabled, conservative local preparation is used |
| `select_relevant_files_min_candidates` | `200` | Minimum candidate file count before AI candidate narrowing is called; smaller changes keep local candidates |
| `structural.enabled` | `true` | Enable structural context; even when enabled, it only runs when focus, diff, sample, or entry files are available |
| `structural.provider` | `auto` | Structural context and symbol-verification source: `auto` prefers CodeGraph and falls back to tree-sitter when unavailable; `codegraph` requires CodeGraph; `treesitter` explicitly selects the embedded parser |
| `structural.max_symbols` | `30` | Maximum symbols emitted into structural context |
| `structural.max_file_size` | `512` | Per-source-file size limit in KB; larger files are skipped |

#### `structural`

The structural provider gives the Agent symbols, imports, entry points, and module clues and also verifies source symbols returned by AI. The default `provider: auto` prefers CodeGraph; it automatically initializes missing project indexes and attempts repair when sync or status checks fail, then falls back to embedded tree-sitter when the CodeGraph command or index is unavailable. `provider: codegraph` requires CodeGraph and returns errors directly; `provider: treesitter` uses only the embedded parser.

Starting in 0.7.1, structural pre-scan, `learn current`, and `preview` share the same file-filtering policy: source files, build config, and dependency config are included by default, while documents, generated outputs, paths matched by global `exclude`, and generated Skills output directories are skipped.

Starting in 0.9.0, project-structure summaries, sample-file collection, and structural pre-scan all use the same configured file-filtering policy. Except for built-in safety boundaries such as `.git`, `.skills-seed`, and configured generated-skills output directories, analyzer no longer keeps extra directory-name keywords. Put dependency, build-output, or project-specific directories in `exclude` when they should be skipped.

The current version applies candidate preparation after local file filtering: smaller changes keep their candidates and no longer drop source files by path vocabulary; path signals are used only to choose structural-context seeds. When candidate count reaches `select_relevant_files_min_candidates` and `select_relevant_files` is enabled, `learning-candidate-select` asks AI to narrow the large candidate set from the candidate list, required paths, and structural context. AI failures fall back to all candidates. Large inputs such as candidate paths, diffs, focused files, and structural context are written into runtime input files and referenced by prompts.

Starting in 0.9.11, file filtering also applies Git ignore rules by default. Starting in 0.9.12, the Git ignore switch lives at `exclude.gitignore`. Set it to `false` when files ignored by `.gitignore` should still be analyzed. Starting in 0.9.13, snapshots still preserve the full current state, but diffs sent to AI are filtered by `exclude.paths` and `exclude.gitignore`, preventing ignored files from entering analysis as deleted diffs.

#### Recommendations

1. Most projects should keep the defaults; structural context still does not run without bounded inputs.
2. For large repositories with many candidates, keep `select_relevant_files: true` and tune `select_relevant_files_min_candidates` to control planning cost.
3. Set `structural.enabled` to `false` when structural context is not needed.
4. Lower `structural.max_file_size` for large repositories when explicitly using tree-sitter to avoid generated files, bundles, or unusually large files.
5. Structural context only consumes bounded seed inputs and does not scan the whole repository when no seed exists.

### Prompt Runtime Debugging

Project context is read from `.skills-seed/context/`, and rendering filters default metadata, empty scaffolding, and unfilled placeholder text. Only user-authored context is kept.

Rendered prompts are saved by default under `.skills-seed/runtime/rendered-prompts/` with a neighboring `.manifest.json`. The manifest records whether built-in, context, and output-contract fragments were merged, plus raw and final lengths, so you can inspect the exact context sent to the Agent. Large inputs such as candidate files, focused files, and structural context are preferably stored in prompt input directories under `.skills-seed/runtime/`, and rendered prompts reference them by path. The final output contract is appended from a separate append template and forces JSON prompts to return exactly one parseable JSON object while keeping semantic output and deterministic ordering stable for identical inputs.

Starting in 0.10.5, `learn current` no longer writes the existing pattern store into every evidence prompt. To inspect stored patterns, read the local pattern store or use `patterns show` / `patterns stats`. Claude and Codex calls now enforce the same DTO-generated schema through native structured-output flags. The parser uses `jsonrepair-go` for JSON syntax repair, and repaired output must still pass strict DTO decoding; unknown fields and invalid shapes are not accepted.

Starting in 0.11.0, `learning.current.mode` can be set to `fast`, `normal`, or `deep` to choose between learning speed and pattern coverage quality; the mode is included in resume-state fingerprints. Generated skills render related-reference routing, importance layers, grouped entry indexes, and path-validated source evidence. Validation commands live only in `references/validation.md`; Go projects additionally derive `references/testing.md` from real `go.mod` and `_test.go` files, assigning each test to its nearest ancestor module.

Starting in 0.11.1, `learning.current.scope` can be set to `domain`, `flow`, or `module` to guide evidence-focus splitting by business domain, workflow, or module/plugin granularity, and it participates in resume-state fingerprints together with `mode`. Model-output parsing also repairs evidence line range expressions, normalizing invalid JSON such as `"line": 29-43` to a single line number.

Starting in 0.11.2, `learning.current.max_focuses_per_call` controls how many lightweight learning focuses one AI call may process, with the default `1` disabling batching. Raising it groups multiple focuses into one call and requires the response to return top-level `focuses`. Generated skills also keep low-frequency or local evidence out of the strong-constraint layer, so incidental examples are not rendered as mandatory project standards.

Candidate preparation decides which files enter agenda planning. AI candidate narrowing, evidence-focus planning, and current-code learning prompts use explicit stable-decision rules; when evidence is equivalent, they prefer structural evidence, routeability, and source vocabulary, then use lexicographic path, ID, or symbol order as the final tie-breaker.

The interactive init prompt writes total Agent parallelism into `agent.parallelism`. Workspace root configs use it for child-project concurrency; ordinary project configs use it for current-code evidence-focus batch concurrency. `learn current` uses independent runtime calls: after candidate preparation, planning creates evidence packs, each evidence-focus batch is analyzed in its own call, AI performs lightweight merge optimization, and the local normalization service hydrates fields, recovers coverage, validates, and stores candidate patterns.

Starting in 0.8.0, Agent outputs are saved separately under `.skills-seed/runtime/agent-outputs/` by default, including final content, raw CLI output, stderr, and a manifest. Runtime logs keep only lengths and archive paths, and no longer include model reply previews or raw stdout/stderr. Starting in 0.10.3, valid JSON final content is formatted as a readable fenced `json` block inside the `.md` archive.

Starting in 0.9.6, debug records under `.skills-seed/runtime` use the `YYYYMMDD-HHMMSS[-NNN]-<kind>-<name>` filename prefix; when multiple runtime IDs are generated in the same second, an incrementing sequence is appended to avoid overwrites. `rendered-prompts/` and their matching `agent-outputs/` share the same date-time ID and semantic name; Agent output files only add the Agent name, making each prompt/output pair easy to correlate. Starting in 0.10.3, valid JSON output is formatted as a readable fenced `json` block inside the `.md` archive.

Starting in 0.9.0, candidates are normalized before entering the pattern store. Current `learn current` receives candidate patterns from evidence-pack analysis; AI merge optimization only proposes source ownership and canonical patterns, while the local normalization service owns field hydration, source ownership validation, recall protection, fallback recovery, and one-shot storage. `generate skills` only reads stored data and performs neither pattern merging nor Agent calls.

The current version no longer maintains skills dirty state. `sync` generates skills only when the learning run changes learned output. Explicit `skills-seed generate skills` deletes the old skills-seed generated output directory and fully rebuilds it; after manually adding a user pattern, run this command to refresh generated artifacts.

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
| `parallelism` | `0` | Agent parallelism; workspace root configs use it for child projects, ordinary project configs use it for evidence-focus batches, `0` means automatic |
| `model` | empty | Model name passed to the Agent CLI for skills-seed calls; empty passes no model flag and inherits the local Agent CLI default |
| `retry.max_retries` | `3` | Maximum retry attempts for retryable errors; `0` uses the default `3` |
| `retry.initial_interval` | `15` | Initial retry wait in seconds; `0` uses the default `15` |
| `retry.max_interval` | `120` | Maximum exponential-backoff wait in seconds; `0` uses the default `120` |

#### `parallelism` Notes

1. In `project` mode, `agent.parallelism > 1` lets `learn current` analyze independent evidence-focus batches concurrently. Each batch uses its own runtime call, then results are merged and checkpointed in agenda order.
2. In `workspace` mode, automatic parallelism is the child project count, capped at `6`.
3. A positive value is used as the explicit concurrency limit.
4. Workspace child project tasks run through a goroutine worker pool. Evidence-focus concurrency inside each child project is controlled by that child project's `agent.parallelism`.

#### `retry` Notes

1. Retry currently applies to retryable Agent CLI errors such as 429 / 529 / overloaded.
2. Wait time starts at `initial_interval`, doubles after each retry, and is capped by `max_interval`.
3. Long-running steps such as `learn current` update the active progress line with the agent error, failed call duration, and backoff wait; the terminal also prints a stable retry notice with the wait duration and extracted API reason.
4. When the next call starts, the progress line switches to `attempt N`; final CLI call failures are shown only for exhausted retries or non-retryable errors.

#### Switch Agent

```yaml
agent:
  engine: "claude"
  model: ""
  commands:
    claude: "claude"
    codex: "codex"

skills:
  target: "codex"
  locale: "en-US"
  paths:
    claude: ".claude/skills/<project-name>-dev"
    codex: ".agents/skills/<project-name>-dev"
```

You can also set the agent during initialization:

```bash
skills-seed init --mode project --agent codex
skills-seed init --mode project --agent codex --agent-model gpt-5-mini
skills-seed init --workspace --agent codex
```

### Workflow Resources

User workflows are not stored in `config.yaml` and are not part of `profile.mode`. The command sends explicitly provided goals, constraints, background, or paths to the current Agent, infers a standard workflow from them, saves the inferred body to `.skills-seed/workflows/<id>/WORKFLOW.md`, and stores original notes plus metadata in `metadata.yaml` in the same directory:

```bash
skills-seed workflow --context "Check environment variables and build artifacts before release, then run smoke tests after deployment"
```

When `--name` is omitted, the Agent generates an English workflow title from `--context` and uses its slug as `<id>`; repeated titles receive a numbered suffix. `--context` can be a goal, constraint, background note, path, or rough description; the Agent infers a standard workflow from that explicit input. Existing same-name workflows are merged and deduplicated by default; use `--overwrite` to replace one completely.

When skills are generated, workflows are written to output `workflows/`, and matching script directories are copied to `scripts/workflows/<id>/`.

### `.skills-seed` Layout

`.skills-seed/store/` is persistent data and should not be deleted. `.skills-seed/cache/` is rebuildable cache. `.skills-seed/runtime/` contains logs, rendered prompts, Agent outputs, and temporary inputs; it can be deleted when you do not need troubleshooting artifacts.

| Path | Purpose |
|---|---|
| `.skills-seed/store/project.db` | Indexed data such as patterns, quality metrics, and file fingerprints |
| `.skills-seed/store/documents/` | Readable JSON documents such as profiles, specs, state, and changelog |
| `.skills-seed/cache/snapshots/` | Rebuildable file snapshot cache |
| `.skills-seed/cache/commands/<command>/state.json` | Resumable state for unfinished commands, including evidence-focus results not yet stored; cleared after persistence and safe to delete for a fresh detection and agenda |
| `.skills-seed/runtime/logs/` | Runtime logs |
| `.skills-seed/runtime/rendered-prompts/` | Rendered prompts and manifests |
| `.skills-seed/runtime/agent-outputs/` | Archived Agent outputs |

### `.skills-seed/context/`

`.skills-seed/context/` is not a `config.yaml` field, but it is created by `skills-seed init` as editable project context. Use it for persistent project background, team constraints, terminology, and workspace constraints.

Common paths:

| Path | Purpose |
|---|---|
| `.skills-seed/context/background.md` | Business background, external systems, and production facts not visible in code |
| `.skills-seed/context/constraints.md` | Long-lived team constraints, compatibility requirements, security boundaries, and forbidden changes |
| `.skills-seed/context/terminology.md` | Domain terms, aliases, state names, and mappings from business language to code terms |
| `.skills-seed/context/workspace.md` | Workspace-level context, generated only in workspace mode |

These files are merged with built-in prompts; they do not replace built-in prompts. Skills Seed appends a built-in final output contract after the merged fragments to protect the JSON / Markdown format expected by parsers.

`--context` and `--context-path` are one-time learning flags. They affect only the current `learn current` run, are not written to `.skills-seed/context/`, and are not passed to `generate skills`. Put long-lived rules in `context/constraints.md`; use `learn current --context` or `learn current --context-path` for temporary guidance.

### `skills`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `target` | `agent.engine` | Generated Skills target type; can differ from `agent.engine` |
| `locale` | `en-US` | Language used for AI learning output, persisted content, and generated Skills |
| `paths.claude` | `.claude/skills/<project-name>-dev` | Claude Code skills output directory; workspace roots default to `<workspace-name>-workspace-dev` |
| `paths.codex` | `.agents/skills/<project-name>-dev` | Codex skills output directory; workspace roots default to `<workspace-name>-workspace-dev` |

#### Notes

1. `generate skills` uses `skills.paths` for the current `skills.target` by default.
2. Use `skills-seed generate skills --output <path>` to override the output directory for one run.
3. `skills.locale` supports `zh-CN` and `en-US` and defaults to English; it controls runtime AI natural-language output, persisted learned content, and the language of `generate skills` artifacts.
4. For a custom engine or target, add `agent.commands.<engine>` and `skills.paths.<target>` respectively.

Runtime AI prompt templates are maintained as English-only source templates. Their final output contract follows `skills.locale`; `profile.locale` only affects tool output, config templates, and seed-context templates.

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

When `gitignore` is disabled, file filtering still applies built-in safety boundaries, generated Skills output directories, and `exclude.paths`, but source files ignored by Git are no longer skipped just because of Git ignore rules.

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
