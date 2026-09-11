package skilloutput

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	markdownast "github.com/yuin/goldmark/ast"
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

func TestAuditReadinessIgnoresExtremeCodeLikeExpressions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{name: "single generic argument", expression: "factory.New[Client](ctx)"},
		{name: "multiple generic arguments", expression: "factory.New[Key,Value](ctx)"},
		{name: "multiple call arguments", expression: "factory.New[Client](ctx,opts)"},
		{name: "nested call argument", expression: "factory.New[Client](resolve(ctx))"},
		{name: "numeric index invocation", expression: "handlers[0](ctx)"},
		{name: "double quoted index invocation", expression: `params["deadline"](RFC3339Nano)`},
		{name: "single quoted index invocation", expression: "params['deadline'](RFC3339Nano)"},
		{name: "chained index invocation", expression: `registry.Lookup()["handler"](ctx)`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\n" + test.expression + "\n"
			require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))

			require.NoError(t, AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}}))
		})
	}
}

func TestAuditReadinessStillChecksAdjacentRealLinks(t *testing.T) {
	tests := []struct {
		name string
		link string
	}{
		{name: "word label", link: "prefix[Missing](./missing.md)"},
		{name: "generic shaped label", link: "prefix[Client](./missing.md)"},
		{name: "numeric label", link: "prefix[0](./missing.md)"},
		{name: "quoted label", link: `prefix["key"](./missing.md)`},
		{name: "formatted label", link: "prefix[*Client*](./missing.md)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\n" + test.link + "\n"
			require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))

			err := AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}})

			require.ErrorContains(t, err, "broken link")
			require.ErrorContains(t, err, "./missing.md")
		})
	}
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

func TestAuditReadinessFindsBrokenLinkAtEndOfLargeDocument(t *testing.T) {
	root := t.TempDir()
	var content strings.Builder
	content.WriteString("---\nname: demo-dev\ndescription: Demo skill\n---\n\n")
	for range 4096 {
		content.WriteString("factory.New[Client](ctx)\n")
	}
	content.WriteString("\n[Missing](./missing.md)\n")
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content.String()), 0o644))

	err := AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}})

	require.ErrorContains(t, err, "broken link")
	require.ErrorContains(t, err, "./missing.md")
}

func TestAuditReadinessRejectsMarkdownSymlinkEscapingSkillRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 创建符号链接需要额外权限")
	}
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	require.NoError(t, os.WriteFile(outside, []byte("outside\n"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "outside.md")))
	content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\n[Outside](./outside.md)\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))

	err := AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}})

	require.ErrorContains(t, err, "escapes skill root")
}

func TestAuditReadinessAcceptsMarkdownSymlinkWithinSkillRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 创建符号链接需要额外权限")
	}
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "references"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "references", "spec.md"), []byte("# Spec\n"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(root, "references", "spec.md"), filepath.Join(root, "spec.md")))
	content := "---\nname: demo-dev\ndescription: Demo skill\n---\n\n[Spec](./spec.md)\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))

	require.NoError(t, AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}}))
}

func TestAuditReadinessReportsUnreadableMarkdownEntry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 创建符号链接需要额外权限")
	}
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\nname: demo-dev\ndescription: Demo skill\n---\n"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(root, "missing-target"), filepath.Join(root, "broken.md")))

	require.Error(t, AuditReadiness(root, ReadinessRequirements{ExpectedFiles: []string{"SKILL.md"}}))
}

func TestMarkdownLinkTargetsPreservesCommonMarkSemantics(t *testing.T) {
	content := "[Inline](inline.md) ![Image](image.png) [Reference][spec]\n\n[spec]: reference.md\n"

	require.Equal(t, []string{"inline.md", "image.png", "reference.md"}, markdownLinkTargets(content))
}

func TestAuditMarkdownLinksRejectsMissingRoot(t *testing.T) {
	err := auditMarkdownLinks(filepath.Join(t.TempDir(), "missing"))

	require.Error(t, err)
}

func TestEmbeddedSourceExpressionLinkRequiresTextLabel(t *testing.T) {
	require.False(t, isEmbeddedSourceExpressionLink(markdownast.NewLink(), nil))
}

func TestCleanAuditPaths(t *testing.T) {
	paths := cleanAuditPaths([]string{"", ".", "../outside", "references/spec.md", "references/../references/spec.md"})

	require.Equal(t, []string{"references/spec.md"}, paths)
}
