package runtimecontext

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserContextLifecycle(t *testing.T) {
	base := context.Background()
	require.Equal(t, base, WithUserContext(base, " \t"))
	require.Empty(t, UserContext(absentContext()))

	ctx := WithUserContext(base, "  release context  ")
	require.Equal(t, "release context", UserContext(ctx))
	require.Empty(t, UserContext(WithoutUserContext(ctx)))
	require.Empty(t, UserContext(WithoutUserContext(absentContext())))
}

func TestSeedPathAndProjectRoot(t *testing.T) {
	base := context.Background()
	require.Equal(t, base, WithSeedPath(base, " \t"))
	require.Empty(t, SeedPath(absentContext()))
	require.Empty(t, ProjectRoot(absentContext()))

	seedPath := filepath.Join("project", ".skills-seed")
	ctx := WithSeedPath(absentContext(), "  "+seedPath+"  ")
	require.Equal(t, seedPath, SeedPath(ctx))
	require.Equal(t, "project", ProjectRoot(ctx))
}

func absentContext() context.Context {
	return nil
}
