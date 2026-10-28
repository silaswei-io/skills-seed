# Learning and Sync

## Local scanning and focused semantic enrichment

Local CodeGraph structural scanning is the default. `codegraph.runtime: auto` prefers an installed external CodeGraph platform package and falls back to bundled 1.6.2; `external` and `embedded` force one source. Structural scanning uses no AI; important semantic gaps still trigger analysis and independent review. Source builds must run `make bundle` first. `SKILLS_SEED_CODEGRAPH_HOME` selects an official platform package and takes precedence over config. An incompatible shared `.codegraph` database schema stops the run before implicit migration.

```bash
skills-seed sync
skills-seed codegraph query "HTTP"
# Optionally request deeper analysis of a concrete scope:
skills-seed learn current --semantic --focus internal/order --context "Check order cancellation state boundaries"
skills-seed generate skills
```

Generated `references/code-navigation.md` links directories to source symbols and instructs development agents to search existing capabilities, inspect signatures and usage examples, then reuse, extend or justify a new component. Directories are structural navigation, not inferred business domains. `codegraph callers/callees/impact` take returned symbol IDs; `codegraph trace <from-id> <to-id>` finds static call paths. Dynamic and cross-service relationships may be incomplete; an empty result does not prove that no reusable capability exists.

The index lives in the standard `.codegraph/` directory and is shared with other CodeGraph tools. Skills-seed adds only its navigation snapshot and lock file; narrowing a scan does not recreate the shared database. Source selection follows repository exclusions and excludes generated Skills and caches. The bundled index corrects Go methods that the SDK leaves unmarked as exported so they appear in capability navigation; called methods without independent cross-file behavior are primarily evidence for their callers. Files not indexed by the SDK are listed explicitly. Extraction failures return errors rather than claiming success.

The local scan builds structural navigation for every selected file and records changed files without an automatic semantic gap as `selection_skipped`, so the next sync does not reassess them. This does not claim their business semantics were extracted. Only successfully analyzed, reviewed, and stored candidates are marked semantically confirmed. Unfinished candidates remain pending, and explicit `--semantic --focus` can revisit files already scanned. Generation omits old Patterns whose source evidence fingerprints changed, retaining the original knowledge in storage. Project navigation is generated deterministically from the local index and saved knowledge. Rule/Workflow maintenance is unchanged. Consuming agents must read applicable authority files directly.

Workspace routing comes from explicit configuration without AI inference of cross-project semantics. Children use the same automatic supplementation policy. Run `--semantic --focus` inside a child for deeper analysis. `--context` is background for the current supplement; `--focus` limits the local scan scope. `--force` rechecks the index and requests output refresh.

Automatic selection first revisits changed evidence of accepted business knowledge. It then follows CodeGraph's actual symbol calls to find cross-file behavior entries, routes, and public capabilities; imports and interface declarations alone do not trigger AI. Private same-file callees are included as entry evidence rather than separate semantic questions. The local graph covers downstream candidates along a real call chain from a public entry or route. Independent downstream capabilities, entries without a reliable upstream, call cycles, and changed evidence of accepted knowledge retain their own questions. Only when multiple evidence batches remain does a compact AI pre-screen use structural facts to exclude clearly low-value candidates in parallel. Screening failures and uncertain items retain source analysis. Files kept only in local navigation have `selection_skipped` status, not semantic confirmation. Explicit `--semantic --focus` can inspect dynamic calls or isolated implementations more deeply.

Candidates are batched dynamically by evidence capacity; input contains complete relevant symbols, call paths, and existing knowledge anchors instead of repeatedly sending whole files. Ordinary evidence bundles are capped at 64 KiB; single-file evidence reserves prompt space within the 192 KiB complete-input safety budget. Reviews of existing knowledge prioritize its source anchors. Candidates with insufficient or oversized evidence remain pending. Deterministic capacity blocks are cached by source fingerprint so unchanged input is not reassessed on every sync; a source or evidence-policy change retries automatically, and an explicit focus can retry as well. Pre-screening and independent evidence batches run concurrently according to `agent.parallelism`: a positive value is explicit, while `0` uses local resources with a cap of six concurrent Agent calls. Analysis and independent review share that budget, while one collector updates checkpoints, progress, and pending state. Normal sync processes all actionable candidates without a project-level deadline; individual Agent calls retain timeout, retry, and input-size protection. A failed batch stays pending while independent batches continue. `.skills-seed/runtime/local-supplement.json` reports structural coverage, screened-out counts, semantic candidates, pending work, and capacity blocks separately. Incremental detection persists file fingerprints without full source snapshots. Deleted source evidence suppresses old knowledge during generation.

Initial learning prioritizes route and cross-file behavior entries. Incremental learning also evaluates changed callee implementations so updates within a known call path are not missed.

> **Use this page when:** you need to refresh generated Skills, understand learning stages, or choose between learning only, generation only, resumption, and restart.

## Choose the Right Command

| Goal | Command | Generates Skills |
|---|---|---|
| Refresh project knowledge and output in daily work | `skills-seed sync` | Yes, after successful learning when output needs refresh |
| Learn only, without changing generated output | `skills-seed learn current` | No |
| Render output from existing learned data | `skills-seed generate skills` | Yes |
| Preview files that would be learned | `skills-seed preview files --mode incremental` | No |

See the complete [command reference](../../COMMANDS.EN.md) for flags.

Selection and prompts have no project-name, framework, or business-directory exceptions. Put project facts and terminology in `.skills-seed/context/` and temporary priorities in `--context`; both reach the same supplement request without replacing source evidence or user-maintained Rules/Workflows. Only independently reviewed business patterns enter the business map; directory names cannot become business responsibilities by inference. Without meaningful profile data, no placeholder overview is emitted.

