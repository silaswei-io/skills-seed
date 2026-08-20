# Knowledge Maintenance

> **Use this page when:** learned patterns are inaccurate, incomplete, or redundant, or when you need to maintain Rules, Workflows, and the project profile. Correct the source of truth and regenerate; do not edit the generated Skill directly.

## Identify What to Maintain First

| Finding | Maintain this | Do not |
|---|---|---|
| Source changed and an old conclusion is stale | Relearn with `learn current` or `sync` | Mask source facts with a user pattern |
| A pattern is wrong, too broad, or missing a boundary | `patterns update` | Edit a generated reference |
| A pattern should not exist | `patterns delete` | Delete only its generated file |
| Several patterns express the same thing | `patterns compact --dry-run`, then compact | Edit the database or runtime directly |
| A long-lived team prohibition or contract | `rule` | Assume source can derive it |
| Release, acceptance, deployment, or other task steps | `workflow` | Let source learning infer it as fact |
| Authority files, structure, or module map changed | `learn current --profile refresh` | Hand-edit profile JSON |

## Reset Knowledge Before Relearning

When incorrect or stale knowledge affects a whole set of resources and individual repair is no longer appropriate, use `reset` to clear the relevant source of truth before relearning. Selective reset keeps config, durable Context, and generated Skills; removed resources first move to `.skills-seed.backup/<timestamp>/knowledge/`.

| Goal | Command | Next action |
|---|---|---|
| Relearn everything from clean knowledge | `skills-seed reset all` | `skills-seed sync` |
| Relearn only source patterns, profiles, and analysis state | `skills-seed reset patterns` | `skills-seed sync` |
| Remove only team Rules | `skills-seed reset rules` | Add needed `rule` resources again, then `generate skills` |
| Remove only task Workflows | `skills-seed reset workflows` | Add needed `workflow` resources again, then `generate skills` |

`patterns` also clears project profiles, file snapshots, resumable checkpoints, and learning history so the next `sync` cannot reuse stale analysis. Scopes can be combined, such as `skills-seed reset patterns rules`; `all` must stand alone, and a selective reset cannot be combined with `--mode`, `--workspace`, `--locale`, or `--skills-locale`.

For one pattern, prefer `patterns update`; for interrupted learning, prefer `sync --resume`. Full `skills-seed reset` without a scope is for reinitializing or switching between project and workspace modes.

`patterns update` is project-scoped and must run from an initialized project or workspace directory. To read its help without opening project runtime state, use `skills-seed patterns update --help`.

## Manual Pattern-Maintenance Loop

1. Run `skills-seed patterns stats` to inspect count, categories, and quality indicators.
2. Use `skills-seed patterns show --sort score` to find important entries, then `patterns show <id> --format json` to read full evidence, scope, and wording.
3. Choose `add`, `update`, `delete`, or `compact --dry-run` based on the finding.
4. If the preview is correct, rerun `compact` without `--dry-run`.
5. Run `skills-seed generate skills` and inspect the entry routing and the corresponding reference projection.

For an overreaching pattern, clearly state the proven fact to preserve, the scope to narrow, and the unknown to leave open:

```bash
skills-seed patterns update <pattern-id> \
  --content "Keep the proven implementation fact; limit scope to evidenced locations; mark unknown guarantees for source verification."
skills-seed generate skills
```

`patterns add` and `patterns update` call an Agent to normalize natural language, so review the saved result. `patterns compact` is local and deterministic, making `--dry-run` the appropriate first review step.

## Maintain Rules and Workflows

Rules are authoritative constraints; Workflows are user-defined task procedures. Both keep their source of truth in `.skills-seed` and are projected into a Skill during generation.

```bash
# Add or extend a durable rule; an existing name is refined by default.
skills-seed rule --name <rule-name> --content "<explicit constraint>"

# Add or extend a task procedure; an existing name is merged and conflicts remain visible.
skills-seed workflow --name <workflow-name> --content "<explicit procedure>"

# Read what was saved, then generate its projection.
skills-seed rule show <rule-id> --format json
skills-seed workflow show <workflow-id> --format json
skills-seed generate skills
```

Use `--overwrite` only when the old body and scope must genuinely be discarded. At a workspace root, state `--child`, `--project`, or `--path` explicitly, covering only truly affected child projects or root paths. When scope is uncertain, clarify it before replacing or expanding a Rule.

## Project Profile and Durable Background

Use `profile show` to inspect the current project profile. Refresh it when authority sources, structure, or module relationships changed:

```bash
skills-seed profile show
skills-seed learn current --profile refresh
skills-seed generate skills
```

Maintain durable business background, terminology, and external-system facts that source cannot show in `.skills-seed/context/`. Use `sync --context` or `--context-path` for one-time guidance; do not accidentally convert it into durable Context, Rule, or Pattern.

## Content You Must Not Edit Directly

- `.skills-seed/runtime/`: diagnostics, checkpoints, Agent output, and generated manifests; changing it breaks recovery and audit evidence.
- The database and profile files under `.skills-seed/store/`: use the CLI to preserve consistency and provenance.
- Generated output directories such as `.agents/skills/` and `.claude/skills/`: `generate skills` rebuilds them completely.

For failure states, Agent output, or a recovery plan, continue to [Learning Failures and Manual Repair](Learning-Failures-and-Manual-Repair.md). See [Command Workbench](Command-Workbench.md) and the [command reference](../../COMMANDS.EN.md) for all command paths and flags.

---

**Next:** [Learning Failures and Manual Repair](Learning-Failures-and-Manual-Repair.md) - handle runtime archives, checkpoints, and non-recoverable state.

**Related:** [Rules and Workflows](Rules-and-Workflows.md) - understand the boundary between the two user resources; [Command Workbench](Command-Workbench.md) - find maintenance subcommands.

**Language:** [简体中文](../Knowledge-Maintenance.md)
