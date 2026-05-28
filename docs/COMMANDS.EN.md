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
| `skills-seed init` | Initialize the current repository | `skills-seed init --mode project --agent codex --locale en-US` | Must run from a Git repository root; existing `.skills-seed` is not overwritten |
| `skills-seed init children` | Initialize child projects from root `workspace.projects` | `skills-seed init children --locale en-US` | Only works from a root repo with `project.mode: "workspace"` |

#### `init` Flags

| Flag | Default | Description |
|---|---:|---|
| `--mode` | `project` | Initialization mode: `project` for a single project, `workspace` for a multi-project root |
| `--agent` | `claude` | Agent provider to write during initialization, for example `claude` or `codex` |
| `--workspace` | `false` | Shortcut for `--mode workspace` |
| `--children` | `false` | With workspace mode, initialize child projects missing `.skills-seed` after the root is initialized |
| `--locale`, `-l` | auto-detect | Config language: `zh-CN` or `en-US` |
| `--help`, `-h` | `false` | Show `init` help |

#### `init children` Flags

| Flag | Default | Description |
|---|---:|---|
| `--locale`, `-l` | root config language | Child project config language: `zh-CN` or `en-US` |
| `--help`, `-h` | `false` | Show `init children` help |

#### Common Examples

```bash
skills-seed init --mode project --locale en-US
skills-seed init --mode project --agent codex --locale en-US
skills-seed init --mode workspace --locale en-US
skills-seed init --workspace
skills-seed init --workspace --children --agent codex
skills-seed init children
```

#### Notes

1. `--agent` sets `agent.provider` and ensures the provider exists in `agent.commands` and `output.skills_paths`.
2. `--workspace --children` initializes the root first, then child repositories missing `.skills-seed` from `workspace.projects`.
3. Newly initialized child repositories inherit root `agent.provider`, `agent.commands`, and `output.skills_paths`.
4. Already initialized children are skipped. If a child agent differs from the root, it is reported and preserved.
5. A successful init prints the relative `.skills-seed` location and the README URL for the current version tag.

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
| `--locale`, `-l` | auto-detect | Config language after reset: `zh-CN` or `en-US` |
| `--help`, `-h` | `false` | Show `reset` help |

#### Common Examples

```bash
skills-seed reset --mode project
skills-seed reset --mode workspace
skills-seed reset --workspace
```

#### Notes

1. Use `reset` to reinitialize or choose another mode.
2. `project.mode` is locked after learning or skill generation starts and should not be changed directly in config.

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
| `--context` | empty | Additional guidance for this learn run, passed to the AI agent |
| `--context-file` | empty | Read additional guidance for this learn run from a file |
| `--help`, `-h` | `false` | Show `learn current` help |

#### `learn history` Flags

| Flag | Default | Description |
|---|---:|---|
| `--limit`, `-n` | `learning.max_commits`, default `50` | Maximum number of commits to analyze |
| `--since`, `-s` | empty | Time range, such as `7d`, `30d`, `6m`, or `1y` |
| `--batch-size`, `-b` | `learning.batch_size`, default `5` | Commits per batch; each batch is merged into one agent call |
| `--help`, `-h` | `false` | Show `learn history` help |

#### `--profile` Values

| Value | Description |
|---|---|
| `auto` | Refreshes for first/full learning and skips narrow changes when possible |
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
2. Generated skill directories are excluded by default, including configured `output.skills_paths`, `.claude/skills/**`, and `.agents/skills/**`.
3. The workspace root coordinates learning and does not store child patterns in root storage.
4. When `workspace.init_children: true`, missing child `.skills-seed` directories are initialized from root agent config before learning.
5. Workspace child projects run with real concurrency according to `agent.parallelism`.

### `skills-seed generate-skills`

#### Command Overview

Generate skills for the current provider from the project profile and learned patterns. Generation calls the CLI configured by `agent.provider` for summary merging and polishing.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed generate-skills` | Generate skills for the current provider | `skills-seed generate-skills --output .agents/skills/my-project` | Defaults to `output.skills_paths` for the current provider |

#### Flags

| Flag | Default | Description |
|---|---:|---|
| `--output`, `-o` | current provider's `output.skills_paths` | Temporarily override the skills output directory |
| `--merge`, `-m` | `false` | Merge similar patterns before generation; deprecated, use `skills-seed patterns merge` |
| `--context` | empty | Additional guidance for this generate run, passed to the AI agent |
| `--context-file` | empty | Read additional guidance for this generate run from a file |
| `--help`, `-h` | `false` | Show `generate-skills` help |

#### Common Examples

```bash
skills-seed generate-skills
skills-seed generate-skills --output .agents/skills/my-project
skills-seed generate-skills --context "Preserve API compatibility constraints"
skills-seed generate-skills --context-file .skills-seed/context.md
```

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

#### Notes

1. Workspace mode generates each child skill using that child's own config first, then generates the workspace root skill.
2. A manual `SKILL.md` without a `generated-by: skills-seed` marker is not overwritten by default.
3. `--merge` is kept for compatibility. Prefer running `skills-seed patterns merge` explicitly.

### `skills-seed patterns`

#### Command Overview

Manage learned patterns. Currently this is mainly used to merge semantically similar patterns and reduce duplicate rules.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed patterns merge` | Ask the current agent to merge similar patterns | `skills-seed patterns merge --category api --dry-run` | Use `--dry-run` to preview without writing to the database |

