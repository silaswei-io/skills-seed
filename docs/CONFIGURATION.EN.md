# Skills Seed Configuration Reference

## Local scanning and focused semantic enrichment

`learn current` and `sync` update a local structure index. `codegraph.runtime: auto` prefers an installed external CodeGraph platform package and falls back to bundled 1.6.2; `external` and `embedded` force one source. Structural scanning uses no AI, while important semantic gaps still trigger analysis and independent review. Source builds must run `make bundle` first. `SKILLS_SEED_CODEGRAPH_HOME` selects an official platform package and takes precedence over config. The bundled component extracts into the user cache on first use without a runtime download.

```bash
skills-seed sync
skills-seed codegraph query "HTTP"
# Optionally request deeper analysis of a concrete scope:
skills-seed learn current --semantic --focus internal/order --context "Check order cancellation state boundaries"
skills-seed generate skills
```

Generated `references/code-navigation.md` links directories to source symbols and instructs development agents to search existing capabilities, inspect signatures and usage examples, then reuse, extend or justify a new component. Directories are structural navigation, not inferred business domains. `codegraph callers/callees/impact` take returned symbol IDs; `codegraph trace <from-id> <to-id>` finds static call paths. Dynamic and cross-service relationships may be incomplete; an empty result does not prove that no reusable capability exists.

The index lives in the standard `.codegraph/` directory and is shared with other CodeGraph tools. Skills-seed adds its own `navigation.json` and lock file; a narrower scan does not recreate the shared database. Before opening the shared database, the selected runtime checks its schema version and refuses an incompatible index instead of migrating it implicitly. Source selection follows repository exclusions and excludes generated Skills and caches. Files not indexed by the SDK are listed explicitly. Extraction failures return errors rather than claiming success.

Structural scans alone never advance semantic fingerprints. Only successfully analyzed, reviewed and stored supplements commit their scoped fingerprints; unfinished checkpoints remain available. Generation omits old Patterns whose source evidence fingerprints changed, retaining the original knowledge in storage. Project navigation is generated deterministically from the local index and saved knowledge; when source evidence changes, generation only stops projecting invalidated business patterns. Rule/Workflow maintenance is unchanged. Consuming agents must read applicable authority files directly.

Workspace routing comes from explicit configuration without AI inference of cross-project semantics. Children use the same automatic supplementation policy. Run `--semantic --focus` inside a child for deeper analysis. `--context` is passed to automatic supplements; `--focus` only limits the local scan scope. `--force` rechecks the index and requests output refresh.

Automatic supplementation first revisits changed evidence of accepted knowledge, then selects semantic entries from the local CodeGraph call graph. Downstream capabilities on the same call chain remain in navigation instead of triggering repeated per-file AI questions; independent entries that the graph cannot explain still need analysis. Only when several complete evidence batches remain does a compact structural pre-screen run concurrently; failures and uncertain cases retain their candidates. Deleted source evidence suppresses old knowledge during generation. Each run dynamically batches candidates by complete evidence capacity while retaining related evidence for existing knowledge. The evidence bundle is capped at 64 KiB and the final prompt at 96 KiB. Oversized evidence is deferred as an abnormal batch, never truncated or marked complete. Normal `sync` continues until all normal candidates are processed; individual Agent calls remain protected by `agent.timeout`, retry limits, and input-size checks. Unavailable AI, errors, timeouts and unresolved evidence keep their batches pending while independent batches continue, with diagnostics in `.skills-seed/runtime/local-supplement.json`. `mode` and `structural` only affect explicit semantic supplementation; the default flow has no AI agenda configuration.

[简体中文](CONFIGURATION.md) | [English](CONFIGURATION.EN.md)

The config file lives at `.skills-seed/config.yaml`. `skills-seed init` creates it from the project context. Most paths are relative to the project root or `.skills-seed`; each field below states the relevant base.

## 0.8.x Config Structure

0.8.x keeps the 0.7.x config shape and continues to avoid compatibility with old fields:

