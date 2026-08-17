# Skills and Resources

> **Use this page when:** you need to understand the generated layout, review an entry Skill, or distinguish references, rules, and workflows.

## Responsibility of Generated Output

A generated Skill is not one oversized project document. The entry file keeps task judgment, precedence, and routing; detailed knowledge is split by responsibility and read only when necessary.

Typical layout:

```text
<project>-dev/
├── SKILL.md
├── agents/
├── references/
│   ├── project-overview.md
│   ├── project-spec.md
│   ├── modules.md
│   ├── business-methods.md
│   └── patterns/
├── rules/
└── workflows/
```

The actual layout changes with project mode, target Agent, learned knowledge, and user resources.

## What the Entry Skill Should Answer

`SKILL.md` must help an Agent decide before editing:

1. Which project, module, or development focus owns the task?
2. Does an explicit Rule or Workflow constrain the task first?
3. Which project references, capability entries, and source locations must be read?
4. When several focuses match, which references must be read together and how should path and evidence resolve ambiguity?
5. Which impact boundaries or unknowns must be confirmed before implementation?

It must not make "current code wins" a blanket rule that overrides explicit user authority, nor treat a technical service name or incidental keyword as a stable business route.

## References, Rules, and Workflows

| Directory | Content | Maintained by |
|---|---|---|
| `references/` | Project profile, module map, capability entries, source patterns | Generated from learning |
| `rules/` | Projection of durable user constraints | Generated from `.skills-seed/rules/` |
| `workflows/` | User-maintained task procedures and related scripts | Generated from `.skills-seed/workflows/` |

An Agent uses references to understand current implementation, rules to understand non-negotiable boundaries, and workflows to execute team-required procedures. They are not substitutes for one another.

## Reviewing Generated Quality

Review against real development decisions:

- Does the entry trigger for implementation, diagnosis, contracts, configuration, boundaries, and verification work?
- Do mandatory constraints come from the correct authority and remain above source observations?
- Do development focuses have natural names, stable route terms, and multi-match disambiguation where needed?
- Do capability entries include a declaration, prerequisites, result semantics, and locatable source?
- Does the module map describe real responsibility and dependencies rather than self-dependencies or guessed paths?
- Does output contain only valid projections with working plans and links?

See [Quality and Contributing](Quality-and-Contributing.md) for quality testing and maintenance.

---

**Next:** [Rules and Workflows](Rules-and-Workflows.md) - add explicit constraints and task procedures to a generated Skill.

**Related:** [Knowledge Maintenance](Knowledge-Maintenance.md) - correct persisted knowledge; [Quality and Contributing](Quality-and-Contributing.md) - judge delivery value through real changes.

**Language:** [简体中文](../Skills-and-Resources.md)
