// Package navigation 将本地结构事实保存为导航快照，不写入语义学习指纹或模式库。
package navigation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/fileio"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
)

// snapshot 同时绑定权威输入版本，避免仅因源码未变而继续投影旧约束。
type snapshot struct {
	codegraph.Index
	AuthorityRevision string `json:"authority_revision"`
}

// SnapshotPath 位于持久知识目录之外，也不受运行日志清理影响。
func SnapshotPath(root string) string {
	return filepath.Join(root, codegraph.IndexDir, "navigation.json")
}

// Refresh 复用统一文件选择策略，索引只接收允许参与源码学习的精确路径。
func Refresh(ctx context.Context, cfg config.Reader, root string) (*codegraph.Index, bool, error) {
	selection, err := fileanalysis.SelectFiles(fileanalysis.SelectOptions{Root: root, Policy: fileanalysis.NewConfiguredSelectionPolicy(cfg, root)})
	if err != nil {
		return nil, false, err
	}
	index, err := codegraph.Scan(ctx, root, selection.Paths(), cfg.GetCodeGraphConfig().EffectiveRuntime())
	if err != nil {
		return nil, false, err
	}
	revision, err := analyzer.EngineeringKnowledgeRevision(root)
	if err != nil {
		return nil, false, err
	}
	index.AuthorityPaths, err = analyzer.EngineeringKnowledgePaths(root)
	if err != nil {
		return nil, false, err
	}
	data, err := json.Marshal(snapshot{Index: *index, AuthorityRevision: revision})
	if err != nil {
		return nil, false, err
	}
	path := SnapshotPath(root)
	previous, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, false, err
	}
	changed := !bytes.Equal(previous, data)
	if changed {
		if err := fileio.WriteFileAtomic(path, data, 0600); err != nil {
			return nil, false, err
		}
	}
	return index, changed, nil
}

// Hash 将生成产物绑定到已保存的本地扫描版本，查询索引不能替代生成提交。
func Hash(root string) (string, error) {
	data, err := os.ReadFile(SnapshotPath(root))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}
