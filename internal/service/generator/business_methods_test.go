package generator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestBuildBusinessMethodIndexKeepsEveryEntryAndGroupsBySource(t *testing.T) {
	index := buildBusinessMethodIndex([]domain.BusinessMethod{
		{
			Name:          "GenerateCurl",
			CodeLocation:  domain.CodeLocation{CurrentLocation: "tools/gen_api_curl_test/main.go:20"},
			Description:   "generates API curl examples for tests",
			Usage:         "when producing API examples",
			Type:          "common",
			Function:      "func GenerateCurl() error",
			Prerequisites: "loaded API definitions",
			Returns:       "an error when generation fails",
		},
		{
			Name:          "ApplyTransition",
			CodeLocation:  domain.CodeLocation{CurrentLocation: "internal/service/order/transition.go:42"},
			Description:   "applies domain workflow state transition and persists the result",
			Usage:         "business flow orchestration",
			Type:          "domain",
			Function:      "func (s *OrderService) ApplyTransition(ctx context.Context, id string) error",
			Prerequisites: "loaded order and transition context",
			Returns:       "an error when validation or persistence fails",
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

func TestRouteableBusinessMethodsDropsIncompleteContracts(t *testing.T) {
	complete := domain.BusinessMethod{
		Name:          "BuildServiceDeps",
		CodeLocation:  domain.CodeLocation{CurrentLocation: "plugins/demo/service/deps.go:20"},
		Description:   "builds plugin service dependencies",
		Usage:         "plugin initialization and service extension",
		Function:      "func BuildServiceDeps(ctx context.Context) (*ServiceDeps, error)",
		Prerequisites: "initialized plugin configuration",
		Returns:       "service dependencies or an initialization error",
	}
	incomplete := complete
	incomplete.Name = "ThinGeneratedWrapper"
	incomplete.CodeLocation.CurrentLocation = "internal/model/item_gen.go:10"
	incomplete.Prerequisites = ""

	methods := routeableBusinessMethods([]domain.BusinessMethod{complete, incomplete})

	require.Len(t, methods, 1)
	require.Equal(t, "BuildServiceDeps", methods[0].Name)
}
