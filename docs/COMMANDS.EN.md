# Skills Seed Command Reference

[简体中文](COMMANDS.md) | [English](COMMANDS.EN.md)

This is the complete command reference. Every command supports `--help`. Commands that read `.skills-seed/config.yaml` require `skills-seed init` first.

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

Initialize `.skills-seed/`, default config, database, and prompt / skills templates in a Git repository. Supports both single-project and workspace modes.

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
| `--skills-locale` | empty | Generated Skills and AI prompt language: `zh-CN` or `en-US`; empty uses the built-in default `en-US` |
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
| `--skills-locale` | empty | Generated Skills and AI prompt language after reset: `zh-CN` or `en-US`; empty uses the built-in default `en-US` |
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
| `skills-seed learn current` | Incrementally learn from the current codebase | `skills-seed learn current --focus internal/service --profile skip` | Compares file md5 values and learns only added, modified, or deleted files |
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
| `--context` | empty | One-time guidance for this learn run, passed to the AI agent and not written to `.skills-seed/prompts/` |
| `--context-file` | empty | Read one-time guidance for this learn run from a file; not written to `.skills-seed/prompts/` |
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
skills-seed learn current -f internal/agent -f internal/service
skills-seed learn current --context "Focus on compatibility boundaries"
skills-seed learn current --context-file .skills-seed/context.md
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
6. The workspace root records an md5 for relationship-analysis inputs. When `workspace.projects`, child project profiles, prompt templates, and this run's one-shot context are unchanged, and workspace profile/spec artifacts already exist, root profile/spec analysis is skipped.
7. Persistent prompt guidance belongs in `.skills-seed/prompts/instructions/<prompt-id>.md`; `--context` and `--context-file` affect only the current command.
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
| `--force` | `false` | Ignore pattern dirty state and regenerate all skills |
| `--help`, `-h` | `false` | Show `generate skills` help |

#### Common Examples

```bash
skills-seed generate skills
skills-seed generate skills --output .agents/skills/my-project
```

One-shot guidance is only accepted during learning, for example `skills-seed learn current --context-file .skills-seed/context.md`. `generate skills` only consumes learned project profiles, workspace profile/spec, and patterns.

#### Prompt Merge Notes

Files under `.skills-seed/prompts/` are merged with built-in prompts; they do not replace built-in prompts. Common persistent guidance locations:

- `.skills-seed/prompts/project/project-profile.md`: project facts.
- `.skills-seed/prompts/project/common.md`: common project constraints.
- `.skills-seed/prompts/project/<prompt-id>.md`: project-level fragment for one prompt.
- `.skills-seed/prompts/workspace/<prompt-id>.md`: workspace-level fragment.
- `.skills-seed/prompts/instructions/<prompt-id>.md`: user instructions.

The merge order is built-in prompt, project profile, common project constraints, project-level fragment, workspace fragment, user instructions, then a built-in final output contract. User files cannot override the final output contract; it protects the JSON / Markdown output format expected by parsers.

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

1. Workspace mode generates each child skill using that child's own config first, then generates the workspace root skill on the first run; later runs generate only targets affected by pattern or workspace-relationship changes.
2. A manual `SKILL.md` without a `generated-by: skills-seed` marker is not overwritten by default.
3. Generation ranking uses `EffectiveScore*0.6 + normalized(HitCount)*0.3 + Confidence*0.1`. `review stats` remains observational and does not directly affect generation.
4. `generate skills` records an md5 for generation inputs. When project profile, patterns, hit stats, config, prompt/skill templates, and output path are unchanged, and generated outputs are complete, Skills Seed skips the agent summary and file rewrite. Workspace root skills use the same mechanism for unchanged root outputs.
5. Use `skills-seed generate skills --force` when every target must be regenerated.

### `skills-seed patterns`

#### Command Overview

