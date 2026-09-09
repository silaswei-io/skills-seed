package skilloutput

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark/ast"
)

func TestAuditReadinessAcceptsCompleteSkill(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "references"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\nname: demo-dev\ndescription: Demo skill\n---\n\n[Spec](./references/spec.md)\n\nrequires_authorization\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "references", "spec.md"), []byte("# Spec\n\nAGENTS.md\n"), 0o644))

	err := AuditReadiness(root, ReadinessRequirements{
		ExpectedFiles: []string{"SKILL.md", "references/spec.md"},
		RequiredContent: map[string][]string{
			"SKILL.md":           {"requires_authorization"},
			"references/spec.md": {"AGENTS.md"},
		},
	})

	require.NoError(t, err)
}

func TestAuditReadinessRejectsBrokenLocalLink(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\nname: demo-dev\ndescription: Demo skill\n---\n\n[Missing](./references/missing.md)\n"), 0o644))

	err := AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}})

	require.ErrorContains(t, err, "broken link")
}

func TestAuditReadinessIgnoresCodeLikeIndexExpression(t *testing.T) {
	root := t.TempDir()
	content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\nctx deadline uses params[\"deadline\"](RFC3339Nano).\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))

	require.NoError(t, AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}}))
}

func TestAuditReadinessIgnoresGoGenericCall(t *testing.T) {
	root := t.TempDir()
	content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\n调用 gobase.SetCtxValue[bool](ctx, gobase.ENABLE_BRUSH_DATA, true) 开启防刷标记。\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))

	require.NoError(t, AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}}))
}

func TestAuditReadinessAcceptsLinkWithTitle(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "references"), 0o755))
	content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\n[Spec](./references/spec.md \"项目规范\")\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "references", "spec.md"), []byte("# Spec\n"), 0o644))

	require.NoError(t, AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}}))
}

func TestAuditReadinessRejectsBrokenReferenceLink(t *testing.T) {
	root := t.TempDir()
	content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\n查看[项目规范][spec]。\n\n[spec]: ./references/missing.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))

	err := AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}})

	require.ErrorContains(t, err, "broken link")
	require.ErrorContains(t, err, "./references/missing.md")
}

func TestAuditReadinessRejectsBrokenImage(t *testing.T) {
	root := t.TempDir()
	content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\n![架构图](./references/missing.png)\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))

	err := AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}})

	require.ErrorContains(t, err, "broken link")
	require.ErrorContains(t, err, "./references/missing.png")
}

func TestAuditReadinessChecksAdjacentMarkdownLink(t *testing.T) {
	root := t.TempDir()
	content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\nprefix[Missing](./missing.md)\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))

	err := AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}})
	require.ErrorContains(t, err, "broken link")
}

func TestAuditReadinessIgnoresCodeSpansAndFences(t *testing.T) {
	root := t.TempDir()
	content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\n`[Missing](./missing.md)`\n\n```go\n[Missing](./missing.md)\n```\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))

	require.NoError(t, AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}}))
}

func TestAuditReadinessRejectsMissingCriticalProjection(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\nname: demo-dev\ndescription: Demo skill\n---\n"), 0o644))

	err := AuditReadiness(root, ReadinessRequirements{RequiredContent: map[string][]string{
		"SKILL.md": {"forbidden"},
	}})

	require.ErrorContains(t, err, "missing required projection")
}

func TestAuditReadinessRejectsMissingAuthoritativeRuleBody(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "references"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\nname: demo-dev\ndescription: Demo skill\n---\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "references", "project-spec.md"), []byte("# Project Spec\n\nCLAUDE.md\n"), 0o644))

	err := AuditReadiness(root, ReadinessRequirements{RequiredContent: map[string][]string{
		"references/project-spec.md": {"preserve the declared contract boundary"},
	}})

	require.ErrorContains(t, err, "missing required projection")
}

func TestAuditReadinessRejectsInvalidFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "missing", content: "# Demo\n", want: "must start with YAML frontmatter"},
		{name: "unclosed", content: "---\nname: demo\n", want: "frontmatter is not closed"},
		{name: "invalid yaml", content: "---\nname: [demo\n---\n", want: "invalid SKILL.md frontmatter"},
		{name: "missing name", content: "---\ndescription: Demo\n---\n", want: "requires non-empty name"},
		{name: "missing description", content: "---\nname: demo\n---\n", want: "requires non-empty description"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(test.content), 0o644))

			err := AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}})

			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestAuditReadinessRequiresSkillAndExpectedFiles(t *testing.T) {
	root := t.TempDir()
	err := AuditReadiness(root, ReadinessRequirements{})
	require.ErrorContains(t, err, "read SKILL.md")

	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\nname: demo\ndescription: Demo\n---\n"), 0o644))
	err = AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"references/missing.md"}})
	require.ErrorContains(t, err, "expected file")

	require.NoError(t, os.Mkdir(filepath.Join(root, "references"), 0o755))
	err = AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"references"}})
	require.ErrorContains(t, err, "is not a regular file")
}

func TestAuditReadinessRejectsMissingRequiredProjectionFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\nname: demo\ndescription: Demo\n---\n"), 0o644))

	err := AuditReadiness(root, ReadinessRequirements{RequiredContent: map[string][]string{
		"references/missing.md": {"required"},
	}})

	require.ErrorContains(t, err, "read required projection")
}

func TestAuditReadinessRejectsLinkEscapingSkillRoot(t *testing.T) {
	root := t.TempDir()
	content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\n[Outside](../outside.md)\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))

	err := AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}})

	require.ErrorContains(t, err, "escapes skill root")
}

func TestAuditReadinessIgnoresNonLocalLinksAndNonMarkdownFiles(t *testing.T) {
	root := t.TempDir()
	content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\n[Anchor](#section) [Mail](mailto:dev@example.com) [Web](https://example.com)\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("[Missing](./missing.md)\n"), 0o644))

	require.NoError(t, AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}}))
}

func TestAuditReadinessAcceptsLocalLinkFragment(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "references"), 0o755))
	content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\n[Spec](./references/spec.md#section)\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "references", "spec.md"), []byte("# Section\n"), 0o644))

	require.NoError(t, AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}}))
}

func TestMarkdownLinkTargetsPreservesCommonMarkSemantics(t *testing.T) {
	content := "[Inline](inline.md) ![Image](image.png) [Reference][spec]\n\n[spec]: reference.md\n"

	require.Equal(t, []string{"inline.md", "image.png", "reference.md"}, markdownLinkTargets(content))
}

func TestAuditMarkdownLinksRejectsMissingRoot(t *testing.T) {
	err := auditMarkdownLinks(filepath.Join(t.TempDir(), "missing"))

	require.Error(t, err)
}

func TestEmbeddedCodeIndexLinkRequiresTextLabel(t *testing.T) {
	require.False(t, isEmbeddedCodeIndexLink(ast.NewLink(), nil))
}

func TestLooksLikeCodeIndexLabel(t *testing.T) {
	require.True(t, looksLikeCodeIndexLabel("\"key\""))
	require.True(t, looksLikeCodeIndexLabel("'key'"))
	require.True(t, looksLikeCodeIndexLabel("42"))
	require.False(t, looksLikeCodeIndexLabel(""))
	require.False(t, looksLikeCodeIndexLabel("key"))
}

func TestCleanAuditPaths(t *testing.T) {
	paths := cleanAuditPaths([]string{"", ".", "../outside", "references/spec.md", "references/../references/spec.md"})

	require.Equal(t, []string{"references/spec.md"}, paths)
}
