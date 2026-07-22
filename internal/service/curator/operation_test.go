package curator

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestCurateAndStoreRejectsUnknownOperation(t *testing.T) {
	service := NewService(nil, &mocks.MockPatternRepository{})

	result, err := service.CurateAndStore(context.Background(), CurateRequest{Operation: Operation("learn_curent")})

	require.ErrorContains(t, err, "unsupported curate operation")
	require.Nil(t, result)
}
