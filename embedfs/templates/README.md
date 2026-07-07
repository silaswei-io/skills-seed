# Template Architecture

Templates are organized by two different dimensions.

## Prompts

`templates/prompts` stores prompt templates used to ask the AI agent for analysis or generated data.

- `loader/` stores English runtime agent prompts rendered by `internal/prompts/loader`.
- `append/` stores mandatory English fragments appended after runtime prompts, such as final output contracts and output-language rules.
- `context/` stores templates used to initialize project context files under `.skills-seed/context`.

Runtime prompt IDs use kebab-case prefixes:

- `learn-*` for learning and code analysis prompts.
- `fix-*` for fix generation prompts.
- `pattern-*` for pattern maintenance prompts.
- `project-*` for project profile analysis prompts.
- `skill-project-*` and `skill-workspace-*` for prompts that produce data consumed by generated skills.

## Skills

`templates/skills` stores templates for files generated into skill directories.

- `common/` stores templates shared by providers.
- `claude/` and `codex/` store provider-specific overrides or metadata.
- `project/` and `workspace/` under those provider directories describe the generated skill type.

Skill template IDs are declared in `internal/templates/skills/catalog.go`. The catalog maps a logical ID such as `project-skill` or `workspace-skill` to a template path and a normalized output path. Root skill templates may have descriptive template filenames, but the generated root file remains `SKILL.md`.

## Locale

Runtime prompt templates under `loader/` and `append/` are maintained in English only. The output language is controlled by `skills.locale` through the appended output contract.

User-facing context and generated Skills templates may have locale variants. Templates without a locale suffix are Simplified Chinese for those user-facing artifacts, and English variants use `.en-US` before the template extension.
