# Quality and Contributing

> **Use this page when:** you are reviewing generated-Skill value, running quality checks, or contributing code and documentation to Skills Seed.

## How to Judge Whether a Generated Skill Helps

The final acceptance criterion is not pattern count, file count, or prompt length. It is whether a future Agent can reliably answer:

1. Which project, module, or capability entry owns this change?
2. Which explicit rules and references must be read first?
3. Which existing capabilities should be reused?
4. Which API, data, configuration, generated-output, or cross-project boundaries may be affected?
5. Which risks and unknowns need confirmation before implementation?

The [ultimate goal](../../ULTIMATE_GOAL.md) is the shared acceptance criterion for implementation and generated output.

## Quality Layers

| Layer | Focus |
|---|---|
| Input and boundaries | Authority sources, candidate paths, scope, and recovery state |
| Output contract | Whether Agent output satisfies structured Schema and source constraints |
| Knowledge admission | Evidence, applicability, reuse value, and overclaim risk |
| Generated delivery | Entry, links, Rule projection, Workflow projection, and references |
| Real tasks | Whether Agents locate, reuse, respect boundaries, and avoid wrong changes |

`.test/quality/` contains repeatable quality-evaluation resources. It must remain free of project-, language-, framework-, and business-specific assumptions; it evaluates general generated-Skill usefulness and contract completeness.

## Development Checks

Run at least:

```bash
go test ./...
go vet ./...
staticcheck ./...
go build ./cmd/skills-seed
```

When extending error branches, path boundaries, or persistence behavior, also generate one repository-wide coverage report and confirm that the new tests execute the intended branches:

```bash
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Release iterations use `.test/release-iterate.sh`, which updates version metadata, runs quality gates, commits, tags, and pushes. Add a new current-version entry to both changelogs before release; historical entries must not be rewritten.

## Documentation Contributions

- When a user-visible entry or behavior changes, update the corresponding Wiki page and README link.
- When flags or configuration change, update the [command reference](../../COMMANDS.EN.md) or [configuration reference](../../CONFIGURATION.EN.md).
- When changing a learning pipeline, prompt, domain model, or template, explain how it improves future Agent decisions against the [ultimate goal](../../ULTIMATE_GOAL.md).
- Keep Chinese and English pages structurally aligned. Examples must remain general and must not make a fixture, language, or framework a product prerequisite.

See [Contributing](../../../CONTRIBUTING.en.md) for detailed rules.

---

**Next:** [Home](Home.md) - return to the paths for first adoption, daily work, and team maintenance.

**Related:** [Skills and Resources](Skills-and-Resources.md) - review final Skill entry points and projections; [Configuration and Reference](Reference.md) - find authoritative implementation and quality-gate material.

**Language:** [简体中文](../Quality-and-Contributing.md)
