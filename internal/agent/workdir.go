package agent

import (
	"context"
	"os"

	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
)

// WorkDirForContext returns the project root bound to ctx, or the current directory.
func WorkDirForContext(ctx context.Context) (string, error) {
	if projectRoot := runtimecontext.ProjectRoot(ctx); projectRoot != "" {
		return projectRoot, nil
	}
	return os.Getwd()
}
