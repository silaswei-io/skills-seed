# Learning and Sync

> **Use this page when:** you need to refresh generated Skills, understand learning stages, or choose between learning only, generation only, resumption, and restart.

## Choose the Right Command

| Goal | Command | Generates Skills |
|---|---|---|
| Refresh project knowledge and output in daily work | `skills-seed sync` | Yes, after successful learning when output needs refresh |
| Learn only, without changing generated output | `skills-seed learn current` | No |
| Render output from existing learned data | `skills-seed generate skills` | Yes |
| Preview files that would be learned | `skills-seed preview files --mode incremental` | No |

See the complete [command reference](../../COMMANDS.EN.md) for flags.

## Current-Code Learning Pipeline

`learn current` and `sync` follow these main stages:

```text
Prepare project context
  -> Local filtering and incremental candidates
  -> Learning-agenda planning
  -> Isolated source-evidence analysis
  -> Independent knowledge review
  -> Knowledge admission and persistence
  -> Authority and project-map refresh
  -> Skills generation
```

Local code validates path scope, input coverage, structured contracts, source safety, and persistence. The Agent assesses meaning, boundaries, and reusable value from supplied evidence. This division avoids hardcoding business judgment into file filters.

## Focuses, Parallelism, and Review

- Every focus centers on an evidence-backed responsibility or behavior boundary.
- Independent source analysis can run in parallel, controlled by `agent.parallelism`.
- Independent knowledge review operates on a complete focus, rather than mechanically splitting by candidate count.
- Finished focuses save recoverable state; cross-focus normalization and persistence happen afterward.

Parallelism improves throughput but increases Agent pressure. Size it for provider limits and repository scale rather than maximizing the value.

## Success and Failure Boundaries

A successful learning run does not prove all project code correct. It means the admitted knowledge passed source, scope, evidence, and structural checks. Generation consumes persisted results and does not make another Agent call.

On Agent failures, Schema incompatibility, or input-coverage failures, partial conclusions must not be written directly into the pattern store. Inspect runtime evidence and recovery guidance in [Learning Failures and Manual Repair](Learning-Failures-and-Manual-Repair.md).

## When to Regenerate Explicitly

Use `skills-seed generate skills` after:

- Adding, updating, or removing a Rule or Workflow.
- Manually maintaining patterns or the project profile.
- Changing the Skills target or output path.
- Verifying delivery content after a template change.

Generation rebuilds the Skills Seed-managed output directory. Move durable manual changes back into their `.skills-seed` source instead of editing generated output.

---

**Next:** [Command Workbench](Command-Workbench.md) - choose learning, generation, inspection, or recovery commands by outcome.

**Related:** [Skills and Resources](Skills-and-Resources.md) - review the generated delivery; [Learning Failures and Manual Repair](Learning-Failures-and-Manual-Repair.md) - handle failure, resumption, or quality issues.

**Language:** [简体中文](../Learning-and-Sync.md)
