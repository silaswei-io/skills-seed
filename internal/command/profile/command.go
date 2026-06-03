package profile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/spf13/cobra"
)

// Cmd 返回 profile 命令
func Cmd(cont *container.Container) *cobra.Command {
	profileCmd := &cobra.Command{
		Use:     "profile",
		Short:   i18n.Get("ProfileShort"),
		Long:    i18n.Get("ProfileLongDesc"),
		Example: i18n.Get("ProfileExample"),
	}

	profileCmd.AddCommand(showCmd(cont))
	profileCmd.AddCommand(refreshCmd(cont))

	return profileCmd
}

func showCmd(cont *container.Container) *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Short:   i18n.Get("ProfileShowShort"),
		Long:    i18n.Get("ProfileShowLongDesc"),
		Example: i18n.Get("ProfileShowExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			return showProfile(cont)
		},
	}
}

func refreshCmd(cont *container.Container) *cobra.Command {
	var language string

	cmd := &cobra.Command{
		Use:     "refresh",
		Short:   i18n.Get("ProfileRefreshShort"),
		Long:    i18n.Get("ProfileRefreshLongDesc"),
		Example: i18n.Get("ProfileRefreshExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			return refreshProfile(cont, language)
		},
	}
	cmd.Flags().StringVarP(&language, "language", "l", "", i18n.Get("LearnFlagLanguage"))
	return cmd
}

func showProfile(cont *container.Container) error {
	ctx := context.Background()
	projectProfile, err := cont.ProfileRepo.Get(ctx)
	if err != nil {
		if errors.Is(err, profilestore.ErrProfileNotFound) {
			return fmt.Errorf("%s", i18n.Get("ProfileMissing"))
		}
		return err
	}

	logger.Info(i18n.GetWithParams("ProfileShowHeader", map[string]interface{}{
		"ProjectName": projectProfile.ProjectName,
		"Language":    projectProfile.Language,
		"GeneratedAt": projectProfile.GeneratedAt,
	}))
	if projectProfile.Summary != "" {
		logger.Info(projectProfile.Summary)
	}
	logger.Info(i18n.GetWithParams("ProfileShowStats", map[string]interface{}{
		"Frameworks":      len(projectProfile.Frameworks),
		"Dependencies":    len(projectProfile.Dependencies),
		"Modules":         len(projectProfile.KeyModules),
		"BusinessMethods": len(projectProfile.BusinessMethods),
	}))
	return nil
}

func refreshProfile(cont *container.Container, language string) error {
	if err := commandutil.RequireAgentAvailable(cont); err != nil {
		return err
	}

	ctx := context.Background()
	if err := commandutil.LockConfiguredMode(ctx, cont); err != nil {
		return err
	}
	projectRoot, projectName, currentLanguage, err := resolveProjectContext(cont, language)
	if err != nil {
		return err
	}

	logger.Info(i18n.GetWithParams("ProfileRefreshStarting", map[string]interface{}{
		"ProjectRoot": projectRoot,
		"ProjectName": projectName,
		"Language":    currentLanguage,
	}))

	tracker := progress.New(1)
	retryProgress := agent.NewRetryProgressBinder(tracker.UpdateStep)
	ctx = retryProgress.WithContext(ctx)
	label := i18n.Get("ProgressProfileRefreshAI")
	var result *analyzer.AnalyzeProjectResult
	err = tracker.RunStep(label, func() error {
		retryProgress.StartStep(label)
		var callErr error
		result, callErr = cont.AnalyzerSvc.AnalyzeProjectFullWithLanguage(ctx, projectRoot, projectName, currentLanguage)
		retryProgress.FinishStep(label, callErr == nil)
		return callErr
	})
	if err != nil {
		return err
	}
	if err := cont.ProfileRepo.Save(ctx, analyzer.NewProjectProfile(result, projectName, currentLanguage)); err != nil {
		return err
	}

	logger.Info(i18n.Get("ProfileRefreshComplete"))
	if err := commandutil.MarkLearned(ctx, cont); err != nil {
		return err
	}
	return nil
}

func resolveProjectContext(cont *container.Container, requestedLanguage string) (string, string, string, error) {
	ctx := context.Background()
	projectRoot, err := cont.GitRepo.GetProjectRoot(ctx)
	if err != nil {
		projectRoot = cont.ConfigRepo.GetProjectConfig().RootPath
	}
	if projectRoot == "" {
		projectRoot, err = os.Getwd()
		if err != nil {
			return "", "", "", err
		}
	}

	projectName := filepath.Base(projectRoot)
	if configuredName := cont.ConfigRepo.GetProjectConfig().Name; configuredName != "" {
		projectName = configuredName
	}

	currentLanguage := requestedLanguage
	if currentLanguage == "" {
		currentLanguage = cont.ConfigRepo.GetProjectConfig().Language
	}
	if currentLanguage == "" {
		currentLanguage = "go"
	}

	return projectRoot, projectName, currentLanguage, nil
}