- Top-level `project` was renamed to `profile`. It describes the project or workspace that owns the config file; it is not the `project` run mode.
- `workspace` now keeps only `projects`; user-written `shared`, `contracts`, and `infra` fields were removed.
- Workspace shared libraries, contracts, and infrastructure routing come from explicit project configuration and local CodeGraph facts; no extra AI profile call guesses them, and generation only consumes saved artifacts.
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
  max_turns: 80
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
    pattern_admission:
      min_confidence: 0.75
      min_single_evidence_confidence: 0.85
    agenda:
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
  enabled: false
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
| `root_path` | current project absolute path or empty | Init normally writes the absolute path; when empty, the runtime derives the project root from the current `.skills-seed` directory |
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
| `mode` | `normal` | All learning strategies use high-precision admission: `fast` narrows evidence exploration; `normal` is the default; `deep` broadens evidence exploration without lowering the pattern admission standard |
| `structural.enabled` | `true` | Enable structural context; it runs only when focus, source sample, or entry files are available |
| `structural.provider` | `auto` | Structural context and symbol-verification source: `auto` prefers CodeGraph and falls back to tree-sitter when unavailable; `codegraph` requires CodeGraph; `treesitter` explicitly selects the embedded parser |
| `structural.max_symbols` | `30` | Maximum symbols emitted into structural context |
| `structural.max_file_size` | `512` | Per-source-file size limit in KB; larger files are skipped |

#### `structural`

The structural provider gives the Agent symbols, imports, entry points, and module clues and also verifies source symbols returned by AI. The default `provider: auto` prefers CodeGraph; it automatically initializes missing project indexes and attempts repair when sync or status checks fail, then falls back to embedded tree-sitter when the CodeGraph command or index is unavailable. `provider: codegraph` requires CodeGraph and returns errors directly; `provider: treesitter` uses only the embedded parser.

Starting in 0.7.1, structural pre-scan, `learn current`, and `preview` share the same file-filtering policy: source files, build config, and dependency config are included by default, while documents, generated outputs, paths matched by global `exclude`, and generated Skills output directories are skipped.

Starting in 0.9.0, project-structure summaries, sample-file collection, and structural pre-scan all use the same configured file-filtering policy. Except for built-in safety boundaries such as `.git`, `.skills-seed`, and configured generated-skills output directories, analyzer no longer keeps extra directory-name keywords. Put dependency, build-output, or project-specific directories in `exclude` when they should be skipped.

The current version keeps every in-scope candidate after local file filtering; no path vocabulary or model guess narrows source candidates before planning. Large inputs such as candidate paths, focused files, and structural context are written into runtime input files and referenced by prompts.

File filtering applies Git ignore rules by default; set `exclude.gitignore` to `false` to analyze ignored files. Persistent file fingerprints determine incremental status without storing source snapshots. AI compares current source evidence with stored knowledge for semantic changes. Opening an existing project removes the obsolete `.skills-seed/cache/snapshots/` directory.

#### Recommendations

1. Most projects should keep the defaults; structural context still does not run without bounded inputs.
2. Set `structural.enabled` to `false` when structural context is not needed.
3. Lower `structural.max_file_size` for large repositories when explicitly using tree-sitter to avoid generated files, bundles, or unusually large files.
4. Structural context only consumes bounded seed inputs and does not scan the whole repository when no seed exists.

### Prompt Runtime Debugging

The project background layer is read from `.skills-seed/context/`, and rendering filters default metadata, empty scaffolding, and unfilled placeholder text. It cannot create or override Rules, command permission, Workflow procedures, or source evidence.

Rendered prompts are saved by default under `.skills-seed/runtime/rendered-prompts/` with a neighboring `.manifest.json`. The manifest records whether built-in, context, and output-contract fragments were merged, plus raw and final lengths, so you can inspect the exact context sent to the Agent. Large inputs such as candidate files, focused files, and structural context are preferably stored in prompt input directories under `.skills-seed/runtime/`, and rendered prompts reference them by path. The final output contract is appended from a separate append template and forces JSON prompts to return exactly one parseable JSON object while keeping semantic output and deterministic ordering stable for identical inputs.

Starting in 0.10.5, `learn current` no longer writes the existing pattern store into every evidence prompt. To inspect stored patterns, read the local pattern store or use `patterns show` / `patterns stats`. Claude and Codex calls enforce the same DTO-generated schema through native structured-output flags; full and delta focus analysis passes the current batch's focus IDs into the actual runtime Schema, constraining both the focus enum and result count. The parser uses `jsonrepair-go` for JSON syntax repair, and repaired output must still pass strict DTO decoding; unknown fields and invalid shapes are not accepted.

