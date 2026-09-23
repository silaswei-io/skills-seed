package learncurrent

import (
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
)

type learnCurrentFocusResult struct {
	evidence          domain.LearningEvidence
	conversation      agent.Conversation
	index             int
	focus             domain.EvidenceFocus
	patterns          []domain.Pattern
	retiredPatternIDs []string
	refreshRecommend  agent.ProfileRefreshRecommendation
	completed         bool
}

// checkpoint 只表示源码分析完成；只有独立审查结果能设置 Reviewed。
func (result learnCurrentFocusResult) checkpoint() commandstate.FocusKnowledgeCheckpoint {
	return commandstate.FocusKnowledgeCheckpoint{
		Focus: result.focus, Evidence: result.evidence.Clone(),
		Patterns:          append([]domain.Pattern(nil), result.patterns...),
		RetiredPatternIDs: appendUniquePatternIDs(nil, result.retiredPatternIDs...),
	}
}

type learnCurrentBatch struct {
	index   int
	focuses []indexedEvidenceFocus
}

type indexedEvidenceFocus struct {
	index int
	focus domain.EvidenceFocus
}

type knowledgeReviewTask struct {
	index int
	unit  commandstate.FocusKnowledgeCheckpoint
}
