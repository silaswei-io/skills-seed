package skilloutput

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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
