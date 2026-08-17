# Getting Started

This page sets up one project. For a multi-repository root, see [Workspace](Workspace.md).

> **Use this page when:** you are generating Skills for an existing Git project for the first time, or need to reconfirm the shortest first-setup path.

## Prerequisites

- Work inside a Git project, or a subdirectory from which its Git root is discoverable.
- Install `skills-seed`.
- Install an Agent CLI that can perform learning. The analysis Agent and the generated-Skills target may differ.

Download the archive for the current system and architecture from the [latest release](https://github.com/silaswei-io/skills-seed/releases/latest), extract `skills-seed` (`skills-seed.exe` on Windows), and place it on `PATH`. Then verify it:

```bash
skills-seed --version
```

To update an installed CLI later:

```bash
skills-seed update
```

The command downloads and verifies an official release, requires no local Go installation, and does not affect project `.skills-seed` state. Building from source is for contributors:

```bash
go build -o skills-seed ./cmd/skills-seed
./skills-seed --version
```

## Initialize a Project

Run this at the project root:

```bash
skills-seed init
```

Interactive initialization selects project mode, analysis Agent, Skills target, locale, parallelism, and base configuration. In automation, provide those choices explicitly:

```bash
skills-seed init --mode project --agent codex --skills codex --locale en-US --no-interactive
```

After initialization, `.skills-seed/` is the local working area for project knowledge. It holds configuration, user resources, learning state, runtime archives, and local data. Teams decide which collaborative files belong in version control.

## Choose an Analysis Agent and Output Target

`agent.engine` selects the Agent CLI that performs analysis and learning. `skills.target` selects the consumer for generated Skills. They may differ, for example when one Agent learns the project and another Agent consumes the generated Skills.

For routine sync, prefer a model with suitable speed and cost. Increase model capability only when the project is complex, evidence judgment is insufficient, or generated quality needs stronger reasoning. The [configuration reference](Reference.md) remains canonical for model names, output paths, and every field.

## Learn and Generate for the First Time

```bash
skills-seed sync
```

`sync` is the normal entry point: it learns current code first, then generates Skills after learning succeeds. The first run can inspect many files; later runs prefer incremental state.

The output target follows `skills.target`. Common entry paths are:

| Target | Default entry |
|---|---|
| Codex | `.agents/skills/<project>-dev/SKILL.md` |
| Claude Code | `.claude/skills/<project>-dev/SKILL.md` |

## Verify the Result

1. Check the terminal summary for successful learning and generation.
2. Open the generated `SKILL.md` and confirm that it routes work to project references, user Rules, and Workflows.
3. Use `skills-seed profile show`, `skills-seed patterns stats`, or `skills-seed log` to inspect learned state.
4. In a real change, verify that the target Agent loads the entry Skill and finds the correct module and constraints.

If the first run stops, do not delete `.skills-seed` first. Use [Operations and Recovery](Operations.md) to decide whether `sync --resume` is appropriate.

## Next Steps

- Read [Core Concepts](Concepts.md) to understand what learning stores.
- Read [Configuration and Reference](Reference.md) to control scope, model, parallelism, or output paths.
- Read [Rules and Workflows](Rules-and-Workflows.md) to record constraints that source code cannot prove.

---

**Next:** [Core Concepts](Concepts.md) - distinguish Rules, Workflows, Context, and source-learned knowledge.

**Related:** [Workspace](Workspace.md) - initialize a multi-project root; [Learning Failures and Manual Repair](Learning-Failures-and-Manual-Repair.md) - handle an interrupted first run.

**Language:** [简体中文](../Getting-Started.md)
