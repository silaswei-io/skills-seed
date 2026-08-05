package pathx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanRelative(t *testing.T) {
	require.Equal(t, "internal/Auth/Login.go", CleanRelative("./internal/Auth/Login.go"))
	require.Equal(t, "internal/auth/login.go", CleanRelative(`internal\auth\login.go`))
	require.Equal(t, "internal/auth/login.go", CleanRelative("internal/auth/../auth/login.go"))
	require.Empty(t, CleanRelative(""))
	require.Empty(t, CleanRelative("."))
	require.Empty(t, CleanRelative("../outside.go"))
	require.Empty(t, CleanRelative("/tmp/outside.go"))
	require.Empty(t, CleanRelative("C:/repo/outside.go"))
	require.Empty(t, CleanRelative(`C:\repo\outside.go`))
}

func TestCleanRelativeList(t *testing.T) {
	got := CleanRelativeList([]string{
		"./b.go",
		"a.go",
		"b.go",
		"../outside.go",
	})

	require.Equal(t, []string{"a.go", "b.go"}, got)
}

func TestCleanEvidenceLocationPath(t *testing.T) {
	require.Equal(t, "internal/Auth/Login.go", CleanEvidenceLocationPath("`internal/Auth/Login.go:42`"))
	require.Equal(t, "internal/auth/login.go", CleanEvidenceLocationPath(`internal\auth\login.go:42`))
	require.Equal(t, "internal/auth/login.go", CleanEvidenceLocationPath("internal/auth/login.go"))
	require.Empty(t, CleanEvidenceLocationPath("../outside.go:9"))
}

func TestCleanEvidenceLocation(t *testing.T) {
	require.Equal(t, "internal/Auth/Login.go:42", CleanEvidenceLocation("`internal/Auth/Login.go:42`"))
	require.Equal(t, "internal/auth/login.go", CleanEvidenceLocation("internal/auth/login.go"))
	require.Empty(t, CleanEvidenceLocation("../outside.go:9"))
}
