package learncurrent

import (
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/stretchr/testify/require"
)

func TestLearnRunObserverTracksStagesSkipsAndAgentCalls(t *testing.T) {
	counter := runtimecontext.NewAgentCallCounter()
	obs := newLearnRunObserver(counter)

	obs.startStage(stageAnalyze)
	time.Sleep(2 * time.Millisecond)
	obs.endStage(stageAnalyze)
	obs.noteSkip(skipReviewEmpty)
	obs.noteAnalysisMode("delta")
	counter.Add("AnalyzeCurrentCodebaseBatch")
	counter.Add("AnalyzeCurrentCodebaseBatch")

	result := &learnCurrentProjectResult{
		patternsCount: 0,
		focusCount:    2,
		learningMode:  "normal",
		changeProfile: "small",
	}
	metrics := obs.buildJournalMetrics(result, 40*time.Millisecond)
	require.NotNil(t, metrics)
	require.Equal(t, int64(40), metrics.WallMs)
	require.Equal(t, 2, metrics.Focuses)
	require.Equal(t, "delta", metrics.AnalysisMode)
	require.Equal(t, []string{skipReviewEmpty}, metrics.SkippedStages)
	require.Equal(t, 2, metrics.AgentCallTotal)
	require.Equal(t, 2, metrics.AgentCalls["AnalyzeCurrentCodebaseBatch"])
	require.Contains(t, metrics.StageMs, stageAnalyze)
	require.GreaterOrEqual(t, metrics.StageMs[stageAnalyze], int64(1))
}

func TestDropReasonCodeCounts(t *testing.T) {
	require.Nil(t, dropReasonCodeCounts(nil))
	counts := dropReasonCodeCounts([]patternnorm.Drop{
		{ReasonCode: patternnorm.DropUnsupportedEvidence},
		{ReasonCode: patternnorm.DropUnsupportedEvidence},
		{ReasonCode: ""},
	})
	require.Equal(t, 2, counts[string(patternnorm.DropUnsupportedEvidence)])
	require.Equal(t, 1, counts["unspecified"])
}
