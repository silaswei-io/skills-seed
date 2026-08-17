# Skills Seed

Skills Seed turns explicit constraints, source-backed evidence, and project navigation into local Skills, so an Agent understands rules, ownership, reusable capabilities, and impact boundaries before changing code.

> **New here?** At the Git project root, run `skills-seed init`, then `skills-seed sync`. See [Getting Started](Getting-Started.md) for prerequisites and verification.

## Choose Your Path

### First Adoption

1. Read [Getting Started](Getting-Started.md) and complete initialization and first generation.
2. Read [Core Concepts](Concepts.md) to distinguish Rules, Workflows, Context, and source-learned knowledge.
3. Open the generated `SKILL.md` and confirm that it routes to project references and user resources.

### Daily Use

1. Run `skills-seed sync` at the project root to refresh knowledge and output.
2. Read [Learning and Sync](Learning-and-Sync.md) for stages, parallelism, admission, and regeneration.
3. Read [Command Workbench](Command-Workbench.md) when choosing a command or subcommand.
4. Read [Rules and Workflows](Rules-and-Workflows.md) and [Knowledge Maintenance](Knowledge-Maintenance.md) when maintaining team resources.

### Team Maintenance and Troubleshooting

- For independent child projects, read [Workspace](Workspace.md).
- For interrupted runs, unexpected output, or manual repair, read [Learning Failures and Manual Repair](Learning-Failures-and-Manual-Repair.md).
- For exact flags, fields, and defaults, read [Configuration and Reference](Reference.md).
- For reviewing output quality or contributing, read [Quality and Contributing](Quality-and-Contributing.md).

## Working Loop

```text
User-maintained Rules / Workflows / Context
                  +
Current source, structure, and change evidence
                  |
                  v
        Learn and persist project knowledge
                  |
                  v
     Generate modular, task-loaded Skills
                  |
                  v
Agent reads the entry and relevant references before editing
```

## What It Is and Is Not

| Not | Instead |
|---|---|
| One static, oversized project description | Modular local context loaded by task |
| Team standards guessed by a model | Explicit Rules and source-backed knowledge with known origin |
| Opaque remote memory | Reviewable, refreshable, committable project resources |

## Design Principles

- **Explicit rules win**: User-maintained Rules define mandatory boundaries; source can provide reusable practice and navigation evidence.
- **Evidence before summary**: Low-frequency examples, guesses, and generic advice must not become project standards.
- **Load on demand**: The entry Skill remains short; details live in references, rules, and workflows.
- **Local and reviewable**: Project state, runtime archives, and generated output stay in the repository.
- **Continuously refreshed**: Knowledge can be added, revised, withdrawn, or downgraded as source changes.

See the [ultimate goal](../../ULTIMATE_GOAL.md) for the full product contract.

---

**Start here:** [Getting Started](Getting-Started.md)

**Language:** [简体中文](../Home.md)
