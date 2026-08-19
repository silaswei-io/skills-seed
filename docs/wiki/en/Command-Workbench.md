# Command Workbench

> **Use this page when:** you know the outcome you need but must choose the right command and subcommand. The [command reference](../../COMMANDS.EN.md) remains the source of truth for exact flags, defaults, and complete examples.

## Start With the Outcome

| Outcome | First command | Calls an Agent | Changes durable state |
|---|---|---:|---:|
| Adopt a project for the first time | `init` → `sync` | `sync` does | Yes |
| See what a run would analyze | `preview files` | No | No |
| Refresh learned knowledge only | `learn current` | Yes | Yes |
| Refresh knowledge and generated Skills | `sync` | Yes | Yes |
| Rebuild generated output only | `generate skills` | No | Yes |
| Inspect or correct knowledge | `patterns ...`, `profile show` | Depends on subcommand | Depends on subcommand |
| Maintain team constraints or procedures | `rule ...`, `workflow ...` | Yes when writing | Yes |
| Observe or recover an execution | `sync --resume`, `log`, `hook ...` | Depends on command | Depends on command |

## Initialize, Learn, and Generate

| Command | Use it when | Common form | Next action |
|---|---|---|---|
| `skills-seed` / `help` | You need the version or command help | `skills-seed help learn current` | Confirm flags in help |
| `init` | Initializing a project or workspace root | `skills-seed init --mode project` | Run `sync` |
| `workspace add` | Registering a workspace child project | `skills-seed workspace add <path>` | Run `sync` at the root |
| `preview files` | Confirming the learning file scope | `skills-seed preview files --mode incremental --focus <path>` | Correct filters or learn |
| `learn current` | Learning source without replacing generated Skills | `skills-seed learn current --focus <path>` | Inspect patterns/profile, then generate if needed |
| `sync` | Daily end-to-end refresh | `skills-seed sync` | Review output and `log` |
| `generate skills` | Knowledge changed and only the projection needs rebuilding | `skills-seed generate skills` | Review generated output |

`--context` and `--context-path` add background only to one learning run. They do not persist as a Rule, Workflow, or Pattern. Use Rules and Workflows for durable constraints and procedures.

## Inspect and Correct Knowledge

| Command | Use it when | Common form | Effect |
|---|---|---|---|
| `patterns show` | Reading all summaries or one complete pattern | `skills-seed patterns show <id> --format json` | Read only |
| `patterns stats` | Checking quality, category, or score distribution | `skills-seed patterns stats` | Read only |
| `patterns add` | Adding a user pattern that source cannot establish | `skills-seed patterns add --content "<description>"` | Agent normalizes and writes |
| `patterns update` | Revising a pattern while retaining its ID | `skills-seed patterns update <id> --content "<correction>"` | Agent normalizes and writes |
| `patterns delete` | Removing an incorrect, obsolete, or unwanted pattern | `skills-seed patterns delete <id>` | Deletes persisted pattern |
| `patterns compact` | Consolidating semantically similar patterns | `skills-seed patterns compact --dry-run` | `--dry-run` previews only |
| `profile show` | Reading the project-profile and project-map summary | `skills-seed profile show` | Read only |

Run `skills-seed generate skills` after a correction. When a quality issue comes from changed source rather than the pattern text, use `learn current` or `sync` to relearn it instead of masking source facts with a manual pattern.

## Rules and Workflows

| Command | Use it when | Common form | Key boundary |
|---|---|---|---|
| `rule` | Creating or incrementally refining a durable mandatory constraint | `skills-seed rule --name <name> --content "<constraint>"` | Same name refines by default; only `--overwrite` replaces fully |
| `rule show` | Viewing a rule summary or full content | `skills-seed rule show <id> --format json` | Read only |
| `workflow` | Creating or incrementally refining a task procedure | `skills-seed workflow --name <name> --content "<procedure>"` | Same name refines by default; conflicts remain to confirm |
| `workflow show` | Viewing a workflow summary or full content | `skills-seed workflow show <id> --format json` | Read only |

At a workspace root, a Rule's `--child`, `--project`, and `--path` must express a real, explicit scope. Do not let the tool or an Agent infer scope from prose. Run `generate skills` after saving either resource; see [Knowledge Maintenance](Knowledge-Maintenance.md) for the full process.

## Automation, Recovery, and Reset

| Command | Use it when | Common form | Risk boundary |
|---|---|---|---|
| `hook install` | Offering an interactive learning choice before commit | `skills-seed hook install` | Skips by default; never blocks non-interactive use |
| `hook run` | Manually testing the hook menu | `skills-seed hook run` | Meaningful only interactively |
| `hook uninstall` | Removing the current project's hook | `skills-seed hook uninstall` | Does not delete learned data |
| `update` | Updating the installed CLI | `skills-seed update` | Downloads and verifies from GitHub Releases; animates lookup, download, verification, and installation; needs neither Go nor project state |
| `cli-skills install` | Installing the global CLI operation Skill | `skills-seed cli-skills install --target auto` | Does not manage generated project Skills |
| `cli-skills uninstall` | Removing that global CLI operation Skill | `skills-seed cli-skills uninstall --target codex` | Deletes only the fixed global target |
| `log` | Reading recent learning and generation changes | `skills-seed log` | Read-only summary, not detailed diagnostics |
| `sync --resume` | Continuing from a recoverable checkpoint | `skills-seed sync --resume` | Prefer over starting again |
| `sync --restart` | Explicitly discarding this run's recovery state | `skills-seed sync --restart` | Use only for incompatible state or untrustworthy input |
| `reset all` | Discarding all resettable knowledge before relearning | `skills-seed reset all` | Preserves config, Context, and generated output; resources enter a backup |
| `reset patterns ...` | Discarding only selected knowledge | `skills-seed reset patterns rules` | Combine patterns, rules, and workflows; patterns also clear dependent learning state |
| `reset` | Backing up all initialization state or changing mode | `skills-seed reset --mode project` | Moves old `.skills-seed` into a backup directory |

Run `skills-seed sync` immediately after a selective reset. For an interrupted run or one poor pattern, start with [Learning Failures and Manual Repair](Learning-Failures-and-Manual-Repair.md); correcting one pattern body usually starts with `patterns update`.

`update` uses the system `HTTPS_PROXY`, `HTTP_PROXY`, and `NO_PROXY` environment variables to reach GitHub Releases. The download stage shows bytes downloaded, total size, and percentage; each request waits for at most two minutes. If it fails after pausing at a stage, check the corresponding network path or proxy before retrying.

---

**Next:** [Knowledge Maintenance](Knowledge-Maintenance.md) - correct knowledge at the source for Patterns, Rules, Workflows, and the profile.

**Related:** [Learning and Sync](Learning-and-Sync.md) - understand the learning stages; [Learning Failures and Manual Repair](Learning-Failures-and-Manual-Repair.md) - choose resumption, restart, or reset.

**Language:** [简体中文](../Command-Workbench.md)
