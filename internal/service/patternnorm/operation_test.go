package patternnorm

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAndStoreRejectsUnknownOperation(t *testing.T) {
	service := NewService(&mocks.MockPatternRepository{})

	result, err := service.NormalizeAndStore(context.Background(), NormalizeRequest{Operation: Operation("learn_curent")})

	require.ErrorContains(t, err, i18n.GetWithParams("PatternNormUnsupportedOperation", map[string]interface{}{"Operation": Operation("learn_curent")}))
	require.Nil(t, result)
}