Manage learned patterns. Supports adding user-defined patterns, compacting semantically similar patterns, and inspecting DB fields, pattern quality, and check-hit statistics.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed patterns add <description>` | Define a pattern in natural language; AI generates a structured pattern | `skills-seed patterns add "Use RESTful API routes" --category api` | Calls the AI agent |
| `skills-seed patterns delete <pattern-id>` | Delete a pattern by ID | `skills-seed patterns delete plugin-source-editing-rule` | Workspace root also deletes the linked child project pattern |
| `skills-seed patterns compact` | Ask the current agent to curate and compact similar patterns | `skills-seed patterns compact --category api --dry-run` | Use `--dry-run` to preview without writing to the database |
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
| `--files`, `-f` | empty | Reference file path; repeat this flag for multiple files. AI reads the files to help generate the pattern |
| `--help`, `-h` | `false` | Show `patterns add` help |

When run from a workspace root, `patterns add` writes the root pattern first. If the description mentions a child project id or path, it also writes the child project's pattern database and marks only affected skills targets dirty.

#### `patterns delete` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `patterns delete` help |

#### `patterns compact` Flags

| Flag | Default | Description |
|---|---:|---|
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

#### Common Examples

```bash
skills-seed patterns add "All API routes use RESTful style"
skills-seed patterns add "Errors must wrap context" --category error
skills-seed patterns add "Database operations use transactions; project uses GORM" --files internal/service/user.go
skills-seed patterns delete plugin-source-editing-rule
skills-seed patterns compact
skills-seed patterns compact --category api
skills-seed patterns compact --category business --dry-run
skills-seed patterns stats
skills-seed patterns show
skills-seed patterns show business-create-order
skills-seed patterns show business-create-order --format json
```

#### Notes

1. `patterns compact` calls the CLI configured by the current `agent.engine`.
2. Use `--dry-run` first when you want to inspect the curation result.
3. `patterns stats` uses recorded check-hit data. Hit counts appear only after checks produce issues with `PatternID`.
4. `patterns show` without arguments prints the pattern overview list; passing a `pattern-id` prints the full detail view for one pattern, including good/bad examples, quality metrics, workspace ownership, business-method fields, code-location history, and language-agnostic symbol snapshots.
5. `patterns stats` and `patterns show` do not call AI and do not modify data, but they still need to open `.skills-seed/memory/project.db`. If another `skills-seed` command is holding the database, the CLI asks you to wait for that command to finish or check for a stale process.

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

Show or refresh the project profile. The profile is stored at `.skills-seed/memory/project-profile.json` and is used to generate `references/project-overview.md`.

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

One-step sync: learn current code, then generate skills. When `--add` is provided, learning is skipped and a user-defined pattern is created before generation.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed sync` | Learn current → generate skills | `skills-seed sync` | Equivalent to `learn current`, `generate skills` in sequence |
| `skills-seed sync --add <desc>` | patterns add → generate skills | `skills-seed sync --add "Use RESTful API routes"` | Skips learning; good for patterns the AI did not discover |

#### Flags

| Flag | Default | Description |
|---|---:|---|
| `--add` | empty | Natural language pattern description; triggers patterns add → generate |
| `--category`, `-c` | empty | Category for `--add` mode |
| `--files`, `-f` | empty | Reference file path for `--add` mode; repeat this flag for multiple files |
| `--context` | empty | Additional context; plain `sync` passes it to `learn current`, while `sync --add` passes it to user pattern generation |
| `--help`, `-h` | `false` | Show `sync` help |

#### Common Examples

```bash
skills-seed sync
skills-seed sync --add "All API routes use RESTful style"
skills-seed sync --add "Errors must wrap context" --category error
skills-seed sync --add "Database operations use transactions" --files internal/service/user.go
skills-seed sync --context "Focus on compatibility boundaries for this run"
```

#### Notes

1. `sync` without `--add` runs `learn current`, then `generate skills`; pattern curation happens while learned candidates are stored.
2. `sync --add` skips learning and defines a pattern from natural language, useful for patterns the AI missed.
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

1. Pre-commit hooks usually run `skills-seed check --interactive=false`.
2. Without `--all`, only the Git staging area is checked.
3. When interactive fix generation is used, the agent's fix summary is printed to logs. Files that cannot be safely rewritten in full are surfaced as manual-review warnings instead of being forced into incomplete fixes.

### `skills-seed hook`

#### Command Overview

Manage Git pre-commit hooks. Subcommands are recommended; `--install` and `--uninstall` remain for legacy compatibility.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed hook install` | Write `.git/hooks/pre-commit` | `skills-seed hook install` | Runs `skills-seed check --interactive=false` before commit |
| `skills-seed hook uninstall` | Remove `.git/hooks/pre-commit` | `skills-seed hook uninstall` | Does not remove `.skills-seed` data |
| `skills-seed hook run` | Manually run hook logic on staged files | `skills-seed hook run` | Useful for local pre-commit verification |

#### `hook` Flags

| Flag | Default | Description |
|---|---:|---|
| `--install`, `-i` | `false` | Install Git pre-commit hook; prefer `hook install` |
| `--uninstall`, `-u` | `false` | Uninstall Git pre-commit hook; prefer `hook uninstall` |
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
skills-seed hook --install
skills-seed hook --uninstall
```

#### Notes

1. Legacy `--install` / `--uninstall` flags still work, but subcommands are preferred.
2. `hook uninstall` only removes the hook file; learned data is preserved.

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
