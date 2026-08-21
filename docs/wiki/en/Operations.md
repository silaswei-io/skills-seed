# Operations and Recovery

> **Use this page when:** sync stops, an Agent call fails, output is unexpected, or you need to decide between archives, resumption, and an explicit restart.

## First Identify the Failure Stage

| Symptom | Check first | Common response |
|---|---|---|
| Unexpected file scope | `preview files --mode incremental`, exclude configuration | Correct scope or Git-ignore policy |
| Agenda or source-analysis failure | Runtime inputs, rendered prompt, Agent-output archive | Correct Agent, model, Schema, or input boundary, then resume |
| Independent-review failure | Current focus candidates and evidence archive | Check whether output violates structure or evidence scope |
| Generation failure | Stored profile, rules, patterns, and templates | Correct data or template, then regenerate |
| Incompatible command state | Runtime and command-state diagnostic | Restart explicitly; do not silently clear state |

## Runtime Archives

`.skills-seed/runtime/` contains diagnostic artifacts and should not normally be edited directly. It commonly includes:

- Rendered prompts and manifests.
- Agent raw output, final content, stderr, and call manifests.
- Candidate files, focus inputs, recovery state, and checkpoints.
- Generated-Skills manifests with version, template, knowledge-snapshot, and file-hash metadata.
- Execution logs and error diagnostics.

The `raw`, `stderr`, and `manifest` paths in an error are first-hand evidence. Read their concrete upstream error before guessing from a top-level "Agent call failed" message.

## Resume or Restart

Learning persists recoverable state. After a normal interruption, recoverable Agent failure, or local save failure, prefer:

```bash
skills-seed sync --resume
```

Resumption should reuse completed filtering, agenda, and focus checkpoints. Restart only when state versions are incompatible, inputs are no longer trustworthy, or the user explicitly requests it. The CLI should report incompatibility rather than silently deleting all state.

If interruption happened after pattern admission, resumption can show `0` pending analysis files. That is normal: the CLI continues the source baseline, authority rules, and project-map refresh, while the final learning summary keeps the committed pattern counts and original change scope.

## Agent and Structured-Output Failures

Use this order:

1. Confirm the configured Agent CLI and model are usable in the environment.
2. Read HTTP, rate-limit, authentication, proxy, or Schema errors in raw output.
3. For retryable rate-limit or overload failures, wait for retry or adjust retry configuration.
4. For input or JSON Schema validation errors, fix the caller or output contract; retrying the same request is usually useless.
5. When switching Agents, keep analysis Agent and Skills output target distinct.

## Daily Inspection Commands

```bash
skills-seed preview files --mode incremental
skills-seed profile show
skills-seed patterns stats
skills-seed log
```

`skills-seed log` reads runtime records under `.skills-seed/runtime/journal/`; workspace roots merge child-project records before display.

See [Learning Failures and Manual Repair](Learning-Failures-and-Manual-Repair.md) for the full repair decision process, and the [command reference](../../COMMANDS.EN.md) and [configuration reference](../../CONFIGURATION.EN.md) for details.

---

**Next:** [Learning Failures and Manual Repair](Learning-Failures-and-Manual-Repair.md) - preserve evidence, classify a failure, then resume or explicitly restart.

**Related:** [Configuration and Reference](Reference.md) - find runtime directories and configuration fields; [Command Workbench](Command-Workbench.md) - choose a diagnostic command.

**Language:** [简体中文](../Operations.md)
