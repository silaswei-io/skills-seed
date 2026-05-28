package runtimecontext

import (
	"context"
	"strings"
)

type userContextKey struct{}
type seedPathKey struct{}

// WithUserContext stores one-shot user context for the current learn/generate run.
func WithUserContext(ctx context.Context, text string) context.Context {
	text = strings.TrimSpace(text)
	if text == "" {
		return ctx
	}
	return context.WithValue(ctx, userContextKey{}, text)
}

// WithoutUserContext masks user context on derived operations.
func WithoutUserContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, userContextKey{}, "")
}

// UserContext returns the one-shot user context attached to ctx.
func UserContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	text, _ := ctx.Value(userContextKey{}).(string)
	return strings.TrimSpace(text)
}

// WithSeedPath stores the current .skills-seed path for one-shot runtime inputs.
func WithSeedPath(ctx context.Context, seedPath string) context.Context {
	seedPath = strings.TrimSpace(seedPath)
	if seedPath == "" {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, seedPathKey{}, seedPath)
}

// SeedPath returns the current .skills-seed path attached to ctx.
func SeedPath(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	path, _ := ctx.Value(seedPathKey{}).(string)
	return strings.TrimSpace(path)
}
