package initcmd

import (
	"fmt"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/interactive"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/spf13/cobra"
)

const (
	defaultInitAgent  = "claude"
	defaultInitSkills = "claude"
)

func shouldRunInteractiveInit(cmd *cobra.Command, opts commandOptions) bool {
	return interactive.IsTerminal() && !opts.noInteractive && !hasAnyChangedInitFlag(cmd)
}

func hasAnyChangedInitFlag(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	for _, name := range []string{"locale", "skills-locale", "mode", "agent", "skills", "workspace"} {
		if flag := cmd.Flags().Lookup(name); flag != nil && flag.Changed {
			return true
		}
	}
	return false
}

func resolveInteractiveInit(cmd *cobra.Command, opts commandOptions) (commandOptions, error) {
	resolved := withInitDefaults(opts)
	interactive.PrintBanner(cmd.OutOrStdout(), "skills-seed", "Project skills for AI agents", bannerTags())

	locale, err := interactive.Select(i18n.GetForLocale(config.DefaultToolLocale, "InteractiveInitToolLocale"), []interactive.Option[string]{
		{Value: i18n.LocaleChinese, Title: "zh-CN"},
		{Value: i18n.LocaleEnglish, Title: "en-US"},
	}, resolved.locale)
	if err != nil {
		return resolved, err
	}
	resolved.locale = locale
	if err := i18n.Init(locale); err != nil {
		return resolved, fmt.Errorf("%s: %w", i18n.Get("InitI18nInitFailed"), err)
	}

	mode, err := interactive.Select(i18n.Get("InteractiveInitMode"), []interactive.Option[string]{
		{Value: domain.ModeProject, Title: i18n.Get("InteractiveInitModeProject"), Description: i18n.Get("InteractiveInitModeProjectDesc")},
		{Value: domain.ModeWorkspace, Title: i18n.Get("InteractiveInitModeWorkspace"), Description: i18n.Get("InteractiveInitModeWorkspaceDesc")},
	}, resolved.mode)
	if err != nil {
		return resolved, err
	}
	resolved.mode = mode
	resolved.workspace = mode == domain.ModeWorkspace

	skillsLocale, err := interactive.Select(i18n.Get("InteractiveInitSkillsLocale"), []interactive.Option[string]{
		{Value: i18n.LocaleEnglish, Title: "en-US"},
		{Value: i18n.LocaleChinese, Title: "zh-CN"},
	}, resolved.skillsLocale)
	if err != nil {
		return resolved, err
	}
	resolved.skillsLocale = skillsLocale

	agent, err := interactive.Select(i18n.Get("InteractiveInitAgent"), agentOptions(), resolved.agent)
	if err != nil {
		return resolved, err
	}
	resolved.agent = agent

	skills, err := interactive.Select(i18n.Get("InteractiveInitSkills"), agentOptions(), resolved.skills)
	if err != nil {
		return resolved, err
	}
	resolved.skills = skills

	interactive.PrintSummary(cmd.OutOrStdout(), i18n.Get("InteractiveInitSummaryTitle"), []interactive.SummaryItem{
		{Label: i18n.Get("InteractiveInitSummaryMode"), Value: localizedInitMode(resolved.mode)},
		{Label: i18n.Get("InteractiveInitSummaryToolLocale"), Value: resolved.locale},
		{Label: i18n.Get("InteractiveInitSummarySkillsLocale"), Value: resolved.skillsLocale},
		{Label: i18n.Get("InteractiveInitSummaryAgent"), Value: resolved.agent},
		{Label: i18n.Get("InteractiveInitSummarySkills"), Value: resolved.skills},
	})

	confirmed, err := interactive.Confirm(i18n.Get("InteractiveConfirmExecute"), i18n.Get("InteractiveYes"), i18n.Get("InteractiveNo"), true)
	if err != nil {
		return resolved, err
	}
	if !confirmed {
		return resolved, interactive.ErrCanceled
	}
	return resolved, nil
}

func bannerTags() []interactive.BannerTag {
	return []interactive.BannerTag{
		{Label: "cli " + metadata.ProgramVersion},
		{Label: "prompts " + shortHash(metadata.HashOrUnavailable(metadata.PromptTemplatesHash(embedfs.FS)))},
	}
}

func shortHash(hash string) string {
	if len(hash) <= 8 {
		return hash
	}
	return hash[:8]
}

func withInitDefaults(opts commandOptions) commandOptions {
	opts.locale = config.NormalizeToolLocale(opts.locale)
	opts.skillsLocale = config.NormalizeSkillsLocale(opts.skillsLocale)
	opts.mode = normalizeInitMode(opts.mode)
	if opts.agent == "" {
		opts.agent = defaultInitAgent
	}
	if opts.skills == "" {
		opts.skills = defaultInitSkills
	}
	opts.workspace = opts.mode == domain.ModeWorkspace || opts.workspace
	if opts.workspace {
		opts.mode = domain.ModeWorkspace
	}
	return opts
}

func agentOptions() []interactive.Option[string] {
	return []interactive.Option[string]{
		{Value: "claude", Title: "claude"},
		{Value: "codex", Title: "codex"},
	}
}

func localizedInitMode(mode string) string {
	if mode == domain.ModeWorkspace {
		return i18n.Get("InteractiveInitModeWorkspace")
	}
	return i18n.Get("InteractiveInitModeProject")
}
