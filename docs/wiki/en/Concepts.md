# Core Concepts

Skills Seed stores information with different confidence levels, owners, and purposes separately. Keeping those distinctions is necessary for reliable generated Skills.

> **Use this page when:** you need to decide whether information belongs in a Rule, Workflow, Context, or source learning and review.

## Information Types

| Type | Source and owner | Purpose | Must not do |
|---|---|---|---|
| Rule | Explicitly maintained by users or teams | Defines mandatory constraints, scope, and command permission | Must not be expanded or deleted from source scans |
| Workflow | Explicitly maintained by users or teams | Defines test, deployment, acceptance, and similar procedures | Must not be inferred from source code |
| Context | User-maintained background, terminology, or one-time guidance | Supplements intent that code cannot express | Must not replace verifiable rules |
| Project profile | Observation of current structure, modules, and relations | Helps an Agent navigate | Must not be phrased as a hard rule |
| Pattern | Reusable practice with source evidence | Helps reuse established implementation knowledge | Must not promote incidental code into a standard |
| Capability entry | Evidence-backed service, method, or utility | Avoids rebuilding existing behavior | Must not be detached from its declaration and source chain |

Rule and Workflow `summary` and `route_terms` are navigation metadata. They may narrow entry-Skill retrieval, but cannot expand a Rule's project/path scope or turn a Workflow into command authorization.

## Authority Order

An Agent resolves conflicts in this order:

1. The user's explicit instruction for the current task.
2. Project-maintained Rules and authoritative project files.
3. User-maintained Workflows.
4. Current source and source-backed learned knowledge.
5. General engineering practice.

Current code proves an existing implementation, but it does not automatically authorize violating an explicit Rule. For example, the existence of a build script does not authorize an Agent to run, deploy, or modify it.

## Evidence Focuses and Patterns

Learning does not mechanically turn directory names or filenames into business domains. It creates an evidence focus for the current candidates: the smallest group of evidence that must be read together to assess responsibility, contracts, state, integration, or cross-cutting boundaries.

A focus is only a unit of current evidence work, not a permanent workflow category. Only reviewed, source-backed patterns enter durable knowledge. Conclusions without evidence, clear applicability, or safe routing value are rejected or downgraded.

## Source of Truth and Projections

User-maintained resources live under `.skills-seed/`:

```text
.skills-seed/
├── context/
├── rules/<id>/RULE.md
├── workflows/<id>/WORKFLOW.md
└── runtime/
```

The final Skill's `rules/` and `workflows/` directories are deterministic Agent-facing projections. Do not edit generated output to maintain a durable Rule; `generate skills` rebuilds it.

## Lifecycle

Source-learned knowledge is not permanent fact. It can become stronger with new evidence, change after an implementation replacement, or be withdrawn, stale, or downgraded after source deletion. Rules and Workflows remain under explicit user control.

Continue with [Learning and Sync](Learning-and-Sync.md) to see how these concepts enter final Skills.

---

**Next:** [Learning and Sync](Learning-and-Sync.md) - see how these knowledge types become part of a final Skill.

**Related:** [Rules and Workflows](Rules-and-Workflows.md) - put team constraints and procedures in the right place.

**Language:** [简体中文](../Concepts.md)
