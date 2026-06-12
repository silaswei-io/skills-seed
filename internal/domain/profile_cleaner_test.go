package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestCleanProjectProfileCleansValidationCommands(t *testing.T) {
	profile := &ProjectProfile{
		ValidationCommands: []ValidationCommand{
			{Command: " task verify ", When: " after changes ", Source: " Taskfile.yml "},
			{Command: "task verify", When: "after changes", Source: "README.md"},
			{Command: ""},
			{Command: "TODO add command"},
			{Command: "待确认验证命令"},
		},
	}

	cleaned := CleanProjectProfile(profile)

	require.Len(t, cleaned.ValidationCommands, 1)
	assert.Equal(t, "task verify", cleaned.ValidationCommands[0].Command)
	assert.Equal(t, "after changes", cleaned.ValidationCommands[0].When)
	assert.Equal(t, "Taskfile.yml", cleaned.ValidationCommands[0].Source)
}

func TestNewProjectSpecFromProfilePreservesValidationCommands(t *testing.T) {
	spec := NewProjectSpecFromProfile(&ProjectProfile{
		ProjectName: "demo",
		Language:    "unknown",
		ValidationCommands: []ValidationCommand{
			{Command: "task verify", When: "after changes", Source: "Taskfile.yml"},
		},
	}, nil, WorkspaceProjectOverride{})

	require.NotNil(t, spec)
	require.Len(t, spec.ValidationCommands, 1)
	assert.Equal(t, "task verify", spec.ValidationCommands[0].Command)
	assert.Equal(t, "Taskfile.yml", spec.ValidationCommands[0].Source)
}
