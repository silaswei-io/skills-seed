# Skills Seed Quality Regression

This directory contains a self-contained quality regression suite for generated project Skills. It is intentionally separate from product code.

## Quick Start

```bash
.test/quality/run.sh
```

The default case runs the following pipeline:

1. Verify that production prompts, templates, and non-test pipeline code contain no private marker from any quality fixture.
2. Build the current `skills-seed` binary.
3. Use `jzero new` to create a fresh project under `.test/quality/runtime/`.
4. Copy only interface descriptions and handwritten scenario code from the fixture.
5. Run `jzero gen` to create transport and generated source in the runtime project.
6. Run `skills-seed init` and `skills-seed sync`.
7. Run `sync` again without source changes and compare the complete generated Skill trees.
8. Score the generated Skill and write JSON and Markdown reports under `.test/quality/reports/`.

Each run archives the generated Skill and `.skills-seed/store/documents` under `.test/quality/runtime/<case>/snapshots/`. These snapshots make stage failures and output drift inspectable without changing production code.

No jzero-generated source is stored in the fixture. The generated project, Skills, logs, caches, and reports are ignored by Git.

Set `QUALITY_AGENT` to exercise another configured Agent engine:

```bash
QUALITY_AGENT=claude .test/quality/run.sh
```

## Fast Checks

Run scorer self-tests without invoking jzero or an Agent:

```bash
python3 -m unittest discover -s .test/quality/tools/tests -p 'test_*.py' -v
```

Re-score an existing runtime project after editing a case definition:

```bash
python3 .test/quality/tools/quality.py \
  --root "$PWD" \
  --config .test/quality/cases/quality_lab.json \
  score
```

Score any existing generated Skill without invoking a builder or Agent:

```bash
.test/quality/score.sh \
  /absolute/path/to/project \
  /absolute/path/to/project/.agents/skills/project-dev
```

The command prints the final score and writes detailed JSON and Markdown reports. The score is always based on Skill content; when project generation or Agent execution fails, the run is reported as `not_scored` instead of assigning a misleading zero.

## Quality Model

The suite uses an explicit 100-point budget and separates hard delivery gates from scored quality dimensions.

- `delivery`: 15 points for expected output, valid local links, and real cited source paths.
- `authority`: 20 points for exact project rules and command boundaries surviving authority extraction and project-map refresh.
- `capability`: 20 points for reusable entries with verified locations, signatures, prerequisites, results, and usage.
- `behavior`: 25 points for accurate state, orchestration, failure, concurrency, idempotency, partial-success, and routing boundaries.
- `architecture`: 10 points for valid module ownership and relationships.
- `loading`: 10 points for progressive routing without duplicated catalogs.

Delivery also includes repeat-run stability: unchanged inputs must produce byte-identical Skill files. Stability is evaluated generically from the output tree and has no project, language, or framework rules.

Loading includes a frontmatter trigger contract because an Agent decides whether to load a Skill from `name` and `description` before it can see body routing. Each case declares the task semantics its entry description must cover; the scoring engine only validates structure and configured semantic groups.

A check earns partial credit for the exact sub-assertions it satisfies. Hard gates additionally cap the final score, so unrelated strengths cannot hide broken links, fabricated paths, missing critical rules, invalid module relations, false atomicity, or inverted business guarantees.

Business-pattern checks require a concrete source anchor and all expected semantics in the same generated knowledge block. Keywords scattered across unrelated sections do not satisfy a pattern contract. Link and source-path checks can also require a minimum amount of evidence, so an empty index cannot pass by vacuous truth.

Each failed check reports evidence and an optimization hint. The hint identifies the likely learning, verification, routing, or rendering stage to inspect; it is not part of the score.

The 100-point result is a regression score against the facts declared by a case fixture. It is stable and explainable for that case, but it is not a universal semantic judge for an arbitrary project with no case-specific oracle. Add or revise fixture contracts when evaluating a new class of project knowledge.

The prompt-independence check is a leakage gate, not proof of semantic generality. Production guidance must still be reviewed in abstract terms, and materially new extraction strategies should be exercised against a structurally different case before they are treated as general improvements.

## Adding Cases

Keep project semantics in a case JSON file and fixture. The scoring engine must remain unaware of project names, languages, frameworks, or business concepts.

A case may use any builder. The builder owns project creation and code generation, while `quality.py` only runs it and evaluates the resulting Skill. Assertions should describe future Agent decisions rather than require a fixed number of patterns or exact prose.

For source-learned business behavior, prefer `section_contract` with an exact source path in `anchors` and several independently meaningful semantic groups. Include both positive contracts and `not_contains` inversions so a plausible but false guarantee cannot receive a high score.
