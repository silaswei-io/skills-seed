package tenant

import (
	"context"
	"errors"
	"strings"
)

var ErrMissing = errors.New("tenant context is missing")

type contextKey struct{}

func WithID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, strings.TrimSpace(id))
}

// RequireID returns the validated tenant identity used by repositories and domain capabilities.
func RequireID(ctx context.Context) (string, error) {
	id, _ := ctx.Value(contextKey{}).(string)
	id = strings.TrimSpace(id)
	if id == "" {
		return "", ErrMissing
	}
	return id, nil
}