Starting in 0.11.0, `learning.current.mode` can be set to `fast`, `normal`, or `deep` to choose between learning speed and pattern coverage quality; the mode is included in resume-state fingerprints. Generated skills render related-reference routing, importance layers, grouped entry indexes, and path-validated source evidence. Testing, deployment, and acceptance procedures are not learned from source; user-maintained Workflows provide their steps, commands, and ordering and are generated as part of the final Skills.

Evidence focuses have no configurable classification. Planning derives the minimum evidence set needed to validate a future development decision from source responsibilities, capabilities, resource actions, state transitions, contracts, integrations, and cross-cutting mechanisms. A focus exists only for the current evidence pass; it is not a Workflow, work procedure, or fixed generated-Skill taxonomy. Model-output parsing also repairs evidence line range expressions, normalizing invalid JSON such as `"line": 29-43` to a single line number.

The default flow groups related evidence into one bounded analysis call and independently reviews only business candidates. Once an analysis result is checkpointed, its independent review can overlap analysis or review of another batch. Checkpoints for one run are committed serially. If a delta candidate fails local evidence admission and no other decision is admitted for that focus, the focus is reduced to exactly one `no_change` receipt, distinguishing “analyzed with no admissible Pattern” from “omitted by the Agent.” Generated skills also keep low-frequency or local evidence out of the strong-constraint layer, so incidental examples are not rendered as mandatory project standards.

In independent review, a complete `business_method` sets or replaces a capability entry, while omission removes it; no redundant verdict field duplicates the object's state. The review Agent must open the exact definition directly called or referenced by candidate evidence and record its full declaration, project-relative location, prerequisites, and result/error semantics. Local code validates only the structured contract and path safety; it does not substitute caller/definition file equality for semantic review.

Candidate preparation applies scope, exclusion rules, and path normalization. Local CodeGraph facts select important business gaps; AI receives only bounded evidence batches. Source analysis and independent review own semantic judgment; local code validates evidence, scope, and structured contracts.

The interactive init prompt writes total Agent parallelism into `agent.parallelism`. Workspace root configs use it for child-project concurrency; ordinary projects share the same Agent concurrency budget with nested calls. Related evidence is analyzed together until the bounded prompt is full. `learning.current.pattern_admission` controls the confidence required for a candidate to enter the merge and storage flow.

Starting in 0.8.0, Agent outputs are saved separately under `.skills-seed/runtime/agent-outputs/` by default, including final content, raw CLI output, stderr, and a manifest. Runtime logs keep only lengths and archive paths, and no longer include model reply previews or raw stdout/stderr. Starting in 0.10.3, valid JSON final content is formatted as a readable fenced `json` block inside the `.md` archive.

Retryable Agent calls are recorded as `ERROR` in both the terminal and runtime log with a localized cause, call duration, and the wait before the next attempt. Raw stdout and stderr remain available through archive paths so large model outputs are not written into logs.

Starting in 0.9.6, debug records under `.skills-seed/runtime` use the `YYYYMMDD-HHMMSS[-NNN]-<kind>-<name>` filename prefix; when multiple runtime IDs are generated in the same second, an incrementing sequence is appended to avoid overwrites. `rendered-prompts/` and their matching `agent-outputs/` share the same date-time ID and semantic name; Agent output files only add the Agent name, making each prompt/output pair easy to correlate. Starting in 0.10.3, valid JSON output is formatted as a readable fenced `json` block inside the `.md` archive.

Candidates are normalized before entering the pattern store. Current `learn current` sends source-analysis candidates through an independent knowledge review, applies the configured evidence and confidence thresholds, and then asks AI only for semantic merge proposals against related stored patterns. The local normalization service validates source IDs, propagates controlled knowledge flags from their sources, resolves duplicate ownership deterministically, restores omitted eligible candidates, and writes once. In learning summaries, “candidate patterns found” is the number of candidates that remain after review, while “normalized patterns written” is the final upsert count including related existing Patterns, so the latter can be larger. Every current source candidate establishes input identity from its own Pattern ID; existing `MergedFrom` values describe historical output lineage and do not participate in current ownership. A current candidate with the same ID is the effective version of that Pattern and replaces the stored record before validation and hydration; only different IDs are evaluated as independent merge sources. When revalidating an already hydrated result, unambiguous historical lineage only resolves to its effective Pattern and hydrated sources are deduplicated by canonical ID; lineage with multiple owners remains rejected. `generate skills` only reads stored data and performs no pattern merging or Agent calls.

