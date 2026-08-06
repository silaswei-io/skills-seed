package learn

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/stretchr/testify/require"
)

func TestEnsureSharedLearningContextWritesRuntimeFile(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	run := &learnCurrentProjectRun{
		cont:            &container.Container{SeedPath: seedPath},
		projectName:     "demo",
		projectRoot:     "/repo",
		currentLanguage: "go",
		learningMode:    "normal",
		learningScope:   "flow",
		analysisState: &commandstate.State{
			Agenda: domain.LearningAgenda{Focuses: []domain.EvidenceFocus{{
				ID:         "auth",
				Name:       "Auth",
				EntryPaths: []string{"internal/auth.go"},
			}}},
		},
	}

	err := run.ensureSharedLearningContext()

	require.NoError(t, err)
	require.Equal(t, filepath.Join(seedPath, "runtime", "learn-current", "shared-context.md"), run.sharedLearningContextPath)
	content, err := os.ReadFile(run.sharedLearningContextPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "Auth (`auth`)")
	require.Contains(t, string(content), "internal/auth.go")
}
