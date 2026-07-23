package sync

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/interactive"
	"github.com/spf13/cobra"
)

func shouldRunInteractiveSync(cmd *cobra.Command, userContext string, noInteractive bool) bool {
	if noInteractive || !interactive.IsTerminal() {
		return false
	}
	if userContext != "" {
		return false
	}
	for _, name := range []string{"resume", "restart"} {
		if flag := cmd.Flags().Lookup(name); flag != nil && flag.Changed {
			return false
		}
	}
	return true
}

func resolveInteractiveSync(ctx context.Context, _ *cobra.Command, cont *container.Container, stateScope string) (syncRunMode, error) {
	hasState, err := hasSyncCommandState(ctx, cont.SeedPath, stateScope)
	if err != nil {
		return syncRunAuto, err
	}
	if hasState {
		mode, err := interactive.Select(i18n.Get("InteractiveSyncInterruptedTitle"), []interactive.Option[syncRunMode]{
			{Value: syncRunResume, Title: i18n.Get("InteractiveSyncResume"), Description: i18n.Get("InteractiveSyncResumeDesc")},
			{Value: syncRunRestart, Title: i18n.Get("InteractiveSyncRestart"), Description: i18n.Get("InteractiveSyncRestartDesc")},
			{Value: "", Title: i18n.Get("InteractiveCancel")},
		}, syncRunResume)
		if err != nil {
			return syncRunAuto, err
		}
		if mode == "" {
			return syncRunAuto, interactive.ErrCanceled
		}
		return mode, nil
	}

	title, options := interactiveSyncRunModeOptions(syncGeneratedSkillMissing(cont))
	mode, err := interactive.Select(title, options, syncRunAuto)
	if err != nil {
		return syncRunAuto, err
	}
	if mode == "" {
		return syncRunAuto, interactive.ErrCanceled
	}
	return mode, nil
}

func interactiveSyncRunModeOptions(firstRun bool) (string, []interactive.Option[syncRunMode]) {
	if firstRun {
		return i18n.Get("InteractiveSyncFirstRunTitle"), []interactive.Option[syncRunMode]{
			{Value: syncRunAuto, Title: i18n.Get("InteractiveSyncFirstRun"), Description: i18n.Get("InteractiveSyncFirstRunDesc")},
			{Value: "", Title: i18n.Get("InteractiveCancel")},
		}
	}
	return i18n.Get("InteractiveSyncRunMode"), []interactive.Option[syncRunMode]{
		{Value: syncRunAuto, Title: i18n.Get("InteractiveSyncAuto"), Description: i18n.Get("InteractiveSyncAutoDesc")},
		{Value: syncRunRestart, Title: i18n.Get("InteractiveSyncRestart"), Description: i18n.Get("InteractiveSyncRestartDesc")},
		{Value: "", Title: i18n.Get("InteractiveCancel")},
	}
}

func hasSyncCommandState(ctx context.Context, seedPath, stateScope string) (bool, error) {
	_, err := commandstate.NewRepository(seedPath, stateScope).Load(ctx)
	if err == nil {
		return true, nil
	}
	if err == commandstate.ErrStateNotFound {
		return false, nil
	}
	return false, err
}

func hasResumableSyncCommandState(ctx context.Context, seedPath, stateScope string) (bool, error) {
	state, err := commandstate.NewRepository(seedPath, stateScope).Load(ctx)
	if err != nil {
		if err == commandstate.ErrStateNotFound {
			return false, nil
		}
		return false, err
	}
	return state != nil && len(state.Files)+len(state.Deleted) > 0 && len(state.Units) > 0, nil
}
