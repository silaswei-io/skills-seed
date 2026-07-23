package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/interactive"
	"github.com/silaswei-io/skills-seed/internal/pkg/changelog"
	"github.com/silaswei-io/skills-seed/internal/service/syncflow"
	"github.com/spf13/cobra"
)

type syncRunMode string

const (
	syncRunAuto    syncRunMode = "auto"
	syncRunResume  syncRunMode = "resume"
	syncRunRestart syncRunMode = "restart"
)

// Dependencies 描述 sync 命令需要调用的应用用例。
type Dependencies struct {
	LearnCurrent func(cont *container.Container, req syncflow.LearnRequest) (domain.LearnCurrentResult, error)
	Generate     func(cont *container.Container) error
}

// Cmd 返回 sync 命令
func Cmd(cont *container.Container, deps ...Dependencies) *cobra.Command {
	dependencies := Dependencies{}
	if len(deps) > 0 {
		dependencies = deps[0]
	}
	userContext := ""
	contextPath := []string{}
	resume := false
	restart := false
	noInteractive := false
	curationOutput := ""

	cmd := &cobra.Command{
		Use:     "sync",
		Short:   i18n.Get("SyncShort"),
		Long:    i18n.Get("SyncLongDesc"),
		Example: i18n.Get("SyncExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			ctx := cmd.Context()
			stateScope := commandutil.CommandStateScopeForCobra(cmd)
			resolvedContext, err := commandutil.ResolveRuntimeContext(userContext, contextPath...)
			if err != nil {
				return err
			}
			inputs, err := normalizeSyncInputs(syncInputs{
				UserContext: resolvedContext,
			})
			if err != nil {
				return err
			}
			resolvedMode, err := syncModeFromFlags(resume, restart)
			if err != nil {
				return err
			}
			if strings.TrimSpace(curationOutput) != "" && resolvedMode != syncRunResume {
				return fmt.Errorf("%s", i18n.Get("SyncCurationOutputRequiresResume"))
			}
			if shouldRunInteractiveSync(cmd, inputs.UserContext, noInteractive) {
				mode, err := resolveInteractiveSync(ctx, cmd, cont, stateScope)
				if err != nil {
					if errors.Is(err, interactive.ErrCanceled) {
						return nil
					}
					return err
				}
				resolvedMode = mode
			}
			if resolvedMode == syncRunRestart {
				if err := commandstate.NewRepository(cont.SeedPath, stateScope).Clear(); err != nil {
					return err
				}
			}
			if resolvedMode == syncRunResume {
				resumable, err := hasResumableSyncCommandState(ctx, cont.SeedPath, stateScope)
				if err != nil {
					return err
				}
				if !resumable {
					return fmt.Errorf("%s", i18n.Get("SyncResumeStateMissing"))
				}
			}
			change := changelog.Start(cont.SeedPath, "sync")
			if err := syncLearn(ctx, cont, stateScope, inputs.UserContext, curationOutput, resolvedMode, change, dependencies); err != nil {
				return err
			}
			return change.Save(i18n.Get("ChangeLogSummarySync"))
		},
	}

	cmd.Flags().StringVar(&userContext, "context", "", i18n.Get("SyncFlagContext"))
	cmd.Flags().StringArrayVar(&contextPath, "context-path", nil, i18n.Get("SyncFlagContextPath"))
	cmd.Flags().BoolVar(&resume, "resume", false, i18n.Get("SyncFlagResume"))
	cmd.Flags().BoolVar(&restart, "restart", false, i18n.Get("SyncFlagRestart"))
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false, i18n.Get("InteractiveFlagNoInteractive"))
	cmd.Flags().StringVar(&curationOutput, "curation-output", "", i18n.Get("SyncFlagCurationOutput"))

	return cmd
}

func syncModeFromFlags(resume, restart bool) (syncRunMode, error) {
	if resume && restart {
		return syncRunAuto, fmt.Errorf("%s", i18n.Get("SyncRunModeConflict"))
	}
	if resume {
		return syncRunResume, nil
	}
	if restart {
		return syncRunRestart, nil
	}
	return syncRunAuto, nil
}

type syncInputs struct {
	UserContext string
}

func normalizeSyncInputs(inputs syncInputs) (syncInputs, error) {
	inputs.UserContext = strings.TrimSpace(inputs.UserContext)
	return inputs, nil
}

// syncLearn 路径 A：学习当前代码 → 生成 Skills。
func syncLearn(ctx context.Context, cont *container.Container, stateScope string, userContext string, curationOutput string, mode syncRunMode, change *changelog.Builder, deps ...Dependencies) error {
	dependencies := Dependencies{}
	if len(deps) > 0 {
		dependencies = deps[0]
	}
	var learnCurrent syncflow.LearnCurrentFunc
	if dependencies.LearnCurrent != nil {
		learnCurrent = func(ctx context.Context, req syncflow.LearnRequest) (domain.LearnCurrentResult, error) {
			return dependencies.LearnCurrent(cont, req)
		}
	}
	var generate syncflow.GenerateFunc
	if dependencies.Generate != nil {
		generate = func(ctx context.Context) error {
			return dependencies.Generate(cont)
		}
	}
	service := syncflow.Service{
		LearnCurrent: learnCurrent,
		Generate:     generate,
		OutputMissing: func() bool {
			return syncGeneratedSkillMissing(cont)
		},
	}
	return service.Run(ctx, syncflow.Request{
		Learn: syncflow.LearnRequest{
			StateScope:     stateScope,
			UserContext:    userContext,
			Force:          mode == syncRunRestart,
			CurationOutput: strings.TrimSpace(curationOutput),
		},
		Change: change,
	})
}

func syncGeneratedSkillMissing(cont *container.Container) bool {
	if cont == nil || cont.ConfigRepo == nil {
		return false
	}
	outputPath := strings.TrimSpace(cont.ConfigRepo.GetEffectiveSkillsPath())
	if outputPath == "" {
		return false
	}
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(cont.Config.Project.RootPath, filepath.FromSlash(outputPath))
	}
	_, err := os.Stat(filepath.Join(outputPath, "SKILL.md"))
	return errors.Is(err, os.ErrNotExist)
}
