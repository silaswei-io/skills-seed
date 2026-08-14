package generator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge"
	"github.com/silaswei-io/skills-seed/internal/service/repositoryscopeconfig"
)

// verifiedKnowledgeSnapshot 是生成阶段唯一允许消费的已核验知识输入。
type verifiedKnowledgeSnapshot struct {
	RenderProfile *domain.ProjectProfile
	Patterns      []domain.Pattern
	Spec          *domain.ProjectSpec
}

func knowledgeSnapshotHash(snapshot verifiedKnowledgeSnapshot) string {
	type hashInput struct {
		Profile  *domain.ProjectProfile `json:"profile"`
		Patterns []domain.Pattern       `json:"patterns"`
		Spec     *domain.ProjectSpec    `json:"spec"`
	}
	patterns := append([]domain.Pattern(nil), snapshot.Patterns...)
	sort.Slice(patterns, func(i, j int) bool { return patterns[i].ID < patterns[j].ID })
	data, _ := json.Marshal(hashInput{
		Profile:  snapshot.RenderProfile,
		Patterns: patterns,
		Spec:     snapshot.Spec,
	})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *GeneratorService) buildVerifiedKnowledgeSnapshot(ctx context.Context, profile *domain.ProjectProfile, patterns []domain.Pattern, projectRoot string) (verifiedKnowledgeSnapshot, error) {
	profile = cleanProjectProfile(profile)
	scope := repositoryscopeconfig.KnowledgeScope(s.configRepo, projectRoot)
	profile, patterns, err := knowledge.VerifyProjectKnowledge(ctx, profile, patterns, projectRoot, s.symbolResolver, scope)
	if err != nil {
		return verifiedKnowledgeSnapshot{}, err
	}
	renderPatterns := patternsForSkillTemplates(patterns)
	renderProfile := profileForSkillTemplates(profile, renderPatterns)
	spec := domain.NewProjectSpecFromProfile(profile, domain.WorkspaceProjectOverride{})
	if spec != nil {
		spec.GeneratedAt = knowledgeUpdatedAt(profile, patterns)
	}
	return verifiedKnowledgeSnapshot{
		RenderProfile: renderProfile,
		Patterns:      renderPatterns,
		Spec:          spec,
	}, nil
}
