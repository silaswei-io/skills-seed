# Workspace

Workspace mode is for a root directory containing multiple independent Git projects. It preserves ownership rather than merging all code into one project: children learn their own source, while the root routes work, captures shared constraints, and describes cross-project impact.

> **Use this page when:** your root contains independent Git child projects and requests need ownership or cross-project impact judgment first.

## When to Use It

Use it when:

- The root contains several independently developed, tested, or released Git projects.
- A request can affect multiple projects and needs ownership and change-order judgment first.
- Each child should retain independent `.skills-seed` data and final Skills.

Do not use it merely because one Git project has many directories. Use project mode and route through module references in that case.

## Initialize and Add Projects

Initialize at the workspace root, then register child projects:

```bash
skills-seed init
skills-seed workspace add <child-path>
skills-seed sync
```

Interactive behavior and flags are defined in the [command reference](../../COMMANDS.EN.md).

Use `skills-seed init --workspace --skills-name team-guide` to name the root Skill. The name applies only to the root; children retain their own default or custom names, and root routing reads their configuration. To customize a child name, run `skills-seed init --skills-name backend-guide` inside that child before initializing the workspace or adding the child. Existing child configurations are preserved.

## Responsibility Split

| Location | Owns |
|---|---|
| Workspace root | Child-project routing, cross-project relations, root Rules, and impact descriptions |
| Child project | Its own source learning, patterns, profile, and generated Skill |
| Root Rule | Must explicitly affect several children or a workspace-root path |
| Child Rule | Belongs to that child and must not be incorrectly promoted to the root |

A root Rule does not automatically become every child's Rule. Resource ownership must be explicitly declared; the system must not infer and expand scope from natural language.

## Runtime Strategy

- Root `agent.parallelism` caps both child-project concurrency and total Agent calls shared across children.
- Each child config controls focus scheduling without enlarging the root budget. Source analysis and review share slots; knowledge-review calls remain serial across children.
- Cross-project work routes from the root Skill to affected child Skills and source.
- Resumption should reuse plans and checkpoints instead of rescanning unchanged children.

See [Configuration and Reference](Reference.md) and [Operations and Recovery](Operations.md) for configuration and root-state details.

---

**Next:** [Operations and Recovery](Operations.md) - inspect runtime archives, state, and daily diagnostic entry points.

**Related:** [Rules and Workflows](Rules-and-Workflows.md) - define child-project or root-path scope; [Command Workbench](Command-Workbench.md) - choose a workspace subcommand.

**Language:** [简体中文](../Workspace.md)
