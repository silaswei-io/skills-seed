package generator

import (
	"context"
	"os"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/metadata"
)

func (s *GeneratorService) prepareProjectSkillsFingerprint(ctx context.Context, resolvedOutputPath string, patterns []domain.Pattern, insights map[string]domain.PatternInsight, profile *domain.ProjectProfile) (*domain.InputFingerprintDecision, error) {
	tracker := s.fileAnalysisTracker()
	if tracker == nil {
		return nil, nil
	}
	return domain.PrepareInputFingerprint(ctx, tracker, projectSkillsFingerprintScope(), "project-skills.json", projectSkillsFingerprintInput{
		Kind:                "project_skills_generation",
		ProgramVersion:      metadata.ProgramVersion,
		PromptTemplatesHash: metadata.HashOrUnavailable(metadata.PromptTemplatesHash(embedfs.FS)),
		SkillsTemplatesHash: metadata.HashOrUnavailable(metadata.SkillsTemplatesHash(embedfs.FS)),
		OutputPath:          filepath.ToSlash(resolvedOutputPath),
		ProjectConfig:       s.configRepo.GetProjectConfig(),
		AgentConfig:         s.configRepo.GetAgentConfig(),
		SkillsConfig:        s.configRepo.GetSkillsConfig(),
		Patterns:            patterns,
		PatternInsights:     insights,
		Profile:             profile,
	})
}

func (s *GeneratorService) fileAnalysisTracker() domain.FileAnalysisTracker {
	tracker, _ := s.patternRepo.(domain.FileAnalysisTracker)
	return tracker
}

func projectSkillsFingerprintScope() domain.FileAnalysisScope {
	return domain.FileAnalysisScope{ProjectID: "__skills__", ScopePath: "project"}
}

func projectSkillsOutputExists(outputPath string, patterns []domain.Pattern) bool {
	if outputPath == "" {
		return false
	}
	requiredFiles := []string{
		"SKILL.md",
		filepath.Join("references", "project-overview.md"),
		filepath.Join("references", "project-spec.md"),
	}
	for _, path := range requiredFiles {
		if _, err := os.Stat(filepath.Join(outputPath, path)); err != nil {
			return false
		}
	}
	if info, err := os.Stat(filepath.Join(outputPath, "references", "patterns")); err != nil || !info.IsDir() {
		return false
	}
	for _, category := range categoryNamesWithPatterns(patterns) {
		if _, err := os.Stat(filepath.Join(outputPath, "references", "patterns", category+".md")); err != nil {
			return false
		}
	}
	return true
}
