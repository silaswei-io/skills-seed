# Rules and Workflows

Rules and Workflows cover information source learning cannot reliably derive: mandatory team boundaries and explicit procedures for testing, release, acceptance, and related work.

> **Use this page when:** you need to save durable constraints, maintain a team procedure, or give one-time guidance that should not persist.

## Context: Background and One-Time Guidance

Use `.skills-seed/context/` for project background, terminology, and durable facts that source cannot show. Put long-lived mandatory constraints in Rules rather than hiding them in Context.

Use `sync --context` or `sync --context-path` for guidance that affects only the current learning task. It is not written into durable Context and does not become a permanent generated-Skill rule. The [configuration reference](Reference.md) remains canonical for Context layout and input-processing details.

## Rules: Durable Constraints

Use a Rule for:

- Editable and non-editable scope.
- API, compatibility, security, and data constraints.
- Command permission and operations that require human confirmation.
- Project terminology, base-code boundaries, or long-lived governance requirements.

Add or incrementally refine a Rule:

```bash
skills-seed rule --name <rule-name> --content "<team constraint>"
skills-seed generate skills
```

A same-name Rule is refined from existing content by default. Only explicit `--overwrite` replaces it completely. Its source of truth is `.skills-seed/rules/<id>/RULE.md`.

## Workflows: Task Procedures

Use a Workflow for test, deployment, release, acceptance, escalation, or handoff steps, commands, ordering, and approval requirements:

```bash
skills-seed workflow --name <workflow-name> --content "<task procedure>"
skills-seed generate skills
```

Source can describe implementation dependencies and impact boundaries, but it cannot reliably determine which commands must run, who approves them, or when deployment occurs. Those are user-maintained Workflow concerns.

## Workflow Path Base

Use repository-relative paths for affected locations, verification targets, and script references in workflow content. A workspace-level workflow is relative to the workspace root; a project workflow, including a `--child` workflow, is relative to the target project root. Do not use paths relative to the current terminal directory or absolute local-machine paths.

## Workflow-Associated Scripts

Each workflow uses its own directory for the procedure and its scripts:

```text
.skills-seed/workflows/<workflow-id>/
├── WORKFLOW.md
└── scripts/
    └── <workflow-specific-script>
```

When a workflow needs a script, place it only in that workflow's `scripts/` directory. Confirm the stable ID with `skills-seed workflow show <workflow-id>` first. After `skills-seed generate skills`, the procedure is written to `workflows/<workflow-id>.md`, its scripts are copied to `scripts/workflows/<workflow-id>/`, and the rendered workflow references them. Do not write scripts directly into generated output.

## Command Policy

Generated Skills should distinguish these policies:

| Policy | Meaning |
|---|---|
| `forbidden` | Must not execute, even when requested |
| `describe_only` | May explain, but must not execute |
| `requires_authorization` | Needs explicit authorization before execution |
| `allowed` | May execute within the current task scope |

An automation, CI, or build file alone does not grant command permission. Only an explicit Rule or user instruction defines that boundary.

## Check After Changes

1. Inspect stored content through `skills-seed rule` or `workflow`.
2. Run `skills-seed generate skills` to refresh projections.
3. Open the generated Skill's `rules/` or `workflows/` and verify content and entry links.
4. For security, deployment, or compatibility Rules, verify behavior in a real task.

See [Knowledge Maintenance](Knowledge-Maintenance.md) for the full maintenance loop, [Command Workbench](Command-Workbench.md) for command selection, and the [command reference](../../COMMANDS.EN.md) for exact parameters.

---

**Next:** [Knowledge Maintenance](Knowledge-Maintenance.md) - inspect, revise, and regenerate Rule and Workflow projections.

**Related:** [Workspace](Workspace.md) - define explicit root-Rule impact scope; [Command Workbench](Command-Workbench.md) - find the relevant subcommand.

**Language:** [简体中文](../Rules-and-Workflows.md)
