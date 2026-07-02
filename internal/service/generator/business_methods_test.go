package generator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestBuildBusinessMethodIndexPrioritizesBusinessEntries(t *testing.T) {
	index := buildBusinessMethodIndex([]domain.BusinessMethod{
		{
			Name:         "GenerateCurl",
			CodeLocation: domain.CodeLocation{CurrentLocation: "tools/gen_api_curl_test/main.go:20"},
			Description:  "generates API curl examples for tests",
			Type:         "common",
			Function:     "func GenerateCurl() error",
		},
		{
			Name:         "ApplyTransition",
			CodeLocation: domain.CodeLocation{CurrentLocation: "internal/service/order/transition.go:42"},
			Description:  "applies domain workflow state transition and persists the result",
			Usage:        "business flow orchestration",
			Type:         "domain",
			Function:     "func (s *OrderService) ApplyTransition(ctx context.Context, id string) error",
		},
	}, "en-US")

	require.Len(t, index.Sections, 2)
	require.Equal(t, "Business & Orchestration Entry Points", index.Sections[0].Title)
	require.Len(t, index.Sections[0].Groups, 1)
	require.Equal(t, "ApplyTransition", index.Sections[0].Groups[0].Methods[0].Name)
	require.Equal(t, "Supporting & Utility Entry Points", index.Sections[1].Title)
	require.Len(t, index.Sections[1].Groups, 1)
	require.Equal(t, "GenerateCurl", index.Sections[1].Groups[0].Methods[0].Name)
}
