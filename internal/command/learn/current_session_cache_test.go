package learn

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestCurrentLearningSessionCacheStoresRuntimeStep(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	ctx := context.Background()
	cache := currentLearningSessionCache{
		AgentName:      "codex",
		SessionID:      "thread-123",
		Step:           learningStagePackAnalysis,
		InvocationHash: "hash",
	}

	require.NoError(t, saveCurrentLearningSessionCache(ctx, seedPath, commandStateLearnCurrent, cache))
	loaded, err := loadCurrentLearningSessionCache(ctx, seedPath, commandStateLearnCurrent)

	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, "codex", loaded.AgentName)
	require.Equal(t, "thread-123", loaded.SessionID)
	require.Equal(t, learningStagePackAnalysis, loaded.Step)
	require.Equal(t, "hash", loaded.InvocationHash)
	require.NotEmpty(t, loaded.UpdatedAt)
	require.True(t, loaded.matches("codex", "hash"))
	require.False(t, loaded.matches("claude", "hash"))
	require.False(t, loaded.matches("codex", "other"))
	require.Contains(t, filepath.ToSlash(currentLearningSessionCachePath(seedPath, commandStateLearnCurrent)), ".skills-seed/runtime/learning-sessions")

	require.NoError(t, clearCurrentLearningSessionCache(seedPath, commandStateLearnCurrent))
	_, err = os.Stat(currentLearningSessionCachePath(seedPath, commandStateLearnCurrent))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestStartLearningSessionResumesRuntimeCacheForRestoredState(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	mockAgent := cont.Agent.(*mocks.MockAgent)
	run := &learnCurrentProjectRun{
		ctx:             context.Background(),
		cont:            cont,
		stateRepo:       learnCurrentStateRepo(cont.SeedPath, ""),
		projectName:     "demo",
		projectRoot:     cont.ConfigRepo.GetProjectConfig().RootPath,
		currentLanguage: "go",
		opts:            learnCurrentProjectOptions{profileMode: learnCurrentProfileSkip},
		stateSession:    &currentStateSession{State: &commandstate.State{}},
	}
	hash := run.currentStateInvocationHash()
	require.NoError(t, saveCurrentLearningSessionCache(context.Background(), cont.SeedPath, commandStateLearnCurrent, currentLearningSessionCache{
		AgentName:      "mock",
		SessionID:      "session-abc",
		Step:           learningStagePackAnalysis,
		InvocationHash: hash,
	}))

	var got agent.LearningSessionRequest
	mockAgent.StartLearningSessionFn = func(ctx context.Context, req agent.LearningSessionRequest) (agent.LearningSession, error) {
		got = req
		return testLearningSession{id: req.ResumeSessionID}, nil
	}

	require.NoError(t, run.startLearningSession(learningStagePackAnalysis))
	require.Equal(t, "session-abc", got.ResumeSessionID)
	require.Equal(t, learningStagePackAnalysis, got.Stage)
	require.Equal(t, learningStagePackAnalysis, run.learningSessionCache.Step)
}

type testLearningSession struct {
	id string
}

func (s testLearningSession) SessionID() string {
	return s.id
}

func (s testLearningSession) SelectLearningCandidates(context.Context, *agent.SelectLearningCandidatesRequest) (*agent.SelectLearningCandidatesResult, error) {
	return &agent.SelectLearningCandidatesResult{}, nil
}

func (s testLearningSession) PlanLearningAgenda(context.Context, *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error) {
	return &agent.PlanLearningAgendaResult{}, nil
}

func (s testLearningSession) AnalyzeCurrentCodebaseBatch(context.Context, *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	return &agent.AnalyzeCurrentCodebaseBatchResult{}, nil
}

func (s testLearningSession) AnalyzeCurrentDeltaBatch(context.Context, *agent.AnalyzeCurrentDeltaBatchRequest) (*agent.AnalyzeCurrentDeltaBatchResult, error) {
	return &agent.AnalyzeCurrentDeltaBatchResult{}, nil
}

func (s testLearningSession) RefreshProjectProfile(context.Context, *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	return &agent.AnalyzeProjectResult{}, nil
}

func (s testLearningSession) CuratePatterns(context.Context, *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
	return &agent.CuratePatternsResult{}, nil
}

func (s testLearningSession) Close(context.Context) error {
	return nil
}
