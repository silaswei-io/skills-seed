<div align="center">

# Skills Seed

**Make AI Agents remember your project rules.**

[![CI](https://img.shields.io/github/actions/workflow/status/silaswei-io/skills-seed/ci.yml?branch=main&label=ci&logo=github&style=flat-square)](https://github.com/silaswei-io/skills-seed/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/silaswei-io/skills-seed?style=flat-square)](https://github.com/silaswei-io/skills-seed/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/silaswei-io/skills-seed?style=flat-square)](go.mod)
[![License](https://img.shields.io/github/license/silaswei-io/skills-seed?style=flat-square)](LICENSE)

[简体中文](README.md) · [English](README.en.md)

`Claude Code` · `Codex` · `Local Skills` · `Workspace` · `Code Review`

[Quick Start](#quick-start) · [Output Preview](#output-preview) · [Prompts](#prompts-and-one-time-guidance) · [Design Principles](#design-principles) · [Workspace](#workspace) · [Command Reference](docs/COMMANDS.EN.md)

</div>

Skills Seed is built for existing projects. It reads current code, Git history, project structure, and recorded check hits, then turns real team practices into local knowledge: naming, error handling, directory layout, business methods, utilities, testing habits, and API conventions.

It solves a specific problem: when an AI Agent enters a project for the first time, it usually does not know how this project should be changed. Skills Seed extracts those implicit rules from real code and turns them into local skills that can be loaded, refreshed, and checked.

What you get is not a generic project summary. It is Agent working context generated for the current project:

- Which directories own which capabilities, and where to look first for a change.
- Which business methods, utilities, error-handling patterns, and test habits are already established.
- Which child project in a workspace should handle a request, and how cross-project changes should be inspected.
- Which rules repeatedly appear in `check` or review and should be prioritized in generated skills.

All data is local by default. The generated skills type is selected by `skills.target`; the Agent CLI used for analysis, learning, and summaries is selected by `agent.engine`. That means you can use `claude` for analysis and summarization while producing `codex` skills.

## Why Use It

| Problem | What Skills Seed Does |
|---|---|
| You need to explain the same project structure to AI repeatedly | Generates reusable skills from code, Git history, and project profiles |
| Team conventions live in code, reviews, and individual memory | Extracts patterns, business methods, utilities, and test habits |
| A multi-project workspace makes context selection unclear | Root skill handles routing; child skills stay independent |
| Generated skills could feed back into later learning | Excludes `.skills-seed`, skills outputs, and build artifacts by default |
| You do not know whether generated rules are useful | Records pattern hits from `check`; `patterns stats` shows quality and usage |

## Use Cases

- You want an AI Agent to work on an existing business system without re-explaining the project structure and constraints every time.
- Your team has stable conventions for naming, layout, error handling, business methods, and tests, and you want those conventions available to the assistant.
- Your workspace contains multiple independent Git child projects, and you want child projects to learn and generate independently while the root skill handles routing and cross-project impact.
- You want `check` / pre-commit hooks to apply learned rules to future changes and record which rules are actually hit.
- You want to import local review comments and see which recurring review issues are already covered by learned patterns.

## Output Preview

After `generate skills`, the default output looks like this:

```text
.agents/skills/skills-seed-skills/
├── SKILL.md
├── agents/
│   └── openai.yaml
└── references/
    ├── project-overview.md
    ├── project-spec.md
    ├── business-methods.md
    ├── modules.md
    └── patterns/
        ├── business.md
        ├── concurrency.md
        ├── config.md
        ├── database.md
        ├── error.md
        ├── middleware.md
        ├── structure.md
        └── utils.md
```

`SKILL.md` is the Agent entry point. `references/` keeps the fuller project profile, specs, and pattern details. The Agent can read references when it needs depth, instead of loading every detail into the entry document.

## Workflow

```text
init -> learn current / learn history -> generate skills -> check
```

| Stage | Command | Output |
|---|---|---|
| Initialize | `skills-seed init` | `.skills-seed/config.yaml`, local database, default prompts |
| Learn current code | `skills-seed learn current` | patterns, business methods, utilities, project profile |
| Learn history | `skills-seed learn history` | long-lived rules extracted from Git evolution |
| Generate skills | `skills-seed generate skills` | `SKILL.md`, project overview, specs, pattern references |
| Check later changes | `skills-seed check` | issues, fix suggestions, and pattern hits based on learned rules |

Starting in 0.10.6, running `skills-seed init` without flags opens an interactive initialization flow. Starting in 0.11.1, the default path asks only for tool language, initialization type, Agent, total Agent parallelism, and the execution plan; analysis depth, unit split scope, Skills language, and Skills type live under optional advanced settings. Running `skills-seed sync` without flags prompts to resume or restart when unfinished state exists. Use `--no-interactive` in scripts, and `sync --resume` / `sync --restart` to control resume behavior explicitly.

Starting in 0.10.7, `patterns add` and user-pattern sync use `--context` for the natural-language description, `patterns update <id> --context "<request>"` can revise one pattern while preserving its ID and workspace ownership, and `patterns show` supports `--sort updated|score|hits|category`. Model-output parsing now also repairs trailing commas, comments, single-quoted strings, Python-style literals, and missing commas between object fields or array values.

Starting in 0.11.0, `learning.current.mode` supports `fast`, `normal`, and `deep` learning strategies. Generated skills now include related-reference routing, business-pattern importance layers, change-scope validation matrices, and module-grouped entry method indexes. Reference generation also validates source evidence paths before rendering, reducing links to files that do not exist.

Starting in 0.11.1, `learn current` supports `learning.current.parallelism` for in-project analysis-unit concurrency, while workspace root `agent.parallelism` only controls child-project concurrency. `learning.current.scope` supports `domain`, `flow`, and `module` to guide analysis-unit splitting by business domain, workflow, or module granularity. Model-output parsing also repairs invalid evidence line ranges, for example normalizing `"line": 29-43` to a single line number.

Starting in 0.11.2, `learning.current.max_units_per_call` controls how many units one AI call may analyze, with the default `1` meaning no batching; raise it explicitly when reducing call count matters more. Batch result parsing recognizes top-level `units`, and current-code learning prompts now harden JSON type contracts such as `profile_delta.layers`. Generated skills also keep low-frequency or local evidence out of the strong-constraint layer so one-off examples are not promoted into mandatory standards.

Starting in 0.11.6, current-code and history-learning prompts use a stricter Candidate Admission Gate: facts, summaries, weak local evidence, and generic engineering practice are dropped unless they become source-backed, project-specific, routeable rules that can guide future changes. Business coverage matrices now prevent missed strong candidates instead of forcing pattern output.

AI file selection only provides relevance recommendations; the final analysis scope is decided by a local stable policy that merges recommendations, cache entries, and explicit user focus paths. Identical inputs reuse the selection cache; explicit focus files cannot be excluded by an AI recommendation, and large candidate sets with overly narrow recommendations are deterministically filled to a minimum budget to reduce large swings in analyzed file counts across runs.

Starting in 0.9.0, pattern deduplication and consolidation happen before storage. Candidate patterns from `learn current`, `learn history`, and `patterns add` are curated by AI and validated by the service before they are written to the local pattern store. `generate skills` only reads stored data and no longer merges or repairs the pattern store. To explicitly compact historical patterns, use `skills-seed patterns compact`.

Starting in 0.10.4, default pre-storage curation uses local deterministic merging and keeps its internal pattern set unique by pattern ID. When a candidate reuses an existing ID, or a historical store already contains duplicate IDs, the merger first collapses them into one higher-quality pattern before writing, avoiding duplicate curated pattern IDs during structural validation.

Starting in 0.10.5, `learn current` unit analysis no longer injects the existing pattern store into every unit prompt, preventing context from growing with the number of saved patterns. Post-learning deduplication remains handled by local deterministic merging; use `skills-seed patterns compact --ai` when explicit semantic compaction is needed.

Starting in 0.9.1, `learn current` can narrow large candidate file sets through AI relevant-file selection before analysis. When `generate skills` is run explicitly, it deletes the old skills-seed generated output directory and fully rebuilds it. The root `completion` command has been removed, and Chinese help text is now consistent.

`generate skills` ranks learned patterns by quality: rules with higher effective score, more check hits, and higher confidence are favored, reducing generic or duplicated rules in the final skills.

Starting in 0.7.0, learning and project-profile analysis use an embedded tree-sitter structural pre-scan when bounded inputs exist. It extracts symbols, imports, entry points, and module clues so the Agent can prioritize source files to inspect. It no longer depends on an external CodeGraph command or index; configure it under `analysis.structural`, where `max_symbols` controls emitted symbol count and `max_file_size` controls the per-source-file size limit.

When an AI Agent hits retryable errors such as 429 / 529 / overloaded, Skills Seed retries with exponential backoff according to `agent.retry`. Long-running progress lines switch between normal, waiting, and retrying states, for example `Analyze current codebase`, `Analyze current codebase (API Error: 529 overloaded_error, call 3m37s, retry in 15s)`, and `Analyze current codebase (attempt 2)`. The call duration is how long the failed Agent call took; `retry in 15s` is the backoff wait.

## Prompts And One-Time Guidance

`skills-seed init` creates `.skills-seed/prompts/`. These files are not full replacements for the built-in prompts. They are merged with built-in prompts as project context, workspace constraints, or user instructions for learning and generation.

Starting in 0.7.1, generated metadata, empty scaffolding, and unfilled placeholder text in default prompt files are filtered during rendering. Only user-authored constraints enter the Agent input. Each rendered prompt is saved under `.skills-seed/runtime/rendered-prompts/`; the neighboring `.manifest.json` records whether base, project, workspace, and instruction fragments were included and their lengths, making context provenance easier to debug.

Starting in 0.7.2, project-profile analysis performs a narrow JSON recovery for duplicated object-start fragments inside object arrays in model output. If parsing still fails, it returns an error and keeps the existing profile instead of saving an `unknown/parse failed` placeholder as a successful result.

Starting in 0.7.3, current-code learning commits file-analysis fingerprints only after patterns are persisted, preventing unsuccessfully learned files from being skipped by later incremental learning. Pattern, file-fingerprint, hit, and review-comment records maintain `created_at/updated_at`; business-method code locations are stored in the DB as language-agnostic snapshot metadata, and `patterns show <pattern-id>` prints the full detail view for one saved pattern.

Starting in 0.9.8, patterns store `evidence_locations` separately as pattern-level source evidence locations. The `patterns show` overview prefers business/utility-method `code_location`; when a pattern has no business method, it falls back to the first evidence location and the detail view prints the full evidence-location list.

Starting in 0.8.0, Agent outputs are saved separately under `.skills-seed/runtime/agent-outputs/`. Runtime logs keep only output lengths and archive paths, and no longer include model reply previews or raw stdout/stderr. Starting in 0.10.3, valid JSON output is formatted as a readable fenced `json` block inside the `.md` archive. Business-method locations now use structured `code_location` metadata throughout, generated business-method references show location status, and project skills/references are more compact so the entry skill guides Agents to read the minimum relevant references for each task.

Starting in 0.9.6, debug records under `.skills-seed/runtime` use the unified `YYYYMMDD-HHMMSS.NNNNNNNNN-<kind>-<name>` filename prefix, including rendered prompts, Agent output archives, and runtime input temporary directories, making it easier to sort by time while inspecting context and model outputs from one run.

Starting in 0.9.0, learning and user-added patterns use the `pattern-curate` prompt for pre-storage curation: every candidate must be covered, duplicate rules must be consolidated, code evidence must come from input source, and invalid or low-quality candidates are dropped. The old pre-generation merge flow and `patterns merge` command have been removed; generation remains read-only.

Starting in 0.9.1, model output parsing runs through a stronger JSON repair flow for common issues such as duplicated object starts, invalid escapes, unescaped quotes inside strings, and missing closing containers. Starting in 0.10.5, the repair flow also handles raw newlines/control characters inside strings, bare object keys, and array items missing an object-start marker; starting in 0.10.7, it also handles trailing commas, comments, single-quoted strings, Python-style literals, and missing commas between object fields or array values.

Common layout:

```text
.skills-seed/prompts/
├── project/
│   ├── project-profile.md      # Project facts used by related prompts
│   ├── common.md               # Common project constraints used by related prompts
│   └── <prompt-id>.md          # Optional project-level fragment for one prompt
├── workspace/
│   ├── skill-workspace-profile.md
│   └── skill-workspace-spec.md
└── instructions/
    └── <prompt-id>.md          # User instructions appended to one prompt
```

Runtime merge order:

```text
built-in prompt
+ project/project-profile.md
+ project/common.md
+ project/<prompt-id>.md
+ workspace/<prompt-id>.md
+ instructions/<prompt-id>.md
+ built-in final output contract
```

Use `instructions/<prompt-id>.md` for stable team requirements, such as "ignore temporary debugging code while learning commits" or "prioritize API compatibility rules when generating skills". These instructions are appended after the built-in prompt, but they must not change the JSON / Markdown output format required by the built-in prompt. Skills Seed appends a non-editable final output contract after user fragments to protect parser-facing output.

`--context` and `--context-path` are one-time guidance for the current `learn current` command. They are not written to `.skills-seed/prompts/`, and they are not passed as temporary input to `generate skills`. Use them for temporary instructions:

```bash
skills-seed learn current --context "Focus only on compatibility boundaries"
skills-seed learn current --context-path .skills-seed/context.md
```

If a rule should apply across future runs, put it in `.skills-seed/prompts/instructions/<prompt-id>.md`. If it only explains or limits one run, use `--context` or `--context-path`.

`learn current`, `preview`, and structural analysis now share one file-selection policy: by default they analyze source files, build config, and dependency config while continuing to skip documents, generated outputs, paths ignored by Git, paths matched by global `exclude`, and generated Skills output directories.

## Quick Start

### Single Project

```bash
cd your-project
skills-seed init --mode project --agent codex --skills codex --locale en-US
skills-seed learn current
skills-seed generate skills
test -f .agents/skills/skills-seed-skills/SKILL.md
```

For Codex, the default generated skill is:

```text
.agents/skills/skills-seed-skills/SKILL.md
```

For Claude Code, the default generated skill is:

```text
.claude/skills/skills-seed-skills/SKILL.md
```

### Workspace

Workspace mode is for a root directory that contains multiple independent Git child projects. During initialization, Skills Seed scans first-level directories; only directories with their own `.git` are added to `workspace.projects`, and `.skills-seed` is initialized for the children found at that time. Files such as `go.mod`, `package.json`, install scripts, Helm charts, and Terraform files classify child project type and language only.

```bash
cd your-workspace
skills-seed init --workspace --agent codex --skills codex --locale en-US
skills-seed learn current
skills-seed generate skills
test -f .agents/skills/skills-seed-skills/SKILL.md
```

If a new project is copied into the workspace root later, use `workspace add` to sync config and initialize the child repo:

```bash
skills-seed workspace add .
skills-seed workspace add backend frontend
```

The workspace root coordinates routing and cross-project relationships only. Child projects use their own `.skills-seed` directories to learn, generate, and store patterns independently. Existing child `.skills-seed/config.yaml` files are never overwritten; if a child uses a different agent from the root, it is reported and preserved.

The only user-facing workspace config field is `workspace.projects`. Shared libraries, contracts, and infrastructure impact are no longer hand-written through `workspace.shared`, `workspace.contracts`, or `workspace.infra`; during `learn current`, they are analyzed from repository evidence, child `project-profile.json` files, and one-shot user context into root `workspace-profile.json` / `workspace-spec.json`. `generate skills` only consumes these learned artifacts and no longer accepts user context.

## Design Principles

- **Local-first**: learned data, config, and generated output stay in the current repository by default.
- **Existing-code-first**: real code, Git history, and check hits are the source of truth, instead of requiring users to hand-write a large project guide.
- **Agent-agnostic**: `agent.engine` and `skills.target` are separate, so one Agent can analyze while another Agent's skill format is generated.
- **Workspace-aware**: root workspaces handle routing and cross-project relationships; child repos learn and generate independently.
- **Feedback-driven**: `check` and review hits feed back into pattern quality, making useful rules more likely to reach the final skills.

## Agent And Skills Target

`agent.engine` selects the Agent CLI used for analysis, learning, and generation summaries. `skills.target` selects which Agent skill format to generate.

For example, use Claude for analysis and summarization while generating Codex skills:

```yaml
agent:
  engine: "claude"
  commands:
    claude: "claude"
    codex: "codex"

skills:
  target: "codex"
  paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"
```

Built-in targets:

| Name | Purpose | Default Output |
|---|---|---|
| `claude` | Claude Code skills | `.claude/skills/skills-seed-skills` |
| `codex` | Codex skills | `.agents/skills/skills-seed-skills` |

## Common Commands

| Command | Description |
|---|---|
| `skills-seed init` | Initialize a single project or workspace root |
| `skills-seed workspace add .` | Auto-detect and add all child projects from a workspace root |
| `skills-seed workspace add <child...>` | Add specific child projects from a workspace root |
| `skills-seed learn current` | Incrementally learn rules and profile from current code |
| `skills-seed learn history` | Learn long-lived rules from Git history |
| `skills-seed generate skills` | Generate skills for the current `skills.target` |
| `skills-seed workflow --context "<notes>"` | Infer and save a user workflow through the Agent; omit `--name` to generate a name, same-name workflows merge by default, use `--overwrite` to replace |
| `skills-seed patterns add --context "<description>"` | Add a user-defined pattern in natural language; use `--context-path` for longer notes |
| `skills-seed patterns update <id> --context "<request>"` | Update a specific user pattern |
| `skills-seed patterns compact` | Explicitly compact similar stored patterns |
| `skills-seed sync` | Learn current code in one command and generate skills when changes are found |
| `skills-seed check` | Check staged files or Git-tracked files |
| `skills-seed patterns stats` | Show pattern quality, hit counts, and recent hits |
| `skills-seed patterns show` | Show pattern timestamps, business-method locations, and pattern evidence locations from the DB |
| `skills-seed review import --from-file` | Import local review comments |
| `skills-seed hook install` | Install the local pre-commit hook |

See [Command Reference](docs/COMMANDS.EN.md) for all flags and forms.

User-provided goals, constraints, background, paths, or rough notes are inferred by the current Agent into a standard workflow, then saved to `.skills-seed/workflows/<id>/WORKFLOW.md`; when `--name` is omitted, `<id>` comes from the Agent-generated English title slug, and repeated titles receive a numbered suffix. Original notes and metadata are stored in `metadata.yaml` in the same directory. Same-name workflows merge and deduplicate by default; add `--overwrite` to replace one completely. Generated skills receive workflows under `workflows/`, with related scripts under `scripts/workflows/<id>/`.

## Local And Safety Boundaries

- Project code is not uploaded to a remote knowledge base by default; learned data is written to `.skills-seed` in the current repository.
- `check` and `generate skills` call the configured Agent CLI, so network behavior depends on the `claude` / `codex` CLI you use.
- `.skills-seed/memory/project.db` is a local BoltDB file and can only be opened by one `skills-seed` process at a time. If another command is learning, compacting, or inspecting patterns, a new command may report that the database is in use; wait for the running command to finish and retry.
- Generated skills directories, `.git/**`, `.skills-seed/**`, and common build outputs are excluded by default so generated content does not feed back into later learning.
- A handwritten `SKILL.md` without a `generated-by: skills-seed` marker is not overwritten by default.

## Install

```bash
go install github.com/silaswei-io/skills-seed/cmd/skills-seed@latest
skills-seed --help
```

If the command is not found, add `$GOPATH/bin` or `$GOBIN` to `PATH`.

Build from source:

```bash
git clone https://github.com/silaswei-io/skills-seed.git
cd skills-seed
go build -o skills-seed ./cmd/skills-seed
```

## Requirements

- Go 1.25.6+
- A Git repository
- An available AI Agent CLI: default `claude`; switch with `--agent codex` or `agent.engine`

## Documentation

- [Command Reference](docs/COMMANDS.EN.md)
- [Configuration Reference](docs/CONFIGURATION.EN.md)
- [Changelog](CHANGELOG.en.md)

## Development

```bash
go test ./...
go vet ./...
go build -o skills-seed ./cmd/skills-seed
```

---

<div align="center">

Released under the [MIT License](LICENSE).

</div>
