package skillgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeneratedSkillName(t *testing.T) {
	tests := []struct {
		name    string
		project string
		want    string
	}{
		{name: "simple name", project: "Demo API", want: "demo-api-dev"},
		{name: "already suffixed", project: "demo-dev", want: "demo-dev"},
		{name: "collapses separators", project: " Demo__API  ", want: "demo-api-dev"},
		{name: "fallback", project: "中文项目", want: "project-dev"},
		{name: "digits", project: "API v2", want: "api-v2-dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GeneratedSkillName(tt.project))
		})
	}
}