The current version no longer maintains skills dirty state. `sync` generates skills only when the learning run changes learned output. Workspace `sync` reports each child's learned-change, generation-skipped, or missing-Skills-restored outcome, then aggregates file changes, candidates, normalized writes, and retired Patterns across all children. Unchanged workspace relationship input does not mean child projects learned nothing. Explicit `skills-seed generate skills` deletes the old skills-seed generated output directory and fully rebuilds it; after manually adding a user pattern, run this command to refresh generated artifacts.

### Generated Notice

The skills-seed generated footer in Skills templates is now controlled by an internal default and is omitted by default, reducing generated-content feedback into later learning. To inspect artifact provenance, use the `generated-by` metadata header or runtime logs.

### `agent`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `engine` | `claude` | Agent engine used for analysis, learning, and generation summaries; matches keys in `commands` |
| `commands` | `claude: claude`, `codex: codex` | Engine-to-CLI command mapping |
| `timeout` | `1800` | Total budget in seconds for one Agent stage, including queueing, execution, backoff, and result repair; retries do not reset the deadline |
| `max_turns` | `80` | Maximum exploration turns for one Agent call; stops at the limit to bound context growth; currently used by Claude CLI |
| `allow_user_plugins` | `false` | Whether agents may load user plugins; disabled by default for stable batch runs |
| `parallelism` | `0` | Agent concurrency limit; workspace roots cap both child tasks and shared Agent calls. Child settings cannot enlarge the shared budget. Analysis and independent review share slots; in project mode `0` uses local CPU capacity, capped at six concurrent calls |
| `model` | empty | Model name passed to the Agent CLI for skills-seed calls; empty passes no model flag and inherits the local Agent CLI default |
| `retry.max_retries` | `3` | Maximum retry attempts for retryable errors; `0` uses the default `3` |
| `retry.initial_interval` | `15` | Initial retry wait in seconds; `0` uses the default `15` |
| `retry.max_interval` | `120` | Maximum exponential-backoff wait in seconds; `0` uses the default `120` |

`max_turns` is used by Claude CLI to bound Agent exploration turns for one call. Learning calls use strict MCP configuration by default and allow only the read-only tools declared by the invocation; unavailable or denied tools are not retried. Codex does not receive this Claude-specific argument.

#### `parallelism` Notes

1. In `project` mode, `agent.parallelism > 1` lets `learn current` analyze independent evidence-focus batches concurrently and overlap review of saved results with other batches. Results are checkpointed individually by the main flow; final knowledge follows agenda order.
2. In `workspace` mode, automatic parallelism is the child project count, capped at `6`.
3. A positive value is used as the explicit concurrency limit.
4. Workspace children run through a goroutine worker pool. Each child controls focus scheduling, but all actual Agent calls share the root budget. Independent analysis and review may overlap; checkpoints for one run are still committed serially.

#### `retry` Notes

1. Retry currently applies to retryable Agent CLI errors such as 429 / 529 / overloaded.
2. Wait time starts at `initial_interval`, doubles after each retry, and is capped by `max_interval`.
   Structure validation failures skip service backoff and get at most one repair with the previous output and error; repair still counts against `max_retries`. All attempts share the stage `timeout`, and no new attempt starts after expiry.
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

User workflows are not stored in `config.yaml` and are not part of `profile.mode`. Users provide complete Markdown content directly; it is saved to `.skills-seed/workflows/<id>/WORKFLOW.md`, where `<id>` is derived from `--name`:

```bash
skills-seed workflow --name release --content "# Release\n\n- Check environment variables and build artifacts"
```

Both `--name` and `--content` are required. The body does not require fixed sections, so users can maintain task-specific steps, checks, and constraints. Same-name workflows never merge automatically; use `--overwrite` to replace one completely.

When skills are generated, workflows are written to output `workflows/`, and matching script directories are copied to `scripts/workflows/<id>/`.

### `.skills-seed` Layout

