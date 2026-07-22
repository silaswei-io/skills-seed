package generator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestBuildBusinessMethodIndexKeepsEveryEntryAndGroupsBySource(t *testing.T) {
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

	require.Equal(t, 2, index.Total)
	require.Len(t, index.Groups, 2)
	groups := make(map[string]string, len(index.Groups))
	for _, group := range index.Groups {
		require.Len(t, group.Methods, 1)
		groups[group.ID] = group.Methods[0].Name
	}
	require.Equal(t, "GenerateCurl", groups["gen_api_curl_test"])
	require.Equal(t, "ApplyTransition", groups["order"])
}
