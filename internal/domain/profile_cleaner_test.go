package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanProjectProfileNormalizesBusinessMethodLocation(t *testing.T) {
	profile := &ProjectProfile{
		BusinessMethods: []BusinessMethod{
			{
				Name:         "CreateOrder",
				CodeLocation: CodeLocation{CurrentLocation: "internal/service/order.go:42"},
			},
		},
	}

	cleaned := CleanProjectProfile(profile)

	assert.Len(t, cleaned.BusinessMethods, 1)
	method := cleaned.BusinessMethods[0]
	assert.Equal(t, "internal/service/order.go:42", method.CodeLocation.HistoricalLocation)
	assert.Equal(t, "internal/service/order.go:42", method.CodeLocation.CurrentLocation)
	assert.Equal(t, CodeLocationStatusValid, method.CodeLocation.Status)
	assert.False(t, method.CodeLocation.CreatedAt.IsZero())
	assert.False(t, method.CodeLocation.UpdatedAt.IsZero())
}
