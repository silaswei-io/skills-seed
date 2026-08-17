# Learning Failures and Manual Repair

> **Use this page when:** learning reports an error, stops, resumes unexpectedly, or generated-Skill knowledge needs human correction. Preserve evidence, classify the problem, then choose resumption, source-of-truth correction, or an explicit restart.

## Decision Path

```text
An error or quality issue appears
  -> File scope is wrong? Start with preview files
  -> Invocation or structural validation failed? Read runtime archives
  -> Persisted knowledge is inaccurate? Correct its source with patterns / rules / workflows
  -> Completed checkpoints remain trustworthy? sync --resume
  -> State is incompatible or input is distorted? sync --restart
  -> Initialization as a whole is unusable? Confirm backup, then reset
```

## File Scope, Inputs, and Focuses

Before learning, or after discovering an omission, verify the files the tool actually sees instead of asking the Agent to guess:

```bash
skills-seed preview files --mode full
skills-seed preview files --mode incremental
skills-seed preview files --mode incremental --focus <path>
```

Check `exclude.paths`, `exclude.gitignore`, focus paths, and the Git working tree. Preview again after correcting the scope; start `learn current` or `sync` only when it is correct. Do not use unrelated Context to compensate for source that should have entered analysis.

## Agent, Network, and Structured-Output Failures

An error's `raw`, `stderr`, and `manifest` paths identify concrete archives beneath `.skills-seed/runtime/agent-outputs/`. Use this order:

1. Read the original error and classify authentication, rate limiting, proxy, service overload, CLI invocation, or structured-output failure.
2. Confirm that the configured Agent CLI, model, and network/proxy are usable locally.
3. For recoverable rate-limit or temporary-service errors, keep checkpoints and run `sync --resume`.
4. For JSON Schema, input-coverage, or source-scope validation, first correct the invocation, prompt, configuration, or version incompatibility. Repeating unchanged input is usually ineffective.
5. After correction, run `sync --resume` and verify that completed focuses were reused.

Runtime archives are diagnostic evidence, not a manual knowledge-editing surface. When fixing prompts or structured contracts in Skills Seed itself, retain the original archive as a regression case.

## Poor Persisted Pattern Quality

Learning success does not mean every pattern belongs in durable knowledge. These cases do not require rerunning all learning:

| Problem | Manual correction |
|---|---|
| A pattern lacks an important user fact | `patterns update <id>`, or add a Rule / Workflow |
| A pattern describes an implementation as a guarantee | `patterns update <id>` to narrow it to evidence and leave unknown boundaries open |
| A pattern is entirely wrong or obsolete | `patterns delete <id>` |
| Several patterns overlap | Run `patterns compact --dry-run`, confirm, then compact |
| A Rule or Workflow projection is missing | Inspect with `rule show` / `workflow show`, then `generate skills` |

Always run `skills-seed generate skills` after correcting a pattern, Rule, or Workflow. See [Knowledge Maintenance](Knowledge-Maintenance.md) for the detailed maintenance loop.

## Resume, Restart, and Reset

| Situation | Action | Why |
|---|---|---|
| Normal interruption, temporary Agent failure, or post-save local failure | `skills-seed sync --resume` | Reuses valid checkpoints and avoids duplicate calls |
| Command-state version is incompatible, or candidate files/context are untrustworthy | `skills-seed sync --restart` | Explicitly clears only this run's recovery state before analysis |
| Initialization state as a whole is unusable or mode must change | `skills-seed reset ...` | Backs up old state before reinitialization |

Do not treat `--restart` as automatic retry, and do not use `reset` for one badly worded pattern. Both discard reusable work; the former only affects the current sync plan, while the latter backs up and rebuilds all initialization state.

## A Problem Remains After Generation

When generation fails or output is unexpected, trace back through sources:

1. Verify that `profile show`, `patterns show`, `rule show`, and `workflow show` contain the correct facts.
2. Verify that the entry, references, rules, and workflows in the output correspond to those facts.
3. When inputs are correct but the projection is wrong, repair the generator template or implementation, then run `generate skills`.
4. Editing generated output is not a repair because complete generation overwrites it.

See [Command Workbench](Command-Workbench.md) for command selection, and [Operations and Recovery](Operations.md) for runtime structure and configuration locations.

---

**Next:** [Knowledge Maintenance](Knowledge-Maintenance.md) - correct persisted Patterns, Rules, Workflows, or the project profile.

**Related:** [Operations and Recovery](Operations.md) - inspect runtime archives; [Command Workbench](Command-Workbench.md) - choose `resume`, `restart`, or `reset`.

**Language:** [简体中文](../Learning-Failures-and-Manual-Repair.md)
