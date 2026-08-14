package learn

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/stretchr/testify/require"
)

func TestClassifyCurrentChangeProfileUsesChangeSemantics(t *testing.T) {
	tests := []struct {
		name    string
		changes *fileanalysis.FileChanges
		want    currentChangeProfile
	}{
		{
			name: "first baseline",
			changes: &fileanalysis.FileChanges{
				AddedOrModified:       []string{"internal/app.go"},
				PreviousAnalyzedCount: 0,
			},
			want: currentChangeProfileInitial,
		},
		{
			name: "incremental change size does not alter profile",
			changes: &fileanalysis.FileChanges{
				AddedOrModified:       []string{"a.go", "b.go", "c.go", "d.go"},
				PreviousAnalyzedCount: 1,
			},
			want: currentChangeProfileIncremental,
		},
		{
			name: "deletion requires reconciliation",
			changes: &fileanalysis.FileChanges{
				AddedOrModified:       []string{"internal/app.go"},
				Deleted:               []string{"internal/obsolete.go"},
				PreviousAnalyzedCount: 1,
			},
			want: currentChangeProfileReconcile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, classifyCurrentChangeProfile(tt.changes))
		})
	}
}

func TestNormalizeCurrentChangeProfileMigratesLegacyState(t *testing.T) {
	require.Equal(t, currentChangeProfileInitial, normalizeCurrentChangeProfile("initial"))
	require.Equal(t, currentChangeProfileReconcile, normalizeCurrentChangeProfile("refactor"))
	require.Equal(t, currentChangeProfileIncremental, normalizeCurrentChangeProfile("minor"))
	require.Equal(t, currentChangeProfileIncremental, normalizeCurrentChangeProfile("unknown"))
}
