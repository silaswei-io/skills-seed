// Package codegraph 提供已有能力检索入口，使用与本地扫描一致的索引和范围。
package codegraph

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	graph "github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/service/navigation"
	"github.com/spf13/cobra"
)

// Cmd 只暴露业务开发需要的只读结构查询，不透传索引修改或任意脚本。
func Cmd(cont *container.Container) *cobra.Command {
	cmd := &cobra.Command{Use: "codegraph", Short: i18n.Get("CodeGraphShort"), Long: i18n.Get("CodeGraphShort"), Example: `skills-seed codegraph query "HTTP"`}
	for _, action := range []string{"query", "node", "callers", "callees", "impact", "trace"} {
		count := 1
		usage := action + " <symbol-id>"
		if action == "query" {
			usage = action + " <search-text>"
		}
		if action == "trace" {
			count = 2
			usage = action + " <from-id> <to-id>"
		}
		cmd.AddCommand(&cobra.Command{Use: usage, Short: i18n.Get("CodeGraphShort"), Long: i18n.Get("CodeGraphShort"), Example: "skills-seed codegraph " + usage, Args: cobra.ExactArgs(count), RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			root := cont.ConfigRepo.GetProjectConfig().RootPath
			if root == "" {
				root = filepath.Dir(cont.SeedPath)
			}
			if _, _, err := navigation.Refresh(cmd.Context(), cont.ConfigRepo, root); err != nil {
				return err
			}
			result, err := graph.Query(cmd.Context(), root, action, args, cont.ConfigRepo.GetCodeGraphConfig().EffectiveRuntime())
			if err != nil {
				return err
			}
			var output any
			if err := json.Unmarshal(result, &output); err != nil {
				return err
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(output)
		}})
	}
	return cmd
}
