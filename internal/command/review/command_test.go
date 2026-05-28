package review

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestImportCmdImportsReviewCommentsFromJSONFile(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	path := filepath.Join(t.TempDir(), "review-comments.json")
	require.NoError(t, os.WriteFile(path, []byte(`[
		{
			"id": "c-1",
			"provider": "local",
			"review_id": "review-1",
			"commit": "abc123",
			"file": "internal/service/checker/service.go",
			"line": 84,
			"author": "reviewer",
			"body": "wrap checker errors",
			"resolved": true,
			"created_at": "2026-05-28T09:02:00Z"
		}
	]`), 0600))

	var imported []domain.ReviewComment
	cont := &container.Container{
		ReviewRepo: &mocks.MockReviewRepository{
			ImportReviewCommentsFn: func(ctx context.Context, comments []domain.ReviewComment) error {
				imported = comments
				return nil
			},
		},
	}
	cmd := importCmd(cont)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--from-file", path})

	require.NoError(t, cmd.Execute())
	require.Len(t, imported, 1)
	require.Equal(t, "c-1", imported[0].ID)
	require.Equal(t, "internal/service/checker/service.go", imported[0].File)
	require.Equal(t, 84, imported[0].Line)
	require.Contains(t, out.String(), "Imported 1 review comments")
}

func TestStatsCmdPrintsReviewStats(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cont := &container.Container{
		ReviewRepo: &mocks.MockReviewRepository{
			GetReviewStatsFn: func(ctx context.Context, lineWindow int) (domain.ReviewStats, error) {
				require.Equal(t, 3, lineWindow)
				return domain.ReviewStats{
					TotalComments:     2,
					PreventedComments: 1,
					MissedComments:    1,
					MatchedPatterns: []domain.ReviewMatchedPatternStats{
						{PatternID: "p-error-wrap", CommentCount: 1},
					},
				}, nil
			},
		},
	}
	cmd := statsCmd(cont)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	require.NoError(t, cmd.Execute())

	text := out.String()
	require.Contains(t, text, "TOTAL")
	require.Contains(t, text, "PREVENTED")
	require.Contains(t, text, "MISSED")
	require.Contains(t, text, "2")
	require.Contains(t, text, "p-error-wrap")
}
