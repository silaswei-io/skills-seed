package codegraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

// File 是索引中的结构事实，不代表业务语义已被学习。
type File struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Hash     string `json:"contentHash"`
}

// Node 保留能力检索和源码定位需要的公开字段。
type Node struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	Path            string `json:"filePath"`
	Line            int    `json:"startLine"`
	EndLine         int    `json:"endLine"`
	Signature       string `json:"signature"`
	Exported        bool   `json:"isExported,omitempty"`
	DeclarationOnly bool   `json:"declarationOnly,omitempty"`
}

// Call 是 CodeGraph 已解析的符号调用，不将普通文件引用误认为行为调用。
type Call struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Dependency 是静态解析出的文件依赖，不保证覆盖动态业务调用。
type Dependency struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Index 是用于生成导航的快照，未索引文件必须显式展示。
type Index struct {
	AuthorityPaths []string     `json:"authority_paths,omitempty"` // 本地发现的权威来源，保持原始规则边界。
	Files          []File       `json:"files"`
	Nodes          []Node       `json:"nodes"`
	Calls          []Call       `json:"calls,omitempty"`
	Dependencies   []Dependency `json:"dependencies"`
	Unindexed      []string     `json:"unindexed"`
}

// Scan 在独立索引中同步精确路径集合，整个请求只打开一次 SDK。
func Scan(ctx context.Context, root string, paths []string, runtime string) (*Index, error) {
	data, err := invoke(ctx, root, map[string]any{"action": "scan", "paths": paths}, runtime)
	if err != nil {
		return nil, err
	}
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("decode CodeGraph index: %w", err)
	}
	normalizeGoExports(&index)
	return &index, nil
}

// normalizeGoExports 补齐 SDK 未标注的 Go 导出方法，保持能力目录与语言可见性一致。
func normalizeGoExports(index *Index) {
	for i := range index.Nodes {
		node := &index.Nodes[i]
		if node.Kind == "method" && strings.HasSuffix(node.Path, ".go") {
			node.Exported = token.IsExported(node.Name)
		}
	}
}

// Query 只读查询已扫描结构；调用方负责在查询前更新本地索引。
func Query(ctx context.Context, root, action string, args []string, runtime string) (json.RawMessage, error) {
	return invoke(ctx, root, map[string]any{"action": action, "args": args}, runtime)
}

func invoke(ctx context.Context, root string, request any, selection string) ([]byte, error) {
	home, err := runtimeHome(ctx, selection)
	if err != nil {
		return nil, err
	}
	node := "node"
	if runtime.GOOS == "windows" {
		node += ".exe"
	}
	bridge, err := runtimeFiles.ReadFile("bridge.cjs")
	if err != nil {
		return nil, err
	}
	input, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, IndexDir), 0700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(root, IndexDir, "skills-seed.lock"))
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, ctx.Err()
	}
	defer lock.Unlock()
	// -e 避免落盘桥接脚本；参数位置与脚本文件执行方式保持一致。
	cmd := exec.CommandContext(ctx, filepath.Join(home, node), "--liftoff-only", "--disable-warning=ExperimentalWarning", "-e", string(bridge), "skills-seed", home)
	cmd.Dir = root
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "CODEGRAPH_DIR=") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "CODEGRAPH_DIR="+IndexDir)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("CodeGraph: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// Load 不把索引缺失伪装为空项目，也不触发 AI 或隐式全量扫描。
func Load(root string) (*Index, error) {
	data, err := os.ReadFile(filepath.Join(root, IndexDir, "navigation.json"))
	if err != nil {
		return nil, fmt.Errorf("load local navigation (run skills-seed learn current first): %w", err)
	}
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}
	normalizeGoExports(&index)
	return &index, nil
}
