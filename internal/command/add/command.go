package add

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	initcmd "github.com/silaswei-io/skills-seed/internal/command/init"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

func Cmd(cont *container.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add .|project-id-or-path...",
		Short:   i18n.Get("AddShort"),
		Long:    i18n.Get("AddLongDesc"),
		Example: i18n.Get("AddExample"),
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			workspaceRoot := cont.ConfigRepo.GetProjectConfig().RootPath
			if workspaceRoot == "" {
				var err error
				workspaceRoot, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("%s: %w", i18n.Get("InitGetCurrentDirFailed"), err)
				}
			}
			return runAddWorkspaceProjects(cmd.Context(), workspaceRoot, cont.ConfigRepo, args)
		},
	}
	return cmd
}

func runAddWorkspaceProjects(ctx context.Context, workspaceRoot string, rootConfigRepo *config.Repository, targets []string) error {
	if rootConfigRepo.GetProjectConfig().Mode != domain.ModeWorkspace {
		return fmt.Errorf("%s", i18n.Get("AddRequireWorkspaceMode"))
	}

	detected := workspacediscovery.DiscoverProjects(workspaceRoot)
	if len(detected) == 0 {
		return fmt.Errorf("%s", i18n.Get("AddProjectsMissing"))
	}

	selected, err := selectWorkspaceProjects(detected, targets)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return fmt.Errorf("%s", i18n.Get("AddProjectsMissing"))
	}

	locale := rootConfigRepo.GetProjectConfig().Locale
	for _, project := range selected {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := initcmd.EnsureWorkspaceChildInitializedAt(workspaceRoot, project, rootConfigRepo, locale); err != nil {
			return err
		}
	}

	workspaceConfig := rootConfigRepo.GetWorkspaceConfig()
	workspaceConfig.Projects = mergeWorkspaceProjects(workspaceConfig.Projects, selected)
	if err := rootConfigRepo.SetWorkspaceConfig(workspaceConfig); err != nil {
		return err
	}

	logger.Info(i18n.Get("AddComplete"))
	return nil
}

func selectWorkspaceProjects(detected []config.WorkspaceProjectConfig, targets []string) ([]config.WorkspaceProjectConfig, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("%s", i18n.Get("AddTargetRequired"))
	}
	if selectsAllDetectedProjects(targets) {
		return detected, nil
	}

	byID := make(map[string]config.WorkspaceProjectConfig, len(detected))
	byPath := make(map[string]config.WorkspaceProjectConfig, len(detected))
	for _, project := range detected {
		byID[project.ID] = project
		byPath[project.Path] = project
	}

	selected := make([]config.WorkspaceProjectConfig, 0, len(targets))
	seen := map[string]bool{}
	for _, target := range targets {
		normalizedTarget := normalizeProjectTarget(target)
		project, ok := byID[normalizedTarget]
		if !ok {
			project, ok = byPath[normalizedTarget]
		}
		if !ok {
			return nil, fmt.Errorf("%s", i18n.GetWithParams("AddProjectNotFound", map[string]interface{}{"ProjectName": target}))
		}
		if seen[project.Path] {
			continue
		}
		seen[project.Path] = true
		selected = append(selected, project)
	}
	return selected, nil
}

func selectsAllDetectedProjects(targets []string) bool {
	return len(targets) == 1 && normalizeProjectTarget(targets[0]) == "."
}

func normalizeProjectTarget(target string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(target, "\\", "/"))
	if normalized == "" {
		return normalized
	}
	normalized = path.Clean(normalized)
	if normalized == "/" {
		return normalized
	}
	return strings.TrimPrefix(normalized, "./")
}

func mergeWorkspaceProjects(existing, additions []config.WorkspaceProjectConfig) []config.WorkspaceProjectConfig {
	merged := make([]config.WorkspaceProjectConfig, 0, len(existing)+len(additions))
	indexByPath := map[string]int{}
	for _, project := range existing {
		indexByPath[project.Path] = len(merged)
		merged = append(merged, project)
	}
	for _, project := range additions {
		if idx, ok := indexByPath[project.Path]; ok {
			merged[idx] = project
			continue
		}
		indexByPath[project.Path] = len(merged)
		merged = append(merged, project)
	}
	return merged
}
