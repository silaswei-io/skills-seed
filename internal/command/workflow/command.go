package workflow

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
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	workflowservice "github.com/silaswei-io/skills-seed/internal/service/workflow"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
	"github.com/silaswei-io/skills-seed/internal/terminal/progress"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

type options struct {
	name      string
	content   string
	overwrite bool
	child     string
}

// Cmd 返回 workflow 命令。
func Cmd(cont *container.Container) *cobra.Command {
	opts := options{}
	cmd := &cobra.Command{
		Use:     "workflow",
		Short:   i18n.Get("WorkflowShort"),
		Long:    i18n.Get("WorkflowLongDesc"),
		Example: i18n.Get("WorkflowExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil || cont.WorkflowSvc == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			targetCont, closeTarget, targetName, err := resolveWorkflowTarget(cmd.Context(), cont, opts.child)
			if err != nil {
				return err
			}
			if closeTarget != nil {
				defer closeTarget()
			}
			tracker := progress.New(1)
			retryProgress := agent.NewRetryProgressBinder(tracker.UpdateStep)
			ctx := retryProgress.WithContext(runtimecontext.WithSeedPath(cmd.Context(), targetCont.SeedPath))
			label := i18n.Get("ProgressOptimizeWorkflowAI")
			var workflow *domain.Workflow
			err = tracker.RunStep(label, func() error {
				retryProgress.StartStep(label)
				var callErr error
				workflow, callErr = targetCont.WorkflowSvc.UpsertWorkflow(ctx, workflowservice.UpsertRequest{
					Name:      opts.name,
					Content:   opts.content,
					Overwrite: opts.overwrite,
				})
				retryProgress.FinishStep(label, callErr == nil)
				return callErr
			})
			if err != nil {
				return err
			}
			logger.Info(i18n.GetWithParams("WorkflowSaved", map[string]interface{}{
				"Name":   workflow.Name,
				"ID":     workflow.ID,
				"Target": workflowTargetLabel(targetName),
			}))
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.name, "name", "", i18n.Get("WorkflowFlagName"))
	cmd.Flags().StringVar(&opts.content, "content", "", i18n.Get("WorkflowFlagContent"))
	cmd.Flags().BoolVar(&opts.overwrite, "overwrite", false, i18n.Get("WorkflowFlagOverwrite"))
	cmd.Flags().StringVar(&opts.child, "child", "", i18n.Get("WorkflowFlagChild"))
	cmd.AddCommand(showCmd(cont))
	return cmd
}

func resolveWorkflowTarget(ctx context.Context, cont *container.Container, child string) (*container.Container, func(), string, error) {
	child = strings.TrimSpace(child)
	if child == "" {
		return cont, nil, workflowTargetName(cont), nil
	}
	if cont.ConfigRepo.GetProjectConfig().Mode != domain.ModeWorkspace {
		return nil, nil, "", fmt.Errorf("%s", i18n.Get("WorkflowChildRequiresWorkspace"))
	}
	project, ok := findWorkflowChild(cont, child)
	if !ok {
		return nil, nil, "", fmt.Errorf("%s", i18n.GetWithParams("WorkflowChildNotFound", map[string]interface{}{"Child": child}))
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
		NotInitialized: "WorkflowChildNotInitialized",
		NotGitRepo:     "WorkflowChildNotGitRepo",
		ModeInvalid:    "WorkflowChildModeInvalid",
	})
	if err != nil {
		return nil, nil, "", err
	}
	return childCont, func() { _ = childCont.Close() }, workflowProjectName(project), nil
}

func findWorkflowChild(cont *container.Container, child string) (config.WorkspaceProjectConfig, bool) {
	for _, project := range cont.ConfigRepo.GetWorkspaceConfig().Projects {
		if project.ID == child || project.Path == child || filepath.Base(filepath.Clean(project.Path)) == child {
			return project, true
		}
	}
	return config.WorkspaceProjectConfig{}, false
}

func workflowProjectName(project config.WorkspaceProjectConfig) string {
	if id := strings.TrimSpace(project.ID); id != "" {
		return id
	}
	return strings.TrimSpace(project.Path)
}

func workflowTargetName(cont *container.Container) string {
	project := cont.ConfigRepo.GetProjectConfig()
	if project.Mode == domain.ModeWorkspace {
		return domain.ModeWorkspace
	}
	if name := strings.TrimSpace(project.Name); name != "" {
		return name
	}
	return domain.ModeProject
}

func workflowTargetLabel(target string) string {
	switch target {
	case domain.ModeWorkspace:
		return i18n.Get("ResourceTargetWorkspace")
	case domain.ModeProject:
		return i18n.Get("ResourceTargetProject")
	default:
		return target
	}
}
