package runtimecontext

import (
	"context"
	"path/filepath"
	"strings"
)

type userContextKey struct{}
type seedPathKey struct{}

// WithUserContext 保存当前 learn/generate 运行的一次性用户上下文。
func WithUserContext(ctx context.Context, text string) context.Context {
	text = strings.TrimSpace(text)
	if text == "" {
		return ctx
	}
	return context.WithValue(ctx, userContextKey{}, text)
}

// WithoutUserContext 在派生操作中屏蔽用户上下文。
func WithoutUserContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, userContextKey{}, "")
}

// UserContext 返回附加到 ctx 的一次性用户上下文。
func UserContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	text, _ := ctx.Value(userContextKey{}).(string)
	return strings.TrimSpace(text)
}

// WithSeedPath 保存当前 .skills-seed 路径，用于一次性运行时输入文件。
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

// SeedPath 返回附加到 ctx 的当前 .skills-seed 路径。
func SeedPath(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	path, _ := ctx.Value(seedPathKey{}).(string)
	return strings.TrimSpace(path)
}

// ProjectRoot 返回当前 .skills-seed 所属的项目根目录。
func ProjectRoot(ctx context.Context) string {
	seedPath := SeedPath(ctx)
	if seedPath == "" {
		return ""
	}
	return filepath.Dir(seedPath)
}
