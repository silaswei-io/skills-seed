# Template Architecture

Templates are organized by two different dimensions.

## Prompts

`templates/prompts` stores prompt templates used to ask the AI agent for analysis or generated data.

- `common/` stores runtime agent prompts shared by project and workspace flows.
- `project/` stores templates used to initialize user-editable project prompt files under `.skills-seed/prompts/project` and `.skills-seed/prompts/custom`.
- `workspace/` stores templates used to initialize user-editable workspace prompt files under `.skills-seed/prompts/workspace`.

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

The default template language is Simplified Chinese. Templates without a locale suffix are Chinese. English variants use `.en-US` before the template extension.
