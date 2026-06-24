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
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	workflowservice "github.com/silaswei-io/skills-seed/internal/service/workflow"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

type options struct {
	name      string
	context   string
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
			ctx := retryProgress.WithContext(cmd.Context())
			label := i18n.Get("ProgressOptimizeWorkflowAI")
			var workflow *domain.Workflow
			err = tracker.RunStep(label, func() error {
				retryProgress.StartStep(label)
				var callErr error
				workflow, callErr = targetCont.WorkflowSvc.UpsertWorkflow(ctx, workflowservice.UpsertRequest{
					Name:      opts.name,
					Context:   opts.context,
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
				"Target": targetName,
			}))
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.name, "name", "", i18n.Get("WorkflowFlagName"))
	cmd.Flags().StringVar(&opts.context, "context", "", i18n.Get("WorkflowFlagContext"))
	cmd.Flags().BoolVar(&opts.overwrite, "overwrite", false, i18n.Get("WorkflowFlagOverwrite"))
	cmd.Flags().StringVar(&opts.child, "child", "", i18n.Get("WorkflowFlagChild"))
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
	project, ok := findWorkflowChildProject(cont, child)
	if !ok {
		return nil, nil, "", fmt.Errorf("%s", i18n.GetWithParams("WorkflowChildNotFound", map[string]interface{}{"Child": child}))
	}
	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	if projectRoot == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return nil, nil, "", err
		}
	}
	projectRootPath, err := workspacediscovery.ResolveProjectRoot(projectRoot, project)
	if err != nil {
		return nil, nil, "", err
	}
	childCont, err := commandutil.OpenWorkspaceChildContainer(ctx, projectRootPath, project, commandutil.WorkspaceChildErrorKeys{
		NotInitialized: "WorkflowChildNotInitialized",
		NotGitRepo:     "WorkflowChildNotGitRepo",
		ModeInvalid:    "WorkflowChildModeInvalid",
	})
	if err != nil {
		return nil, nil, "", err
	}
	return childCont, func() { _ = childCont.Close() }, workflowProjectID(project), nil
}

func findWorkflowChildProject(cont *container.Container, child string) (config.WorkspaceProjectConfig, bool) {
	for _, project := range cont.ConfigRepo.GetWorkspaceConfig().Projects {
		if project.ID == child || project.Path == child || filepath.Base(filepath.Clean(project.Path)) == child {
			return project, true
		}
	}
	return config.WorkspaceProjectConfig{}, false
}

func workflowProjectID(project config.WorkspaceProjectConfig) string {
	if strings.TrimSpace(project.ID) != "" {
		return strings.TrimSpace(project.ID)
	}
	return strings.TrimSpace(project.Path)
}

func workflowTargetName(cont *container.Container) string {
	if cont == nil || cont.ConfigRepo == nil {
		return ""
	}
	project := cont.ConfigRepo.GetProjectConfig()
	if project.Mode == domain.ModeWorkspace {
		return "workspace"
	}
	if strings.TrimSpace(project.Name) != "" {
		return strings.TrimSpace(project.Name)
	}
	return "project"
}
