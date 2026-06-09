package patterns

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestShowCmdPrintsPatternDatabaseFields(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	createdAt := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 9, 10, 30, 0, 0, time.UTC)
	p := domain.NewPattern("business-create-order", "创建订单", domain.CategoryBusiness)
	p.Source = domain.SourceLearnedCurrent
	p.Confidence = 0.86
	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt
	p.BusinessMethod = &domain.BusinessMethod{
		Name: "CreateOrder",
		CodeLocation: domain.CodeLocation{
			HistoricalLocation: "service/order.ts:10",
			CurrentLocation:    "service/order.ts:20",
			Status:             domain.CodeLocationStatusChanged,
			ChangeKinds:        []domain.CodeLocationChangeKind{domain.CodeLocationChangeMoved, domain.CodeLocationChangeInputsChanged},
			UpdatedAt:          updatedAt,
			Snapshot: &domain.SymbolSnapshot{
				Language:   "typescript",
				Kind:       "method",
				Name:       "createOrder",
				InputTypes: []string{"CreateOrderRequestV2"},
			},
		},
	}

	cont := &container.Container{
		PatternReader: &mocks.MockPatternRepository{
			GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
				return []domain.Pattern{*p}, nil
			},
		},
	}
	cmd := showCmd(cont)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	require.NoError(t, cmd.Execute())

	text := out.String()
	require.Contains(t, text, "business-create-order")
	require.Contains(t, text, "business")
	require.Contains(t, text, "learned_current")
	require.Contains(t, text, "2026-06-01")
	require.Contains(t, text, "2026-06-09")
	require.Contains(t, text, "changed")
	require.Contains(t, text, "service/order.ts:20")
}

func TestShowCmdPrintsSinglePatternDetails(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	p := domain.NewPattern("business-create-order", "创建订单", domain.CategoryBusiness)
	p.Description = "创建订单并写入业务流水"
	p.Rule = "订单创建必须经过领域服务"
	p.BusinessMethod = &domain.BusinessMethod{
		Name: "CreateOrder",
		CodeLocation: domain.CodeLocation{
			HistoricalLocation: "service/order.ts:10",
			CurrentLocation:    "service/order.ts:20",
			Status:             domain.CodeLocationStatusChanged,
			ChangeKinds:        []domain.CodeLocationChangeKind{domain.CodeLocationChangeMoved, domain.CodeLocationChangeInputsChanged},
			Snapshot: &domain.SymbolSnapshot{
				Language:   "typescript",
				Kind:       "method",
				Name:       "createOrder",
				InputTypes: []string{"CreateOrderRequestV2"},
			},
		},
	}

	cont := &container.Container{
		PatternReader: &mocks.MockPatternRepository{
			GetFn: func(ctx context.Context, id string) (*domain.Pattern, error) {
				require.Equal(t, "business-create-order", id)
				return p, nil
			},
		},
	}
	cmd := showCmd(cont)
	cmd.SetArgs([]string{"business-create-order"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	require.NoError(t, cmd.Execute())

	text := out.String()
	require.Contains(t, text, "id")
	require.Contains(t, text, "business-create-order")
	require.Contains(t, text, "description")
	require.Contains(t, text, "创建订单并写入业务流水")
	require.Contains(t, text, "current_location")
	require.Contains(t, text, "service/order.ts:20")
	require.Contains(t, text, "historical_location")
	require.Contains(t, text, "service/order.ts:10")
	require.Contains(t, text, "change_kinds")
	require.Contains(t, text, "moved,inputs_changed")
	require.Contains(t, text, "snapshot_language")
	require.Contains(t, text, "typescript")
	require.Contains(t, text, "snapshot_inputs")
	require.Contains(t, text, "CreateOrderRequestV2")
}

func TestShowCmdPrintsJSON(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	p := domain.NewPattern("business-create-order", "创建订单", domain.CategoryBusiness)
	p.BusinessMethod = &domain.BusinessMethod{
		Name: "CreateOrder",
		CodeLocation: domain.CodeLocation{
			CurrentLocation: "service/order.ts:20",
			Status:          domain.CodeLocationStatusValid,
		},
	}

	cont := &container.Container{
		PatternReader: &mocks.MockPatternRepository{
			GetFn: func(ctx context.Context, id string) (*domain.Pattern, error) {
				return p, nil
			},
		},
	}
	cmd := showCmd(cont)
	cmd.SetArgs([]string{"business-create-order", "--format", "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	require.NoError(t, cmd.Execute())

	var got domain.Pattern
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Equal(t, "business-create-order", got.ID)
	require.Equal(t, "service/order.ts:20", got.BusinessMethod.CodeLocation.CurrentLocation)
	require.Equal(t, domain.CodeLocationStatusValid, got.BusinessMethod.CodeLocation.Status)
}
