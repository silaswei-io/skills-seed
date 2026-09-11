package profile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	"github.com/stretchr/testify/require"
)

func TestCommandMetadataAndNilContainer(t *testing.T) {
	cmd := Cmd(nil)
	require.Equal(t, "profile", cmd.Use)
	require.Len(t, cmd.Commands(), 1)
	cmd.SetArgs([]string{"show"})
	require.Error(t, cmd.Execute())
}

func TestShowProfile(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	repo := profilestore.NewRepository(seedPath)
	profile := &domain.ProjectProfile{
		ProjectName:     "demo",
		Language:        "go",
		Summary:         "summary",
		GeneratedAt:     "2026-09-11",
		Frameworks:      []string{"cobra"},
		Dependencies:    []string{"testify"},
		KeyModules:      []domain.ModuleInfo{{Name: "command"}},
		BusinessMethods: []domain.BusinessMethod{{Name: "sync"}},
	}
	require.NoError(t, repo.Save(context.Background(), profile))
	cont := &container.Container{ProfileRepo: repo}

	require.NoError(t, showProfile(cont))
	cmd := Cmd(cont)
	cmd.SetArgs([]string{"show"})
	require.NoError(t, cmd.Execute())
}

func TestShowProfileReportsMissingAndMalformedData(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	cont := &container.Container{ProfileRepo: profilestore.NewRepository(seedPath)}
	require.Error(t, showProfile(cont))

	path := cont.ProfileRepo.Path()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o644))
	require.Error(t, showProfile(cont))
}
