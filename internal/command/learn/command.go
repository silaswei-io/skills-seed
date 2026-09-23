package learn

import (
	"context"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/changelog"
	"github.com/silaswei-io/skills-seed/internal/service/learncurrent"
	"github.com/spf13/cobra"
)

// CurrentRunOptions 描述外部命令调用 learn current 时允许覆盖的执行选项。
type CurrentRunOptions = learncurrent.CurrentRunOptions

// Cmd 返回 learn 命令。
func Cmd(cont *container.Container) *cobra.Command {
	learnCmd := &cobra.Command{
		Use:     "learn",
		Short:   i18n.Get("LearnShort"),
		Long:    i18n.Get("LearnLongDesc"),
		Example: i18n.Get("LearnExample"),
	}

	var (
		language    string
		focusPaths  []string
		profileMode = "auto"
		contextText string
		contextPath []string
		force       bool
	)
	currentCmd := &cobra.Command{
		Use:     "current",
		Short:   i18n.Get("LearnCurrentShort"),
		Long:    i18n.Get("LearnCurrentLongDesc"),
		Example: i18n.Get("LearnCurrentExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			result, err := learncurrent.RunWithFlags(cmd.Context(), cont, learncurrent.Flags{
				Language:    language,
				FocusPaths:  focusPaths,
				ProfileMode: profileMode,
				ContextText: contextText,
				ContextPath: contextPath,
				Force:       force,
			})
			if err != nil {
				return err
			}
			change := changelog.Start(cont.SeedPath, "learn current")
			learncurrent.RecordSummary(change, result)
			return change.Save(i18n.Get("ChangeLogSummaryLearnCurrent"))
		},
	}
	currentCmd.Flags().StringVarP(&language, "language", "l", "", i18n.Get("LearnFlagLanguage"))
	currentCmd.Flags().StringArrayVarP(&focusPaths, "focus", "f", nil, i18n.Get("LearnFlagFocus"))
	currentCmd.Flags().StringVar(&profileMode, "profile", "auto", i18n.Get("LearnFlagProfile"))
	currentCmd.Flags().StringVar(&contextText, "context", "", i18n.Get("LearnFlagContext"))
	currentCmd.Flags().StringArrayVar(&contextPath, "context-path", nil, i18n.Get("LearnFlagContextPath"))
	currentCmd.Flags().BoolVar(&force, "force", false, i18n.Get("LearnFlagForce"))

	learnCmd.AddCommand(currentCmd)
	return learnCmd
}

// RunLearnCurrent 导出：从当前代码库学习，并返回学习摘要。
func RunLearnCurrent(ctx context.Context, cont *container.Container) (domain.LearnCurrentResult, error) {
	return learncurrent.Run(ctx, cont)
}

// RunLearnCurrentWithContext 导出：从当前代码库学习，附加一次性用户上下文。
func RunLearnCurrentWithContext(ctx context.Context, cont *container.Container, userContext string) (domain.LearnCurrentResult, error) {
	return learncurrent.RunWithContext(ctx, cont, userContext)
}

// RunLearnCurrentWithStateScope 从当前代码库学习，并使用指定恢复状态 scope。
func RunLearnCurrentWithStateScope(ctx context.Context, cont *container.Container, stateScope string, userContext string) (domain.LearnCurrentResult, error) {
	return learncurrent.RunWithStateScope(ctx, cont, stateScope, userContext)
}

// RunLearnCurrentWithStateScopeOptions 从当前代码库学习，并允许调用方指定运行选项。
func RunLearnCurrentWithStateScopeOptions(ctx context.Context, cont *container.Container, stateScope string, userContext string, opts CurrentRunOptions) (domain.LearnCurrentResult, error) {
	return learncurrent.RunWithStateScopeOptions(ctx, cont, stateScope, userContext, opts)
}

// RunWorkspaceRelationships 保存工作区关系产物，供 workspace sync 收尾调用。
func RunWorkspaceRelationships(cont *container.Container, userContext string) (bool, error) {
	return learncurrent.RunWorkspaceRelationships(cont, userContext)
}
