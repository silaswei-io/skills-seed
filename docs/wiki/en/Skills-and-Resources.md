# Skills and Resources

By default, `sync` / `learn current` scan locally with CodeGraph (external first, bundled fallback), then select important semantic gaps using changed knowledge evidence and local call relationships. Candidates are dynamically batched by priority and evidence capacity, with independent review and checkpoints. When AI is unavailable, navigation remains usable and pending gaps are recorded. Use `learn current --semantic --focus <path>` for explicit deeper analysis. See [Learning and Sync](Learning-and-Sync.md).

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
│   ├── business-methods/        # One detail per capability entry
│   ├── code-navigation.md       # CodeGraph query entry
│   └── patterns/
│       ├── business.md          # Business rule index
│       ├── business/groups/      # Responsibility-theme indexes from reviewed titles
│       └── business/items/      # One detail per business rule
├── rules/
└── workflows/
```

The actual layout changes with project mode, target Agent, learned knowledge, and user resources.

The business map and capability index only route requests. Reviewed rules with a shared responsibility theme get a small index derived from their titles; each rule keeps its own detail and evidence, while standalone rules link directly from the map. CodeGraph structure stays in `.codegraph` instead of being copied into Skill pages. `code-navigation.md` explains local queries. Entries independently reviewed by AI and matched to local symbol facts appear in `business-methods.md` with task applicability, prerequisites and result or failure semantics. An Agent loads only the relevant content.

## What the Entry Skill Should Answer

`SKILL.md` must help an Agent decide before editing:

1. Which project, module, or reviewed business responsibility owns the task?
2. Does an explicit Rule or Workflow constrain the task first?
3. Which project references, capability entries, and source locations must be read?
4. When several business patterns apply, which responsibility does the evidence establish and when should related references be read?
5. Which impact boundaries or unknowns must be confirmed before implementation?

After a business pattern matches, read only its detail and evidence entry; follow another reference when the target page explicitly identifies a cross-boundary relationship. A generic verb or technical term alone cannot establish ownership, and current source remains the final check.

Rule and Workflow `summary` and `route_terms` are also entry-navigation metadata only; explicit scopes and the complete resource text still define boundaries and permissions.

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
- Do reviewed business patterns have natural responsibility names, evidence entries, and clear applicability boundaries?
- Do capability entries include a declaration, prerequisites, result semantics, and locatable source?
- Does the module map describe real responsibility and dependencies rather than self-dependencies or guessed paths?
- Does output contain only valid projections with working plans and links?

Generation readiness parses real Markdown links and confirms, after symlink resolution, that every local target remains inside the current Skill root. Missing or escaping targets block delivery, while valid generic, index, and call expressions in source text are not treated as local links.

See [Quality and Contributing](Quality-and-Contributing.md) for quality testing and maintenance.

---

**Next:** [Rules and Workflows](Rules-and-Workflows.md) - add explicit constraints and task procedures to a generated Skill.

**Related:** [Knowledge Maintenance](Knowledge-Maintenance.md) - correct persisted knowledge; [Quality and Contributing](Quality-and-Contributing.md) - judge delivery value through real changes.

**Language:** [简体中文](../Skills-and-Resources.md)
