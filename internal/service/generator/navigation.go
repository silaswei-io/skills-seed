package generator

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/skillgen"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
)

// appendNavigation 只生成查询入口，不把结构图复制成大量目录和符号页面。
func appendNavigation(p *skillgen.Plan, index *codegraph.Index, methods []domain.BusinessMethod) {
	p.AddFile("references/code-navigation.md", skillgen.RelativeTemplate, "project/references/code-navigation", struct {
		Files, Unindexed   int
		AuthorityPaths     []string
		HasBusinessMethods bool
	}{len(index.Files), len(index.Unindexed), index.AuthorityPaths, len(methods) > 0})
}

// localKnowledge 只投影证据指纹仍有效的语义知识；不修改原知识或语义学习基线。
func (s *GeneratorService) localKnowledge(ctx context.Context, root string, profile *domain.ProjectProfile, patterns []domain.Pattern) (*domain.ProjectProfile, []domain.Pattern, error) {
	current := make(map[string]bool)
	profileCurrent := false
	if s.fileTracker != nil {
		changes, err := fileanalysis.PrepareCurrentChanges(ctx, s.fileTracker, s.configRepo, root, root, domain.FileAnalysisScope{}, nil)
		if err != nil {
			return nil, nil, err
		}
		for _, file := range changes.Unchanged {
			current[file] = true
		}
		revision, err := analyzer.EngineeringKnowledgeRevision(root)
		if err != nil {
			return nil, nil, err
		}
		profileCurrent = changes.PreviousAnalyzedCount > 0 && !changes.HasChanges() && profile != nil && profile.AuthorityRevision == revision
	}
	if !profileCurrent {
		profile = s.fallbackProjectProfile()
	}
	retained := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern.AllowsHardConstraint() {
			retained = append(retained, pattern)
			continue
		}
		valid := len(pattern.EvidenceLocations) > 0
		for _, evidence := range pattern.EvidenceLocations {
			if evidence.ContentHash != "" {
				hash, err := sourcecode.EvidenceDigest(root, evidence.Path)
				if err != nil || hash != evidence.ContentHash {
					valid = false
					break
				}
				continue
			}
			if !current[evidence.Path] {
				valid = false
				break
			}
		}
		if valid && (pattern.Source != domain.SourceLearnedCurrent || pattern.IsSemanticKnowledge()) {
			retained = append(retained, pattern)
		}
	}
	return profile, retained, nil
}
