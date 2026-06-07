package workspace

import (
	"context"
	"os"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/metadata"
)

func (g *WorkspaceGenerator) prepareWorkspaceSkillsFingerprint(ctx context.Context, outputPath string, data workspaceSkillTemplateData) (*domain.InputFingerprintDecision, error) {
	tracker := g.fileAnalysisTracker()
	if tracker == nil {
		return nil, nil
	}
	return domain.PrepareInputFingerprint(ctx, tracker, workspaceSkillsFingerprintScope(), "workspace-skills.json", workspaceSkillsFingerprintInput{
		Kind:                "workspace_skills_generation",
		ProgramVersion:      metadata.ProgramVersion,
		SkillsTemplatesHash: metadata.HashOrUnavailable(metadata.SkillsTemplatesHash(embedfs.FS)),
		OutputPath:          filepath.ToSlash(outputPath),
		TemplateData:        data,
	})
}

func workspaceSkillsFingerprintScope() domain.FileAnalysisScope {
	return domain.FileAnalysisScope{ProjectID: "__skills__", ScopePath: "workspace"}
}

func workspaceRootSkillsOutputExists(outputPath string) bool {
	if outputPath == "" {
		return false
	}
	requiredFiles := []string{
		"SKILL.md",
		filepath.Join("references", "workspace-overview.md"),
		filepath.Join("references", "cross-project-rules.md"),
	}
	for _, path := range requiredFiles {
		if _, err := os.Stat(filepath.Join(outputPath, path)); err != nil {
			return false
		}
	}
	return true
}
