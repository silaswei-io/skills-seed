package learncurrent

import (
	"context"
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
)

// learnDeps 是运行期不可变的外部依赖。
type learnDeps struct {
	cont      *container.Container
	opts      learnCurrentProjectOptions
	stateRepo *commandstate.Repository
	ctx       context.Context
	startedAt time.Time
	steps     *commandutil.ConsoleStepRunner
}

// learnProjectCtx 描述本轮项目身份与画像刷新意图。
type learnProjectCtx struct {
	projectRoot        string
	projectName        string
	currentLanguage    string
	learningMode       string
	resolvedFocusPaths []string
	refreshProfile     bool
	existingProfile    *domain.ProjectProfile
}

// learnChangeCtx 描述增量候选与恢复状态。
type learnChangeCtx struct {
	incrementalChanges  *fileanalysis.FileChanges
	effectiveFocusPaths []string
	selectedFiles       []domain.FileInfo
	selectionSummary    fileSelectionSummary
	selectionPlan       currentFileSelectionPlan
	stateSession        *currentStateSession
	stateInvalidated    bool
	resumeSummary       *learnCurrentResumeSummary
	changeProfile       currentChangeProfile
}

// learnAgendaCtx 描述议程与分析运行态。
type learnAgendaCtx struct {
	analysisState             *commandstate.State
	plannedFocuses            []domain.EvidenceFocus
	codebaseRunContext        *analyzer.CodebaseRunContext
	sharedLearningContextPath string
	// conversations 仅在当前进程内保存焦点会话，检查点不会持久化模型会话标识。
	conversations map[string]agent.Conversation
}

// learnKnowledgeCtx 描述审查后的知识与入库结果。
type learnKnowledgeCtx struct {
	patterns                  []domain.Pattern
	retiredPatternIDs         []string
	focusKnowledge            []commandstate.FocusKnowledgeCheckpoint
	profileRefreshRecommended agent.ProfileRefreshRecommendation
	savedCount                int
	retiredCount              int
	dropped                   []patternnorm.Drop
}

// learnCurrentProjectRun 是项目级 learn current 的可恢复会话。
// 字段按依赖、项目、变更、议程、知识分组，避免扁平 God Object 继续膨胀。
type learnCurrentProjectRun struct {
	learnDeps
	learnProjectCtx
	learnChangeCtx
	learnAgendaCtx
	learnKnowledgeCtx

	progressDetailMu           sync.Mutex
	fileSelectionSummaryLogged bool
}
