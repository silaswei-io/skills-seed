package profile

import (
	"context"
	"errors"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
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