`.skills-seed/store/` is persistent data and should not be deleted. `.skills-seed/cache/` holds resumable command state. `.skills-seed/runtime/` contains optional file logs, rendered prompts, Agent outputs, and temporary inputs; it can be deleted when you do not need troubleshooting artifacts. Initialization does not pre-create `runtime/`. Prompt input directories are created only when input files are written, so inline inputs leave no empty directories. Disabling file logging affects `runtime/logs/`; other runtime artifacts are still written when needed.

| Path | Purpose |
|---|---|
| `.skills-seed/store/project.db` | Indexed data such as patterns, quality metrics, and file fingerprints |
| `.skills-seed/store/documents/` | Readable JSON documents such as profiles, specs, state, and changelog |
| `.skills-seed/cache/commands/<command>/state.json` | Resumable state for unfinished commands, including evidence-focus results not yet stored; cleared after persistence and safe to delete for a fresh detection and agenda |
| `.skills-seed/runtime/logs/` | File logs written only with `logging.enabled: true` |
| `.skills-seed/runtime/rendered-prompts/` | Rendered prompts and manifests |
| `.skills-seed/runtime/agent-outputs/` | Archived Agent outputs |

### `.skills-seed/context/`

`.skills-seed/context/` is not a `config.yaml` field, but it is created by `skills-seed init` as an editable, non-authoritative project background layer. Use it only for project background, terminology, and workspace background. Mandatory long-lived rules are maintained separately under `.skills-seed/rules/`; task procedures are maintained under `.skills-seed/workflows/`.

Common paths:

| Path | Purpose |
|---|---|
| `.skills-seed/context/background.md` | Business background, external systems, and production facts not visible in code |
| `.skills-seed/context/terminology.md` | Domain terms, aliases, state names, and mappings from business language to code terms |
| `.skills-seed/context/workspace.md` | Workspace-level context, generated only in workspace mode |

These files are merged with built-in prompts; they do not replace built-in prompts. Skills Seed appends a built-in final output contract after the merged fragments to protect the JSON / Markdown format expected by parsers.

`--context` and `--context-path` are one-time background inputs. They affect only the current `learn current` run, are not written to `.skills-seed/context/`, and are not passed to `generate skills`. Maintain long-lived rules with `skills-seed rule`, task procedures with `skills-seed workflow`, and temporary background with `learn current --context` or `learn current --context-path`.

### `skills`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `target` | `agent.engine` | Generated Skills target type; can differ from `agent.engine` |
| `name` | empty | Explicit Skill name; empty uses the project default, or the workspace default for a workspace root |
| `locale` | `en-US` | Language used for AI learning output, persisted content, and generated Skills |
| `paths.claude` | `.claude/skills/<project-name>-dev` | Claude Code skills output directory; workspace roots default to `<workspace-name>-workspace-dev` |
| `paths.codex` | `.agents/skills/<project-name>-dev` | Codex skills output directory; workspace roots default to `<workspace-name>-workspace-dev` |

#### Notes

1. `generate skills` uses `skills.paths` for the current `skills.target` by default.
2. Use `skills-seed generate skills --output <path>` to override the output directory for one run.
3. `skills.locale` supports `zh-CN` and `en-US` and defaults to English; it controls runtime AI natural-language output, persisted learned content, and the language of `generate skills` artifacts.
4. For a custom engine or target, add `agent.commands.<engine>` and `skills.paths.<target>` respectively.
5. Relative `skills.paths` and `--output` values resolve from the current project root. Workspace children use their own roots. The output must remain inside that project, including after symlink resolution.

Use `skills-seed init --skills-name team-guide` to set both `skills.name` and target output paths during initialization. Explicit names are used unchanged, without a `-dev` suffix. They must contain 1–64 lowercase letters, digits, or hyphens, with no leading, trailing, or consecutive hyphens. This does not change `profile.name` or project ownership, and a root name is not inherited by children. When renaming manually after initialization, also update the directory names in `skills.paths`; configuration changes do not move existing artifacts.

Runtime AI prompt templates are maintained as English-only source templates. Their final output contract follows `skills.locale`; `profile.locale` only affects tool output, config templates, and seed-context templates.

### `runtime`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `cleanup_before_reanalysis` | `true` | Clear `.skills-seed/runtime/` before reanalysis; `resume` flows do not trigger it |

### `logging`

#### Fields

| Field | Default | Description |
|---|---:|---|
| `enabled` | `false` | Write runtime log files; terminal output, run records, and checkpoints still work when disabled |
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
