# Configuration and Reference

This Wiki explains the operating model. The documents below are canonical for flags, fields, templates, and defaults.

> **Use this page when:** you know what to do but need the exact command, flag, field, default, or maintainer reference.

## User References

| Topic | Document | Read when |
|---|---|---|
| Commands, subcommands, flags, and examples | [Command Reference](../../COMMANDS.EN.md) | Automating, finding a flag, or diagnosing command behavior |
| Choosing a command and subcommand by outcome | [Command Workbench](Command-Workbench.md) | Before initialization, learning, repair, recovery, or reset |
| Configuration structure, defaults, directories, and runtime debugging | [Configuration Reference](../../CONFIGURATION.EN.md) | Adjusting Agent, parallelism, filters, output, or logging |
| Maintaining Patterns, Rules, Workflows, and the profile | [Knowledge Maintenance](Knowledge-Maintenance.md) | Persisted knowledge needs addition, revision, deletion, or consolidation |
| Learning failures, resumption, restart, and runtime archives | [Learning Failures and Manual Repair](Learning-Failures-and-Manual-Repair.md) | Failure diagnosis or quality correction |
| Real project result | [Medusa Demo Case Study](../../MEDUSA_DEMO_CASE.EN.md) | Evaluating generated output and adoption |
| Version changes | [Changelog](../../../CHANGELOG.en.md) | Upgrading, regressing, or comparing behavior |

## Maintainer References

| Topic | Document | Purpose |
|---|---|---|
| Product acceptance boundary | [Ultimate Goal](../../ULTIMATE_GOAL.md) | Final basis for prompt, data-flow, and template changes |
| Prompt inventory and redundancy audit | [Prompt Documentation](../../PROMPTS.md) | Check runtime prompt ownership and duplication |
| Contribution and local checks | [Contributing](../../../CONTRIBUTING.en.md) | Code, test, and contribution conventions |

## Configuration Decision Map

| Need | Main setting or command |
|---|---|
| Use a particular analysis Agent or model | `agent.engine`, `agent.model` |
| Adjust analysis throughput | `agent.parallelism` |
| Exclude paths from learning | `exclude.paths`, `exclude.gitignore` |
| Change Skills target or output location | `skills.target`, `skills.paths.<target>` |
| Preview or learn only | `preview files`, `learn current` |
| Re-render explicitly | `generate skills` |

Field names, accepted values, and full behavior are defined by the [configuration reference](../../CONFIGURATION.EN.md).

---

**Next:** [Quality and Contributing](Quality-and-Contributing.md) - review changes against the ultimate goal and quality gates.

**Related:** [Command Workbench](Command-Workbench.md) - choose a command by outcome first; [Knowledge Maintenance](Knowledge-Maintenance.md) - maintain persisted knowledge.

**Language:** [简体中文](../Reference.md)
