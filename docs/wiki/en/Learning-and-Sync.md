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
  -> Local filtering and incremental candidates (candidates enter the agenda directly; no separate selection step)
  -> Learning-agenda planning
     AI returns only high-value source focuses; unassigned paths are split into a small number of resumable coverage-safety batches by `learning.current.agenda.fallback_paths_per_focus`, not treated as low-value by omission
  -> Isolated source-evidence analysis and independent knowledge review (full/delta share one Mode selector for input materials)
  -> Knowledge admission and persistence
  -> Authority and project-map refresh
  -> Skills generation
```

Local code validates path scope, input coverage, structured contracts, source safety, and persistence. The Agent assesses meaning, boundaries, and reusable value from supplied evidence. This division avoids hardcoding business judgment into file filters.

## Focuses, Parallelism, and Review

- Every focus centers on an evidence-backed responsibility or behavior boundary.
- A focus is a resumable analysis batch, not a final Skill taxonomy. Merge neighboring responsibilities that share a future change boundary; split only clearly independent or high-risk boundaries.
- `EntryPaths` own a focus's exclusive learning responsibility. `RelatedPaths` are shared read-only evidence and do not count toward coverage. Both are restricted to this run's input files.
- Source analysis and knowledge review form a pipeline sharing the `agent.parallelism` limit; knowledge review is serial. The workspace root also caps total Agent calls across children, so child parallelism cannot multiply the shared budget.
- The actual Claude and Codex runtime Schemas constrain focus IDs and result counts to the current batch. If a delta candidate fails local evidence admission and its focus has no other admitted decision, exactly one `no_change` receipt remains instead of misclassifying “no admissible Pattern” as an omitted focus.
- Even after CLI success, dynamic JSON Schema and business-parser validation still apply. Invalid structure gets at most one repair using the previous output and concrete validation error, without repeating source discovery. Transient service failures use `agent.retry` backoff. Repair also counts against the total retry limit. `agent.timeout` covers the entire stage, including queueing, execution, backoff, and repair; retries do not reset it.
- Independent knowledge review operates on a complete focus, rather than mechanically splitting by candidate count.
- Candidates and the agenda are persisted immediately after planning. Source-analysis success checkpoints candidates and program-supplied evidence before review starts; review success saves a separate completion state. Review uses an independent session with the saved source paths, diff references, and structural facts, without inheriting analysis reasoning. After review failure, `--resume` retries only review. Unreviewed candidates and retirement decisions cannot enter the store. If one focus fails, other in-flight analyses finish and save successful results.
- `--resume` reuses valid inputs and agendas. Changed source hashes, changes outside the saved scope, or explicit restart invalidate them. Checkpoints now use schema 5; old checkpoints are not migrated and require `sync --restart`. With a valid format, a CLI version change alone does not trigger replanning.
- Normalization sends only candidates with related knowledge to the Agent; isolated candidates are retained unchanged. Single-source IDs, wording, and confidence come from the reviewed result. Only actual multi-source merges may synthesize text. Decision checkpoints bind candidates, related existing knowledge, and user-maintained context to prevent stale replay.
- Resumption revalidates saved Pattern-normalization decisions. Each current source candidate first rebuilds source identity from its own Pattern ID; historical `MergedFrom` values remain lineage of an existing output and cannot flow back into current input ownership. A current candidate with the same ID then replaces the stored version of that Pattern. During repeated validation of an already hydrated result, unambiguous lineage only resolves to its effective Pattern, while lineage with multiple owners remains rejected. If a decision actually merges capability entries from different IDs, the CLI retains separate candidates and replaces that checkpoint instead of admitting the merge or interrupting persistence.
- Pattern admission, the source baseline, and project-map refresh commit in phases. If interruption happens after patterns were stored, resumption restores the committed candidate-found and normalized-written counts plus the original change scope, so `sync` still decides Skills generation from the completed learning result.
- Learning summaries distinguish “candidate patterns found” from “normalized patterns written”: the former counts candidates remaining after review, while the latter counts final upserts including related existing Patterns and can therefore be larger.
- Workspace `sync` distinguishes child outcomes as learned changes, no learned changes with generation skipped, or no learned changes with missing Skills restored. After all children finish, it reports changed-project, file-change, candidate, normalized-write, and retired totals. An unchanged workspace relationship input only means cross-project profile and rules analysis did not need to run; it does not mean child projects learned nothing.

Parallelism improves throughput but increases Agent pressure. Size it for provider limits and repository scale rather than maximizing the value.

## Success and Failure Boundaries

A successful learning run does not prove all project code correct. It means the admitted knowledge passed source, scope, evidence, and structural checks. Generation consumes persisted results and does not make another Agent call.

Generation also runs a delivery-readiness check: local inline links, reference links, and images recognized by standard Markdown syntax must resolve to files inside the generated directory. Inline code, fenced code, source index expressions, Go generic calls, and other non-Markdown links are ignored. When this check fails, inspect the concrete generated file and archived output first.

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