## Current-Code Learning Pipeline

`learn current` and `sync` follow these main stages:

```text
Build or update the local CodeGraph index
  -> Select business-semantic gaps locally from changes, calls, and saved knowledge
  -> Screen clearly low-value items from compact structural facts when multiple batches are needed
  -> Batch the rest dynamically by evidence byte capacity
  -> Call AI only when local evidence cannot decide the business rule, then review independently
  -> Admit and persist durable knowledge
  -> Deterministically generate navigation and Skills
```

Local code validates path scope, input coverage, structured contracts, source safety, and persistence. The Agent assesses meaning, boundaries, and reusable value from supplied evidence. This division avoids hardcoding business judgment into file filters.

A project `learn current` or `sync` run shows one pass through six top-level stages: CodeGraph indexing, local file filtering, semantic candidate planning, structural coverage commit, batched semantic supplementation, and learning-state save. Analysis, independent review, and admission for each dynamic evidence batch appear as details under the fifth stage, with progress counted by processed candidate files. Failed batches remain pending while independent batches continue. Each workspace child reports stages in the same order. While waiting, animation and elapsed time remain visible; only changed rows are redrawn, without repeatedly clearing the entire panel. Redirected output continues to use line-by-line logs.

## Evidence Batches and Review

- Each evidence batch contains locally selected questions and relevant call paths, without presuming a business domain or output count.
- Checkpoints retain the `EvidenceFocus` field to identify a resumable evidence group. Default learning no longer asks AI to plan focuses, and this field does not define the final Skill taxonomy. One batch can yield several distinct business patterns, each admitted by independent review.
- `EntryPaths` own that evidence group's exclusive learning responsibility. `RelatedPaths` are shared read-only evidence and do not count toward coverage. Both are restricted to this run's input files.
- Default supplementation analyzes and independently reviews evidence batches until normal candidates are complete. Public entry points are semantic candidates whether or not they have known callers; evidence and review decide which ones are retained. CodeGraph supplies symbol identity, location, signature and call facts; AI adds task applicability, prerequisites and result semantics, and reviewed entries must still match a real symbol in this run's graph. Workspace children share the Agent concurrency budget and do not impose a project-level learning deadline.
- The actual Claude and Codex runtime Schemas constrain focus IDs and result counts to the current batch. If a delta candidate fails local evidence admission and its focus has no other admitted decision, exactly one `no_change` receipt remains instead of misclassifying “no admissible Pattern” as an omitted focus.
- Even after CLI success, dynamic JSON Schema and business-parser validation still apply. Invalid structure gets at most one repair using the previous output and concrete validation error, without repeating source discovery. Transient service failures use `agent.retry` backoff. Repair also counts against the total retry limit. `agent.timeout` covers the entire stage, including queueing, execution, backoff, and repair; retries do not reset it.
- Independent knowledge review operates on a complete focus, rather than mechanically splitting by candidate count.
- Candidates and focuses are persisted once the local evidence batch is selected. Source-analysis success checkpoints candidates and program-supplied evidence before review starts; review success saves a separate completion state. Review uses an independent session with current source paths and structural facts, without inheriting analysis reasoning. After review failure, `--resume` retries only review. Unreviewed candidates and retirement decisions cannot enter the store. Failed batches remain pending; previously committed batches are preserved.
- `--resume` reuses valid inputs and agendas. Changed source hashes, changes outside the saved scope, or explicit restart invalidate them. Checkpoints now use schema 5; old checkpoints are not migrated and require `sync --restart`. With a valid format, a CLI version change alone does not trigger replanning.
- Normalization sends only candidates with related knowledge to the Agent; isolated candidates are retained unchanged. Single-source IDs, wording, and confidence come from the reviewed result. Only actual multi-source merges may synthesize text. Decision checkpoints bind candidates, related existing knowledge, and user-maintained context to prevent stale replay.
- Resumption revalidates saved Pattern-normalization decisions. Each current source candidate first rebuilds source identity from its own Pattern ID; historical `MergedFrom` values remain lineage of an existing output and cannot flow back into current input ownership. A current candidate with the same ID then replaces the stored version of that Pattern. During repeated validation of an already hydrated result, unambiguous lineage only resolves to its effective Pattern, while lineage with multiple owners remains rejected. If a decision actually merges capability entries from different IDs, the CLI retains separate candidates and replaces that checkpoint instead of admitting the merge or interrupting persistence.
- Pattern admission and the source baseline commit in phases. If interruption happens after patterns were stored, resumption restores the committed candidate-found and normalized-written counts plus the original change scope, so `sync` still decides Skills generation from the completed learning result.
- Learning summaries distinguish “candidate patterns found” from “normalized patterns written”: the former counts candidates remaining after review, while the latter counts final upserts including related existing Patterns and can therefore be larger.
- Candidates explicitly dropped during normalization are recorded in the run journal and changelog with a `reason_code` and short reason, so reviewers can see why they were not stored.
- Workspace `sync` distinguishes child outcomes as learned changes, no learned changes with generation skipped, or no learned changes with missing Skills restored. After all children finish, it reports changed-project, file-change, candidate, normalized-write, and retired totals. An unchanged workspace relationship input only means cross-project profile and rules analysis did not need to run; it does not mean child projects learned nothing.

More parallelism does not reduce call count. The default path combines related evidence and reuses valid checkpoints rather than splitting requests by candidate count.

## Success and Failure Boundaries

When files remain pending assessment, sync reports the unfinished count even if a partial Skill was generated successfully. Pending files are not a count of proven business problems.

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
