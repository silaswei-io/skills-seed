package rule

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	ruleservice "github.com/silaswei-io/skills-seed/internal/service/rule"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
	"github.com/silaswei-io/skills-seed/internal/terminal/progress"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

type options struct {
	name      string
	content   string
	overwrite bool
	child     string
	projects  []string
	paths     []string
}

type scope struct {
	projects []string
	paths    []string
}

// Cmd 返回 rule 命令。
func Cmd(cont *container.Container) *cobra.Command {
	opts := options{}
	cmd := &cobra.Command{
		Use:     "rule",
		Short:   i18n.Get("RuleShort"),
		Long:    i18n.Get("RuleLongDesc"),
		Example: i18n.Get("RuleExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cont == nil || cont.RuleSvc == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			target, closeTarget, targetName, err := resolveTarget(cmd.Context(), cont, opts.child)
			if err != nil {
				return err
			}
			if closeTarget != nil {
				defer closeTarget()
			}
			ruleScope, err := resolveScope(cont, target, opts)
			if err != nil {
				return err
			}
			tracker := progress.New(1)
			retryProgress := agent.NewRetryProgressBinder(tracker.UpdateStep)
			ctx := retryProgress.WithContext(runtimecontext.WithSeedPath(cmd.Context(), target.SeedPath))
			label := i18n.Get("ProgressOptimizeRuleAI")
			var saved *domain.Rule
			if err := tracker.RunStep(label, func() error {
				retryProgress.StartStep(label)
				var callErr error
				saved, callErr = target.RuleSvc.UpsertRule(ctx, ruleservice.UpsertRequest{
					Name:             opts.name,
					Content:          opts.content,
					AffectedProjects: ruleScope.projects,
					Paths:            ruleScope.paths,
					Overwrite:        opts.overwrite,
				})
				retryProgress.FinishStep(label, callErr == nil)
				return callErr
			}); err != nil {
				return err
			}
			logger.Info(i18n.GetWithParams("RuleSaved", map[string]interface{}{"Name": saved.Name, "ID": saved.ID, "Target": targetLabel(targetName)}))
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.name, "name", "", i18n.Get("RuleFlagName"))
	cmd.Flags().StringVar(&opts.content, "content", "", i18n.Get("RuleFlagContent"))
	cmd.Flags().BoolVar(&opts.overwrite, "overwrite", false, i18n.Get("RuleFlagOverwrite"))
	cmd.Flags().StringVar(&opts.child, "child", "", i18n.Get("RuleFlagChild"))
	cmd.Flags().StringSliceVar(&opts.projects, "project", nil, i18n.Get("RuleFlagProject"))
	cmd.Flags().StringSliceVar(&opts.paths, "path", nil, i18n.Get("RuleFlagPath"))
	cmd.AddCommand(showCmd(cont))
	return cmd
}

func resolveScope(cont, target *container.Container, opts options) (scope, error) {
	ruleScope := scope{projects: stringx.UniqueNonBlank(opts.projects)}
	root := target.ConfigRepo.GetProjectConfig().RootPath
	paths, err := cleanPaths(root, opts.paths)
	if err != nil {
		return scope{}, err
	}
	ruleScope.paths = paths
	if err := validateScope(cont, opts.child, ruleScope); err != nil {
		return scope{}, err
	}
	return ruleScope, nil
}

func validateScope(cont *container.Container, child string, ruleScope scope) error {
	mode := cont.ConfigRepo.GetProjectConfig().Mode
	if strings.TrimSpace(child) != "" {
		if len(ruleScope.projects) > 0 {
			return fmt.Errorf("%s", i18n.Get("RuleChildProjectConflict"))
		}
		return nil
	}
	if mode != domain.ModeWorkspace {
		if len(ruleScope.projects) > 0 {
			return fmt.Errorf("%s", i18n.Get("RuleProjectRequiresWorkspace"))
		}
		return nil
	}
	known := make(map[string]config.WorkspaceProjectConfig)
	for _, project := range cont.ConfigRepo.GetWorkspaceConfig().Projects {
		known[project.ID] = project
	}
	for _, id := range ruleScope.projects {
		if _, ok := known[id]; !ok {
			return fmt.Errorf("%s", i18n.GetWithParams("RuleProjectNotFound", map[string]interface{}{"Project": id}))
		}
	}
	return nil
}

func cleanPaths(root string, paths []string) ([]string, error) {
	normalized := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range stringx.UniqueNonBlank(paths) {
		path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
		if path == "." || path == ".." || strings.HasPrefix(path, "../") || filepath.IsAbs(filepath.FromSlash(path)) {
			return nil, fmt.Errorf("%s", i18n.GetWithParams("RulePathInvalid", map[string]interface{}{"Path": path}))
		}
		if strings.TrimSpace(root) != "" {
			if _, err := projectpath.CanonicalWithinRoot(root, filepath.Join(root, filepath.FromSlash(path))); err != nil {
				return nil, fmt.Errorf("%s", i18n.GetWithParams("RulePathInvalid", map[string]interface{}{"Path": path}))
			}
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		normalized = append(normalized, path)
	}
	return normalized, nil
}

func resolveTarget(ctx context.Context, cont *container.Container, child string) (*container.Container, func(), string, error) {
	child = strings.TrimSpace(child)
	if child == "" {
		return cont, nil, ruleTargetName(cont), nil
	}
	if cont.ConfigRepo.GetProjectConfig().Mode != domain.ModeWorkspace {
		return nil, nil, "", fmt.Errorf("%s", i18n.Get("RuleChildRequiresWorkspace"))
	}
	project, ok := findChild(cont, child)
	if !ok {
		return nil, nil, "", fmt.Errorf("%s", i18n.GetWithParams("RuleChildNotFound", map[string]interface{}{"Child": child}))
	}
	root := strings.TrimSpace(cont.ConfigRepo.GetProjectConfig().RootPath)
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, nil, "", err
		}
	}
	projectRoot, err := workspacediscovery.ResolveProjectRoot(root, project)
	if err != nil {
		return nil, nil, "", err
	}
	childCont, err := commandutil.OpenWorkspaceChildContainer(ctx, projectRoot, project, commandutil.WorkspaceChildErrorKeys{
		NotInitialized: "RuleChildNotInitialized",
		NotGitRepo:     "RuleChildNotGitRepo",
		ModeInvalid:    "RuleChildModeInvalid",
	})
	if err != nil {
		return nil, nil, "", err
	}
	return childCont, func() { _ = childCont.Close() }, ruleProjectName(project), nil
}

func findChild(cont *container.Container, child string) (config.WorkspaceProjectConfig, bool) {
	for _, project := range cont.ConfigRepo.GetWorkspaceConfig().Projects {
		if project.ID == child || project.Path == child || filepath.Base(filepath.Clean(project.Path)) == child {
			return project, true
		}
	}
	return config.WorkspaceProjectConfig{}, false
}

func ruleProjectName(project config.WorkspaceProjectConfig) string {
	if id := strings.TrimSpace(project.ID); id != "" {
		return id
	}
	return strings.TrimSpace(project.Path)
}

func ruleTargetName(cont *container.Container) string {
	project := cont.ConfigRepo.GetProjectConfig()
	if project.Mode == domain.ModeWorkspace {
		return domain.ModeWorkspace
	}
	if name := strings.TrimSpace(project.Name); name != "" {
		return name
	}
	return domain.ModeProject
}

func targetLabel(target string) string {
	switch target {
	case domain.ModeWorkspace:
		return i18n.Get("ResourceTargetWorkspace")
	case domain.ModeProject:
		return i18n.Get("ResourceTargetProject")
	default:
		return target
	}
}