#### `patterns` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `patterns` help |

#### `patterns merge` Flags

| Flag | Default | Description |
|---|---:|---|
| `--category`, `-c` | empty | Merge only one category, such as `business`, `api`, or `testing`; empty means all |
| `--dry-run` | `false` | Preview merge results without writing to the database |
| `--help`, `-h` | `false` | Show `patterns merge` help |

#### Common Examples

```bash
skills-seed patterns merge
skills-seed patterns merge --category api
skills-seed patterns merge --category business --dry-run
```

#### Notes

1. Merge runs call the CLI configured by the current `agent.provider`.
2. Use `--dry-run` first when you want to inspect the merge result.

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

### `skills-seed view`

#### Command Overview

View learned or generated content from `.skills-seed`. Currently this mainly shows learned patterns.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed view patterns` | View learned patterns grouped by category | `skills-seed view patterns --category testing` | Read-only; does not modify the database |

#### `view` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `view` help |

#### `view patterns` Flags

| Flag | Default | Description |
|---|---:|---|
| `--category`, `-c` | empty | Filter by category: `naming`, `error`, `structure`, `concurrency`, `testing`, `business`, `api`, `database`, `utils`, `middleware`, `config` |
| `--help`, `-h` | `false` | Show `view patterns` help |

#### Common Examples

```bash
skills-seed view patterns
skills-seed view patterns --category testing
```

#### Notes

1. Without `--category`, all categories are displayed.
2. This command is read-only and does not trigger learning, merging, or generation.

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

### `skills-seed completion`

#### Command Overview

Generate an autocompletion script for a specified shell. This command is provided by Cobra and is intended for local shell setup.

#### Command Forms

| Command Form | Description | Common Example | Notes |
|---|---|---|---|
| `skills-seed completion bash` | Generate Bash completion | `source <(skills-seed completion bash)` | Requires bash-completion |
| `skills-seed completion zsh` | Generate Zsh completion | `source <(skills-seed completion zsh)` | If completion is not enabled, run `autoload -U compinit; compinit` once |
| `skills-seed completion fish` | Generate Fish completion | `skills-seed completion fish \| source` | Can be written to `~/.config/fish/completions/skills-seed.fish` |
| `skills-seed completion powershell` | Generate PowerShell completion | `skills-seed completion powershell \| Out-String \| Invoke-Expression` | Can be added to the PowerShell profile |

#### `completion` Flags

| Flag | Default | Description |
|---|---:|---|
| `--help`, `-h` | `false` | Show `completion` help |

#### Subcommand Flags

| Subcommand | Flag | Default | Description |
|---|---|---:|---|
| `completion bash` | `--no-descriptions` | `false` | Disable completion descriptions |
| `completion bash` | `--help`, `-h` | `false` | Show Bash completion help |
| `completion zsh` | `--no-descriptions` | `false` | Disable completion descriptions |
| `completion zsh` | `--help`, `-h` | `false` | Show Zsh completion help |
| `completion fish` | `--no-descriptions` | `false` | Disable completion descriptions |
| `completion fish` | `--help`, `-h` | `false` | Show Fish completion help |
| `completion powershell` | `--no-descriptions` | `false` | Disable completion descriptions |
| `completion powershell` | `--help`, `-h` | `false` | Show PowerShell completion help |

#### Common Examples

```bash
source <(skills-seed completion bash)
source <(skills-seed completion zsh)
skills-seed completion fish | source
skills-seed completion powershell | Out-String | Invoke-Expression
```

#### Notes

1. For persistent installation, read `skills-seed completion <shell> --help`.
2. macOS/Linux completion installation paths depend on the shell and package manager.

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
skills-seed help completion zsh
```

#### Notes

1. `skills-seed help <command>` is convenient for multi-level subcommands.
2. `skills-seed <command> --help` prints the same command help.
