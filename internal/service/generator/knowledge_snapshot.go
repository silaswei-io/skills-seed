package generator

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
)

// verifiedKnowledgeSnapshot 是生成阶段唯一允许消费的已核验知识输入。
type verifiedKnowledgeSnapshot struct {
	RenderProfile *domain.ProjectProfile
	Patterns      []domain.Pattern
	Spec          *domain.ProjectSpec
	GoTests       sourcecode.GoTestInventory
}

func (s *GeneratorService) buildVerifiedKnowledgeSnapshot(ctx context.Context, profile *domain.ProjectProfile, patterns []domain.Pattern, projectRoot string) (verifiedKnowledgeSnapshot, error) {
	profile = cleanProjectProfile(profile)
	profile, patterns, err := sanitizeGenerationInputs(ctx, profile, patterns, projectRoot, s.symbolResolver)
	if err != nil {
		return verifiedKnowledgeSnapshot{}, err
	}
	goTests, err := sourcecode.DiscoverGoTests(projectRoot)
	if err != nil {
		return verifiedKnowledgeSnapshot{}, err
	}
	return verifiedKnowledgeSnapshot{
		RenderProfile: profileForSkillTemplates(profile, patterns),
		Patterns:      patterns,
		Spec:          s.projectSpecFromProfileAndPatterns(profile, patterns, config.WorkspaceProjectConfig{}),
		GoTests:       goTests,
	}, nil
}
