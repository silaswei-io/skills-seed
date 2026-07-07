# Skills Seed Command Reference

[简体中文](COMMANDS.md) | [English](COMMANDS.EN.md)

This is the complete command reference. Every command supports `--help`. Commands that read `.skills-seed/config.yaml` require `skills-seed init` first.

## Command Overview

| Stage | Command | Purpose | Common Entry |
|---|---|---|---|
| Basics | [`skills-seed`](#skills-seed) | View global help, version, and template hashes | `skills-seed --help` |
| Initialization | [`skills-seed init`](#skills-seed-init) | Initialize a single project or workspace root | `skills-seed init --mode project` |
| Workspace | [`skills-seed workspace`](#skills-seed-workspace) | Add or manage workspace child projects | `skills-seed workspace add .` |
| Reset | [`skills-seed reset`](#skills-seed-reset) | Back up and recreate `.skills-seed` | `skills-seed reset --mode workspace` |
| Learning | [`skills-seed learn`](#skills-seed-learn) | Learn patterns from current code or Git history | `skills-seed learn current` |
| Generation | [`skills-seed generate`](#skills-seed-generate) | Generate skills from profiles and patterns | `skills-seed generate skills` |
| Preview | [`skills-seed preview`](#skills-seed-preview) | Preview files selected for full or incremental analysis | `skills-seed preview files` |
| Pattern Management | [`skills-seed patterns`](#skills-seed-patterns) | Add, delete, curate, and inspect patterns | `skills-seed patterns show` |
| Workflow | [`skills-seed workflow`](#skills-seed-workflow) | Add or update user task workflows | `skills-seed workflow --context "..."` |
| Review Metrics | [`skills-seed review`](#skills-seed-review) | Import review comments and measure pattern coverage | `skills-seed review stats` |
| Project Profile | [`skills-seed profile`](#skills-seed-profile) | Show or refresh the project profile | `skills-seed profile show` |
| One-Step Sync | [`skills-seed sync`](#skills-seed-sync) | Learn current code and generate skills | `skills-seed sync` |
| Change History | [`skills-seed log`](#skills-seed-log) | Show learned change history | `skills-seed log` |
| Checks | [`skills-seed check`](#skills-seed-check) | Check staged or tracked files | `skills-seed check` |
| Git Hook | [`skills-seed hook`](#skills-seed-hook) | Install, remove, or manually run the pre-commit hook | `skills-seed hook install` |
| Help | [`skills-seed help`](#skills-seed-help) | Show help for any command path | `skills-seed help learn current` |

## Common Workflows

| Scenario | Recommended Commands | Notes |
|---|---|---|
| Initialize one project | `skills-seed init --mode project` → `skills-seed sync` | Create config, learn current code, and generate skills |
| Initialize a workspace | `skills-seed init --workspace` → `skills-seed workspace add .` → `skills-seed sync` | The root coordinates child learning, then generates child and root skills |
| Daily incremental update | `skills-seed sync` | Learns current changes and generates skills only when learning changed output |
| Add one missed rule | `skills-seed patterns add --context "<description>"` → `skills-seed generate skills` | Adds a natural-language pattern, then regenerates |
| Update task workflow | `skills-seed workflow --context "<notes>"` → `skills-seed generate skills` | `--context` is inferred by the Agent from goals, constraints, background, or paths; omit `--name` to generate one, same-name workflows merge by default, and `--overwrite` replaces one completely |
| Pre-commit updates | `skills-seed hook install` | Install the pre-commit hook and choose sync, learn only, or skip before commit |
| Inspect learned changes | `skills-seed log` | Show recent learned and generated changes in a git-log-like format |
| Inspect learned output | `skills-seed patterns show` → `skills-seed profile show` | Verify learned patterns and the current project profile |

<!-- COMMAND_TREE_START -->
## Generated Command Index

> This section is generated from the Cobra command tree to keep commands, subcommands, and flag defaults aligned with the CLI implementation. Detailed usage notes remain in the command sections below.

| Command | Summary | Subcommands | Flags |
|---|---|---|---|
| `skills-seed` | Growing project skills for AI agents | `check`, `generate`, `hook`, `init`, `learn`, `log`, `patterns`, `preview`, `profile`, `reset`, `review`, `sync`, `workflow`, `workspace` | `--help, -h` = `false`<br>`--version, -v` = `false` |
| `skills-seed check` | Check staged files | - | `--all, -a` = `false`<br>`--help, -h` = `false`<br>`--interactive, -i` = `true` |
| `skills-seed generate` | Generate AI Agent outputs | `skills` | `--help, -h` = `false` |
| `skills-seed generate skills` | Generate AI Agent skills | - | `--help, -h` = `false`<br>`--no-references` = `false`<br>`--output, -o` = `` |
| `skills-seed hook` | Manage Git hooks | `install`, `run`, `uninstall` | `--help, -h` = `false` |
| `skills-seed hook install` | Install Git pre-commit hook | - | `--help, -h` = `false` |
| `skills-seed hook run` | Run the pre-commit hook manually | - | `--help, -h` = `false` |
| `skills-seed hook uninstall` | Uninstall Git pre-commit hook | - | `--help, -h` = `false` |
| `skills-seed init` | Initialize skills-seed project | - | `--agent` = ``<br>`--help, -h` = `false`<br>`--locale, -l` = ``<br>`--mode` = `project`<br>`--no-interactive` = `false`<br>`--skills-locale` = ``<br>`--skills` = ``<br>`--workspace` = `false` |
| `skills-seed learn` | Learn from Git history | `current`, `history` | `--help, -h` = `false` |
| `skills-seed learn current` | Learn from current codebase | - | `--context-path` = `[]`<br>`--context` = ``<br>`--focus, -f` = `[]`<br>`--force` = `false`<br>`--help, -h` = `false`<br>`--language, -l` = ``<br>`--profile` = `auto` |
| `skills-seed learn history` | Learn from Git history | - | `--batch-size, -b` = `10`<br>`--help, -h` = `false`<br>`--limit, -n` = `50`<br>`--since, -s` = `` |
| `skills-seed log` | Show learned change history | - | `--help, -h` = `false` |
| `skills-seed patterns` | Manage learned patterns | `add (--context <description> \| --context-path <path>)`, `compact`, `delete <pattern-id>`, `show [pattern-id]`, `stats`, `update <pattern-id> (--context <description> \| --context-path <path>)` | `--help, -h` = `false` |
| `skills-seed patterns add (--context <description> \| --context-path <path>)` | Add a user-defined pattern using natural language | - | `--category, -c` = ``<br>`--context-path` = `[]`<br>`--context` = ``<br>`--help, -h` = `false` |
| `skills-seed patterns compact` | Compact similar patterns | - | `--ai` = `false`<br>`--category, -c` = ``<br>`--dry-run` = `false`<br>`--help, -h` = `false` |
| `skills-seed patterns delete <pattern-id>` | Delete a pattern | - | `--help, -h` = `false` |
| `skills-seed patterns show [pattern-id]` | Show learned pattern overview or full details | - | `--format` = `table`<br>`--help, -h` = `false`<br>`--sort` = `updated` |
| `skills-seed patterns stats` | Show learned pattern quality and check hit statistics | - | `--help, -h` = `false` |
| `skills-seed patterns update <pattern-id> (--context <description> \| --context-path <path>)` | Update a pattern | - | `--category, -c` = ``<br>`--context-path` = `[]`<br>`--context` = ``<br>`--help, -h` = `false` |
| `skills-seed preview` | Preview analysis inputs | `files` | `--help, -h` = `false` |
| `skills-seed preview files` | Preview files selected for analysis | - | `--focus, -f` = `[]`<br>`--help, -h` = `false`<br>`--limit` = `200`<br>`--mode` = `full` |
| `skills-seed profile` | Show or refresh the project profile | `refresh`, `show` | `--help, -h` = `false` |
| `skills-seed profile refresh` | Re-analyze the project and save the project profile | - | `--help, -h` = `false`<br>`--language, -l` = `` |
| `skills-seed profile show` | Show the current project profile summary | - | `--help, -h` = `false` |
| `skills-seed reset` | Back up and reset skills-seed initialization state | - | `--help, -h` = `false`<br>`--locale, -l` = ``<br>`--mode` = `project`<br>`--skills-locale` = ``<br>`--workspace` = `false` |
| `skills-seed review` | Import review comments and show prevention statistics | `import`, `stats` | `--help, -h` = `false` |
| `skills-seed review import` | Import review comments from a JSON file | - | `--from-file` = ``<br>`--help, -h` = `false` |
| `skills-seed review stats` | Show review comment prevention statistics | - | `--help, -h` = `false`<br>`--line-window` = `3` |
| `skills-seed sync` | Sync skills | - | `--context-path` = `[]`<br>`--context` = ``<br>`--help, -h` = `false`<br>`--no-interactive` = `false`<br>`--restart` = `false`<br>`--resume` = `false` |
| `skills-seed workflow` | Add or update a user workflow | - | `--child` = ``<br>`--context` = ``<br>`--help, -h` = `false`<br>`--name` = ``<br>`--overwrite` = `false` |
| `skills-seed workspace` | Manage workspace sub-projects | `add .\|project-id-or-path...` | `--help, -h` = `false` |
| `skills-seed workspace add .\|project-id-or-path...` | Add sub-projects to workspace | - | `--help, -h` = `false` |
<!-- COMMAND_TREE_END -->

## Usage Conventions

### `skills-seed`

#### Command Overview

`skills-seed` is the root command. Use it to view global help, print version details, and enter each functional command.

#### Global Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show help for the current command |
| `--version`, `-v` | `false` | Print version and embedded template hashes |

#### Common Examples

```bash
skills-seed --help
skills-seed --version
skills-seed <command> --help
```

#### Version Output

```text
skills-seed version <version>
prompt-templates-sha256: <hash>
skills-templates-sha256: <hash>
```

#### Notes

1. Use `skills-seed <command> --help` to view detailed flags for any command.
2. `--version` prints the current binary version. Runtime documentation links point to the matching tag instead of `main`.

## Top-Level Commands

### `skills-seed init`

#### Command Overview

Initialize `.skills-seed/`, default config, database, project context, and skills templates in a Git repository. Supports both single-project and workspace modes.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed init` | Initialize the current repository | `skills-seed init --mode project --agent codex --skills codex --locale en-US` | Must run from a Git repository root; existing `.skills-seed` is not overwritten |

#### `init` Flags

| Flag | Default | Description |
|---|---:|---|
| `--mode` | `project` | Initialization mode: `project` for a single project, `workspace` for a multi-project root |
| `--agent` | empty | Execution Agent engine to write during initialization, for example `claude` or `codex`; empty uses the built-in default |
| `--skills` | empty | Skills output type to write during initialization, for example `claude` or `codex`; empty uses the built-in default |
| `--workspace` | `false` | Shortcut for `--mode workspace` |
| `--locale`, `-l` | empty | Tool output and config template language: `zh-CN` or `en-US`; empty uses the built-in default `zh-CN` |
| `--skills-locale` | empty | AI learning output, generated Skills, and persisted content language: `zh-CN` or `en-US`; empty uses the built-in default `en-US` |
| `--help`, `-h` | `false` | Show `init` help |

#### Common Examples

```bash
skills-seed init --mode project --locale en-US
skills-seed init --mode project --agent claude --skills codex --locale en-US
skills-seed init --mode workspace --locale en-US
skills-seed init --workspace
skills-seed init --workspace --agent codex --skills codex
```

#### Notes

1. `--agent` sets `agent.engine` and ensures the engine exists in `agent.commands`.
2. `--skills` sets `skills.target` and ensures `skills.paths` contains the target's default output directory.
3. `--workspace` initializes the root and the child repositories detected at that time.
4. Newly initialized child repositories inherit root `agent.engine`, `agent.commands`, `skills.target`, and `skills.paths`.
5. Already initialized children are skipped. If a child agent differs from the root, it is reported and preserved.
6. A successful init prints the relative `.skills-seed` location and the README URL for the current version tag.
7. Workspace child discovery only treats first-level independent Git repositories as children; marker files classify type and language only.

### `skills-seed workspace`

#### Command Overview

Manage sub-projects in workspace mode.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed workspace add .` | Detect and add all child repositories | `skills-seed workspace add .` | Only works from a workspace-mode root repository |
| `skills-seed workspace add <child...>` | Add only selected child repositories | `skills-seed workspace add backend frontend` | Arguments may be detected child ids or paths |

#### `workspace` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `workspace` help |

#### Notes

1. `workspace add` uses the same discovery rule as `init --workspace`: only first-level directories with their own `.git` are treated as child repositories.
2. Files such as `go.mod`, `package.json`, install scripts, Helm charts, and Terraform files only classify child `type` and `language`.
3. Workspace config no longer exposes `shared`, `contracts`, or `infra`; cross-project impact is analyzed and persisted into workspace profile/spec during `learn current`, and generation only consumes learned artifacts.
4. If a child has no `.skills-seed`, it is initialized in project mode.
5. If a child already has `.skills-seed/config.yaml`, it is skipped and preserved.
6. If a child has a `.skills-seed` directory but no `config.yaml`, the command fails instead of overwriting partial state.

### `skills-seed reset`

#### Command Overview

Back up and reset the current repository's `.skills-seed`. Existing data is moved to `.skills-seed.backup/<timestamp>`, then config and directories are recreated for the selected mode.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed reset` | Reset the current repository initialization state | `skills-seed reset --mode workspace` | Backs up the old `.skills-seed`; still review the current worktree first |

#### Flags

| Flag | Default | Description |
|---|---:|---|
| `--mode` | `project` | Mode after reset: `project` or `workspace` |
| `--workspace` | `false` | Shortcut for `--mode workspace` |
| `--locale`, `-l` | empty | Tool output and config template language after reset: `zh-CN` or `en-US`; empty uses the built-in default `zh-CN` |
| `--skills-locale` | empty | AI learning output, generated Skills, and persisted content language after reset: `zh-CN` or `en-US`; empty uses the built-in default `en-US` |
| `--help`, `-h` | `false` | Show `reset` help |

#### Common Examples

```bash
skills-seed reset --mode project
skills-seed reset --mode workspace
skills-seed reset --workspace
```

#### Notes

1. Use `reset` to reinitialize or choose another mode.
2. `profile.mode` is locked after learning or skill generation starts and should not be changed directly in config.

### `skills-seed learn`

#### Command Overview

Learn coding patterns, business methods, and best practices from the current codebase or Git history, then store them in the `.skills-seed` database.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed learn current` | Incrementally learn from the current codebase | `skills-seed learn current --focus internal/service --profile skip` | Compares file md5 values and learns only added, modified, or deleted files; add `--force` after prompt or template upgrades to relearn the current scan scope |
| `skills-seed learn history` | Learn from Git commit history | `skills-seed learn history --limit 50 --batch-size 5` | Already learned commits are skipped |

#### `learn` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `learn` help |

#### `learn current` Flags

| Flag | Default | Description |
|---|---:|---|
| `--language`, `-l` | config or auto-detect | Primary project language |
| `--focus`, `-f` | empty | Learn only a directory or file; may be repeated, and paths must stay under the project root |
| `--profile` | `auto` | Project profile refresh strategy: `auto`, `skip`, or `refresh` |
| `--context` | empty | One-time guidance for this learn run, passed to the AI agent and not written to `.skills-seed/context/` |
| `--context-path` | empty | Read one-time guidance for this learn run from files or directories; may be repeated and is not written to `.skills-seed/context/` |
| `--help`, `-h` | `false` | Show `learn current` help |

#### `learn history` Flags

| Flag | Default | Description |
|---|---:|---|
| `--limit`, `-n` | `learning.history.max_commits`, default `50` | Maximum number of commits to analyze |
| `--since`, `-s` | empty | Time range, such as `7d`, `30d`, `6m`, or `1y` |
| `--batch-size`, `-b` | `learning.history.batch_size`; `10` when config is not loaded | Commits per batch; each batch is analyzed by one agent call and candidate patterns are curated before storage |
| `--help`, `-h` | `false` | Show `learn history` help |

#### `--profile` Values

| Value | Description |
|---|---|
| `auto` | Creates the project profile when missing; refreshes when this run writes new or updated patterns; otherwise skips |
| `skip` | Learn patterns only |
| `refresh` | Force profile refresh from the current input |

#### Common Examples

```bash
skills-seed learn current
skills-seed learn current --focus internal/service --profile skip
skills-seed learn current --force --profile refresh
skills-seed learn current -f internal/agent -f internal/service
skills-seed learn current --context "Focus on compatibility boundaries"
skills-seed learn current --context-path .skills-seed/context.md
skills-seed learn history --limit 50
skills-seed learn history --since 30d
skills-seed learn history --limit 40 --batch-size 5
```

#### Notes

1. After the first successful run, Skills Seed records md5 fingerprints for analyzed files. If no learnable files changed, pattern learning and profile refresh are skipped.
2. Generated skill directories are excluded by default, including configured `skills.paths`, `.claude/skills/**`, and `.agents/skills/**`.
3. The workspace root coordinates learning and does not store child patterns in root storage.
4. Workspace child projects run with real concurrency according to `agent.parallelism`.
5. After child learning completes, the workspace root still analyzes the workspace profile, workspace rules, and saves relationship artifacts; terminal progress stays visible during these longer agent calls.
6. The workspace root records an md5 for relationship-fact inputs. When `workspace.projects`, child project profiles, and this run's one-shot context are unchanged, and workspace profile/spec artifacts already exist, root profile/spec analysis is skipped. CLI version or prompt-template changes no longer retrigger relationship learning by themselves; an explicit `generate skills` run rebuilds generated outputs directly.
7. Persistent project context belongs in `.skills-seed/context/`; `--context` and `--context-path` affect only the current command.
8. `learn current` uses file snapshots to detect added, modified, and deleted states. After analysis, snapshots are replaced within the current scope so the next run computes diffs from the new clean snapshot.
9. When bounded inputs such as focus paths, diffs, samples, or entry files exist, learning and project-profile analysis use the embedded tree-sitter structural pre-scan configured by `learning.current.structural`; without bounded inputs, it does not scan the whole repository.
10. When an agent hits retryable errors such as 429 / 529 / overloaded, Skills Seed retries according to `agent.retry`; the active progress line shows the agent error, failed call duration, and backoff wait, then switches to `attempt N` when the next call starts.

### `skills-seed generate`

#### Command Overview

Generate AI Agent related outputs. Currently supports the `skills` subcommand.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed generate skills` | Generate skills from patterns and project profile | `skills-seed generate skills --output .agents/skills/my-project` | Defaults to `skills.paths` for the current `skills.target` |

#### `generate` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `generate` help |

#### `generate skills` Flags

| Flag | Default | Description |
|---|---:|---|
| `--output`, `-o` | current `skills.target`'s `skills.paths` | Temporarily override the skills output directory |
| `--no-references` | `false` | Generate only the entry `SKILL.md` and skip detailed `references/` files |
| `--help`, `-h` | `false` | Show `generate skills` help |

#### Common Examples

```bash
skills-seed generate skills
skills-seed generate skills --output .agents/skills/my-project
```

One-shot guidance is only accepted during learning, for example `skills-seed learn current --context-path .skills-seed/run-context.md`. `generate skills` only consumes learned project profiles, workspace profile/spec, patterns, and long-lived context under `.skills-seed/context/`.

#### Project Context Notes

Files under `.skills-seed/context/` are merged with built-in prompts; they do not replace built-in prompts. Common persistent guidance locations:

- `.skills-seed/context/background.md`: business background, external systems, and production facts not visible in code.
- `.skills-seed/context/constraints.md`: long-lived team constraints, compatibility requirements, security boundaries, and forbidden changes.
- `.skills-seed/context/terminology.md`: terms, aliases, state names, and mappings from business language to code terms.
- `.skills-seed/context/workspace.md`: workspace-level context, generated only in workspace mode.

The merge order is built-in prompt, `context/background.md`, `context/constraints.md`, `context/terminology.md`, `context/workspace.md`, then a built-in final output contract. User files cannot override the final output contract; it protects the JSON / Markdown output format expected by parsers.

#### Generated Content

```text
SKILL.md
agents/
references/
  project-overview.md
  project-spec.md
  patterns/*.md
  examples/*.md
```

`SKILL.md` includes summary-stage key insights and improvement suggestions when the agent returns those fields, giving the entry skill extra project-specific judgment context.

#### Notes

1. Workspace mode regenerates each child skill using that child's own config first, then regenerates the workspace root skill.
2. A manual `SKILL.md` without a `generated-by: skills-seed` marker is not overwritten by default.
3. Generation ranking uses `EffectiveScore*0.6 + normalized(HitCount)*0.3 + Confidence*0.1`. `review stats` remains observational and does not directly affect generation.
4. `generate skills` does not check a generation-input fingerprint. When run explicitly, it deletes the old skills-seed generated output directory and fully rebuilds from the current profile, patterns, and workflows.

### `skills-seed preview`

#### Command Overview

Preview files selected for full or incremental analysis under the current configuration without calling an AI agent. Use it to debug `exclude.paths`, `exclude.gitignore`, focus paths, and file-selection behavior.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed preview files` | Preview files selected for analysis | `skills-seed preview files --mode incremental --focus internal/service` | Prints file-selection results only; does not learn patterns |

#### `preview` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `preview` help |

#### `preview files` Flags

| Flag | Default | Description |
|---|---:|---|
| `--mode` | `full` | Preview mode: `full`/`first` for full selection, `incremental`/`current` for current snapshot diffs |
| `--focus`, `-f` | empty | Preview only files under these paths; may be repeated |
| `--limit` | `200` | Maximum number of files to print |
| `--help`, `-h` | `false` | Show `preview files` help |

#### Common Examples

```bash
skills-seed preview files
skills-seed preview files --mode full
skills-seed preview files --mode incremental
skills-seed preview files --mode incremental --focus internal/service
skills-seed preview files --limit 500
```

#### Notes

1. `preview files` shares the file-selection policy used by `learn current`, so it shows which files would enter learning analysis.
2. `--mode incremental` shows added, modified, and deleted candidates from the current file snapshot. Without an existing snapshot, the result is close to first-run learning scope.
3. Skipped counts help confirm whether documents, configured excludes, or Git ignore rules filtered the expected files.

### `skills-seed patterns`

#### Command Overview

Manage learned patterns. Supports adding user-defined patterns, compacting semantically similar patterns, and inspecting DB fields, pattern quality, and check-hit statistics.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed patterns add --context <description>` | Define a pattern in natural language; AI generates a structured pattern | `skills-seed patterns add --context "Use RESTful API routes" --category api` | Calls the AI agent |
| `skills-seed patterns update <pattern-id> --context <request>` | Update one pattern while preserving its original ID and ownership | `skills-seed patterns update resp-extra-update-logging --context "Require audit logging"` | Calls the AI agent |
| `skills-seed patterns delete <pattern-id>` | Delete a pattern by ID | `skills-seed patterns delete plugin-source-editing-rule` | Workspace root also deletes the linked child project pattern |
| `skills-seed patterns compact` | Compact similar patterns locally by default; call the Agent for semantic merging only with `--ai` | `skills-seed patterns compact --category api --dry-run` | Use `--dry-run` to preview without writing to the database |
| `skills-seed patterns stats` | Show pattern quality and check-hit statistics | `skills-seed patterns stats` | Does not call the AI agent or modify the database |
| `skills-seed patterns show [pattern-id]` | Show the overview without arguments, or full details for one ID | `skills-seed patterns show business-create-order --format json` | Does not call the AI agent or modify the database |

#### `patterns` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `patterns` help |

#### `patterns add` Flags

| Flag | Default | Description |
|---|---:|---|
| `--category`, `-c` | empty | Specify a category, such as `business`, `api`, or `testing`; leave empty for AI auto-detection |
| `--context` | empty | User-provided natural-language pattern description; required |
| `--context-path` | empty | Read natural-language pattern description or one-shot reference material from files or directories; may be repeated |
| `--help`, `-h` | `false` | Show `patterns add` help |

When run from a workspace root, `patterns add` writes the root pattern first. If the description mentions a child project id or path, it also writes the child project's pattern database. Skills are regenerated by `sync` or an explicit `generate skills` run.

#### `patterns update` Flags

| Flag | Default | Description |
|---|---:|---|
| `--category`, `-c` | empty | Specify the revised category; empty keeps the existing category |
| `--context` | empty | User-provided natural-language update request; required |
| `--context-path` | empty | Read natural-language update request or one-shot reference material from files or directories; may be repeated |
| `--help`, `-h` | `false` | Show `patterns update` help |

#### `patterns delete` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `patterns delete` help |

#### `patterns compact` Flags

| Flag | Default | Description |
|---|---:|---|
| `--ai` | `false` | Use AI for semantic merging; default uses local deterministic merging and does not call the Agent |
| `--category`, `-c` | empty | Compact only one category, such as `business`, `api`, or `testing`; empty means all |
| `--dry-run` | `false` | Preview compact results without writing to the database |
| `--help`, `-h` | `false` | Show `patterns compact` help |

#### `patterns stats` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `patterns stats` help |

#### `patterns show` Flags

| Flag | Default | Description |
|---|---:|---|
| `--format` | `table` | Output format: `table` or `json` |
| `--help`, `-h` | `false` | Show `patterns show` help |
| `--sort` | `updated` | Overview sort: `updated`, `score`, `hits`, or `category` |

#### Common Examples

```bash
skills-seed patterns add --context "All API routes use RESTful style"
skills-seed patterns add --context "Errors must wrap context" --category error
skills-seed patterns add --context-path docs/pattern-notes.md --category database
skills-seed patterns update resp-extra-update-logging --context "Require audit logging for response extra field updates"
skills-seed patterns update resp-extra-update-logging --context-path docs/pattern-update.md
skills-seed patterns delete plugin-source-editing-rule
skills-seed patterns compact
skills-seed patterns compact --category api
skills-seed patterns compact --category business --dry-run
skills-seed patterns compact --ai --dry-run
skills-seed patterns stats
skills-seed patterns show
skills-seed patterns show --sort score
skills-seed patterns show business-create-order
skills-seed patterns show business-create-order --format json
```

#### Notes

1. `patterns compact` uses local deterministic merging by default and does not call the Agent. It calls the CLI configured by the current `agent.engine` only when `--ai` is set.
2. Use `--dry-run` first when you want to inspect the curation result.
3. `patterns stats` uses recorded check-hit data. Hit counts appear only after checks produce issues with `PatternID`.
4. `patterns show` without arguments prints the pattern overview list, sorted by latest update by default. Use `--sort score` for high-value rules, `--sort hits` for frequently hit rules, and `--sort category` for category grouping. The location column prefers business/utility-method `code_location`; when a pattern has no business method, it falls back to the first pattern-level `evidence_locations` entry. Passing a `pattern-id` prints the full detail view for one pattern, including good/bad examples, quality metrics, workspace ownership, evidence locations, business-method fields, code-location history, and language-agnostic symbol snapshots.
5. `patterns stats` and `patterns show` do not call AI and do not modify data, but they still need to open `.skills-seed/store/project.db`. If another `skills-seed` command is holding the database, the CLI asks you to wait for that command to finish or check for a stale process.

### `skills-seed review`

#### Command Overview

Import local code review comments and compare them with recorded pattern hits by file and line window to measure which comments may already be covered by existing patterns.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed review import --from-file <file>` | Import a JSON array of review comments | `skills-seed review import --from-file review-comments.json` | Saved by comment `id`; importing the same comment again does not duplicate counts |
| `skills-seed review stats` | Show review comment prevention statistics | `skills-seed review stats --line-window 3` | Does not call the AI agent or modify the database |

#### `review` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `review` help |

#### `review import` Flags

| Flag | Default | Description |
|---|---:|---|
| `--from-file` | required | JSON file containing a review comment array |
| `--help`, `-h` | `false` | Show `review import` help |

#### `review stats` Flags

| Flag | Default | Description |
|---|---:|---|
| `--line-window` | `3` | Allowed line distance when matching a review comment to an existing pattern hit |
| `--help`, `-h` | `false` | Show `review stats` help |

#### Import JSON Fields

| Field | Description |
|---|---|
| `id` | Unique comment ID |
| `provider` | Source, such as `local`, `github`, or `gitlab` |
| `review_id` | Review ID |
| `commit` | Related commit |
| `file` | File path |
| `line` | Comment line number |
| `author` | Comment author |
| `body` | Comment body |
| `resolved` | Whether the comment is resolved |
| `created_at` | RFC3339 timestamp, such as `2026-05-28T09:02:00Z` |

#### Common Examples

```bash
skills-seed review import --from-file review-comments.json
skills-seed review stats
skills-seed review stats --line-window 5
```

#### Notes

1. The MVP supports local JSON import only; it does not connect to GitHub or GitLab directly.
2. `review stats` depends on existing `check` hit records. Without pattern hits, imported comments are counted as missed.
3. Matching requires the same file path and a line distance within `--line-window`.

### `skills-seed profile`

#### Command Overview

Show or refresh the project profile. The profile is stored at `.skills-seed/store/documents/project-profile.json` and is used to generate `references/project-overview.md`.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed profile show` | Print the current project profile summary | `skills-seed profile show` | Does not call the AI agent or modify the database |
| `skills-seed profile refresh` | Re-analyze the project and overwrite the profile | `skills-seed profile refresh --language go` | Does not learn patterns; only refreshes the profile |

#### `profile` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `profile` help |

#### `profile refresh` Flags

| Flag | Default | Description |
|---|---:|---|
| `--language`, `-l` | config or auto-detect | Temporarily specify the project language |
| `--help`, `-h` | `false` | Show `profile refresh` help |

#### Common Examples

```bash
skills-seed profile show
skills-seed profile refresh
skills-seed profile refresh --language go
```

#### Notes

1. `profile show` is useful for quickly checking the current profile.
2. `profile refresh` overwrites the existing project profile, but does not run pattern learning.

### `skills-seed sync`

#### Command Overview

One-step sync: learn current code, then generate skills. `--context` and `--context-path` are passed only as background for this learning run. Use `patterns add/update` to add or revise user-defined patterns from natural language.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed sync` | learn current → generate skills | `skills-seed sync` | Resumes unfinished sync state first; generates skills when learning changed output |
| `skills-seed sync --context <background>` | learn current with context → generate skills | `skills-seed sync --context "On-prem deployment, not SaaS"` | Provides one-shot analysis background and does not write a user pattern |
#### Flags

| Flag | Default | Description |
|---|---:|---|
| `--context` | empty | Extra background for this learning run; only affects learn current prompts |
| `--context-path` | empty | Read extra background for this learning run from files or directories; may be repeated |
| `--help`, `-h` | `false` | Show `sync` help |

#### Common Examples

```bash
skills-seed sync
skills-seed sync --context "On-prem deployment, not SaaS"
skills-seed sync --context-path docs/plan.md --context-path docs/specs
skills-seed sync --restart
```

#### Notes

1. `sync` runs `learn current` first by default; it continues to `generate skills` only when this run writes new/updated patterns or changes workspace relationship artifacts.
2. `sync --context` does not add a user pattern; it only affects this learning analysis. Use `patterns add` or `patterns update` to add user-defined patterns.
3. If any step fails, subsequent steps are skipped.

### `skills-seed check`

#### Command Overview

Check staged files or all Git-tracked files against learned patterns, with optional interactive handling.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed check` | Check staged files or all Git-tracked files | `skills-seed check --all --interactive=false` | Defaults to staged files only |

#### Flags

| Flag | Default | Description |
|---|---:|---|
| `--interactive`, `-i` | `true` | Enable interactive fix confirmation; hooks usually use `false` |
| `--all`, `-a` | `false` | Check all Git-tracked files; default checks staged files only |
| `--help`, `-h` | `false` | Show `check` help |

#### Common Examples

```bash
skills-seed check
skills-seed check --all
skills-seed check --interactive=false
```

#### Notes

1. Run `skills-seed check --interactive=false` directly when you only need checks.
2. Without `--all`, only the Git staging area is checked.
3. When interactive fix generation is used, the agent's fix summary is printed to logs. Files that cannot be safely rewritten in full are surfaced as manual-review warnings instead of being forced into incomplete fixes.

### `skills-seed hook`

#### Command Overview

Manage Git pre-commit hooks. After installation, commits open an interactive menu where the user can sync and generate skills, learn only, or skip this time.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed hook install` | Write `.git/hooks/pre-commit` | `skills-seed hook install` | Opens a selection menu before commit |
| `skills-seed hook uninstall` | Remove `.git/hooks/pre-commit` | `skills-seed hook uninstall` | Does not remove `.skills-seed` data |
| `skills-seed hook run` | Open the hook menu manually | `skills-seed hook run` | Non-interactive environments skip directly |

#### `hook` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `hook` help |

#### Subcommand Flags

| Subcommand | Flag | Default | Description |
|---|---|---:|---|
| `hook install` | `--help`, `-h` | `false` | Show `hook install` help |
| `hook uninstall` | `--help`, `-h` | `false` | Show `hook uninstall` help |
| `hook run` | `--help`, `-h` | `false` | Show `hook run` help |

#### Common Examples

```bash
skills-seed hook install
skills-seed hook uninstall
skills-seed hook run
```

#### Notes

1. `hook run` defaults to skip, avoiding high-cost AI learning as the default commit action.
2. Non-interactive terminals skip directly, so scripts, IDEs, and Git automation are not blocked.
3. `hook uninstall` only removes the hook file; learned data is preserved.

### `skills-seed log`

#### Command Overview

Show recent changes recorded into project skills. This command reads `.skills-seed/store/documents/change-log.json`, prints a git-log-like history, and does not print diagnostic logs.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed log` | Show recent learned changes | `skills-seed log` | Prints all change records in reverse chronological order |

#### Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `log` help |

#### Common Examples

```bash
skills-seed log
```

#### Notes

1. `sync`, `learn current`, and `generate skills` write learned change records.
2. Detailed diagnostic logs remain under `.skills-seed/runtime/logs/` for troubleshooting.

### `skills-seed help`

#### Command Overview

Show help for any command path. This command is provided by Cobra.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed help [command]` | Show help for a command path | `skills-seed help learn current` | Equivalent to that command's `--help` |

#### Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show help for the `help` command |

#### Common Examples

```bash
skills-seed help init
skills-seed help learn current
```

#### Notes

1. `skills-seed help <command>` is convenient for multi-level subcommands.
2. `skills-seed <command> --help` prints the same command help.
