package patterns

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestStatsCmdPrintsPatternMetricsAndHits(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	p := domain.NewPattern("domain-error-wrap", "领域错误包装", domain.CategoryError)
	p.Confidence = 0.8
	p.Metrics = domain.PatternMetrics{
		SpecificityScore: 0.72,
		GenericPenalty:   0.1,
		EffectiveScore:   0.66,
		EvidenceCount:    3,
	}
	lastHit := time.Date(2026, 5, 28, 10, 30, 0, 0, time.UTC)
	cont := &container.Container{
		PatternStats: &mocks.MockPatternStatsRepository{
			GetPatternHitStatsFn: func(ctx context.Context) ([]domain.PatternHitStats, error) {
				return []domain.PatternHitStats{
					{Pattern: *p, HitCount: 2, LastHitAt: lastHit},
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
	require.Contains(t, text, "domain-error-wrap")
	require.Contains(t, text, "error")
	require.Contains(t, text, "0.72")
	require.Contains(t, text, "0.66")
	require.Contains(t, text, "2")
	require.Contains(t, text, "2026-05-28")
}
