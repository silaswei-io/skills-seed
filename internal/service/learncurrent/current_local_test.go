package learncurrent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/codegraph"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestLocalLearningNeedsNoAgentOrSemanticStore(t *testing.T) {
	if _, err := os.Stat("../../../internal/codegraph/assets/runtime.zip"); err != nil && os.Getenv("SKILLS_SEED_CODEGRAPH_HOME") == "" {
		t.Skip("run make bundle for the SDK integration test")
	}
	root := t.TempDir()
	seed := filepath.Join(root, ".skills-seed")
	cfg, err := config.NewRepository(seed, "en-US")
	require.NoError(t, err)
	value := cfg.Get()
	value.Project.RootPath = root
	require.NoError(t, cfg.Update(value))
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package sample\nfunc Reuse() {}\n"), 0600))
	// Agent、语义仓储、文件分析指纹均故意不注入；本地路径不得访问它们。
	cont := &container.Container{SeedPath: seed, ConfigRepo: cfg}
	result, err := Run(context.Background(), cont)
	require.NoError(t, err)
	require.True(t, result.Summary.NavigationUpdated)
	require.Zero(t, result.Summary.PatternsFound)
	index, err := codegraph.Load(root)
	require.NoError(t, err)
	require.Len(t, index.Files, 1)
	result, err = Run(context.Background(), cont)
	require.NoError(t, err)
	require.False(t, result.Summary.NavigationUpdated)
	// 权威规则变化也需要重新投影，不能因源码没变而跳过生成。
	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Do not run deployment commands."), 0600))
	result, err = Run(context.Background(), cont)
	require.NoError(t, err)
	require.True(t, result.Summary.NavigationUpdated)
}

func TestSemanticLearningRequiresExplicitScope(t *testing.T) {
	cfg, err := config.NewRepository(t.TempDir(), "en-US")
	require.NoError(t, err)
	_, err = RunWithFlags(context.Background(), &container.Container{ConfigRepo: cfg}, Flags{Semantic: true})
	require.Error(t, err)
}

func TestLocalScanFinishesBeforeSemanticProgressStarts(t *testing.T) {
	if _, err := os.Stat("../../../internal/codegraph/assets/runtime.zip"); err != nil && os.Getenv("SKILLS_SEED_CODEGRAPH_HOME") == "" {
		t.Skip("run make bundle for the SDK integration test")
	}
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	cont.PatternReader = cont.PatternRepo
	mock := cont.Agent.(*mocks.MockAgent)
	cont.AnalyzerSvc = analyzer.NewAnalyzerService(mock, cont.ConfigRepo)
	cont.PatternNormSvc = patternnorm.NewServiceWithNormalizer(cont.PatternRepo, mock)
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, root, "main.go", "package main\nfunc Decide() {}\n")
	gitAddAll(t, root)
	analyses := 0
	mock.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		analyses++
		return &agent.AnalyzeCurrentCodebaseBatchResult{Assessment: "resolved", Focuses: []agent.AnalyzeCurrentEvidenceResult{{
			FocusID:  req.Focuses[0].EvidenceFocus.ID,
			Patterns: []domain.Pattern{*admittedLearnCurrentPatternForTest("decision", "Decision", domain.CategoryBusiness, "main.go")},
		}}}, nil
	}
	mock.ReviewKnowledgeFn = func(ctx context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		return acceptKnowledgeCandidates(req.Candidates), nil
	}
	active := ""
	var started, completed []string
	_, err := runLearnCurrentProjectWithOptions(context.Background(), cont, learnCurrentProjectOptions{
		focusPaths: []string{"main.go"},
		onStepStart: func(label string) {
			require.Empty(t, active, "new step %q overlaps active step %q", label, active)
			active = label
			started = append(started, label)
		},
		onStepComplete: func(label string) {
			require.Equal(t, active, label)
			active = ""
			completed = append(completed, label)
		},
	})
	require.NoError(t, err)
	require.Empty(t, active)
	require.Len(t, started, learnCurrentProjectStepTotal)
	require.Equal(t, started, completed)
	require.Equal(t, 1, analyses)
}
