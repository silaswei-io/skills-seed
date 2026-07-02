package initcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/interactive"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

const (
	defaultInitAgent  = "claude"
	defaultInitSkills = "claude"
)

type initExistingAction string

const (
	initExistingInspect initExistingAction = "inspect"
	initExistingReset   initExistingAction = "reset"
)

func shouldRunInteractiveInit(cmd *cobra.Command, opts commandOptions) bool {
	return interactive.IsTerminal() && !opts.noInteractive && !hasAnyChangedInitFlag(cmd)
}

func isProjectInitialized(projectRoot string) (bool, error) {
	_, err := os.Stat(filepath.Join(projectRoot, ".skills-seed"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
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

	agent, err := interactive.Select(i18n.Get("InteractiveInitAgent"), agentOptions(), resolved.agent)
	if err != nil {
		return resolved, err
	}
	resolved.agent = agent
	if opts.skills == "" {
		resolved.skills = agent
	}

	totalParallelism, err := interactive.Int(i18n.Get("InteractiveInitAgentTotalParallelism"), resolved.agentTotalParallelism, 1)
	if err != nil {
		return resolved, err
	}
	resolved.agentTotalParallelism = totalParallelism

	customizeAdvanced, err := interactive.Confirm(i18n.Get("InteractiveInitCustomizeAdvanced"), i18n.Get("InteractiveYes"), i18n.Get("InteractiveNo"), false)
	if err != nil {
		return resolved, err
	}
	if customizeAdvanced {
		learningMode, err := interactive.Select(i18n.Get("InteractiveInitLearningMode"), learningModeOptions(), resolved.learningMode)
		if err != nil {
			return resolved, err
		}
		resolved.learningMode = learningMode

		learningScope, err := interactive.Select(i18n.Get("InteractiveInitLearningScope"), learningScopeOptions(), resolved.learningScope)
		if err != nil {
			return resolved, err
		}
		resolved.learningScope = learningScope

		skillsLocale, err := interactive.Select(i18n.Get("InteractiveInitSkillsLocale"), []interactive.Option[string]{
			{Value: i18n.LocaleEnglish, Title: "en-US"},
			{Value: i18n.LocaleChinese, Title: "zh-CN"},
		}, resolved.skillsLocale)
		if err != nil {
			return resolved, err
		}
		resolved.skillsLocale = skillsLocale

		skills, err := interactive.Select(i18n.Get("InteractiveInitSkills"), agentOptions(), resolved.skills)
		if err != nil {
			return resolved, err
		}
		resolved.skills = skills
	}

	projectCount := detectedWorkspaceProjectCount(resolved.mode)
	interactive.PrintSummary(cmd.OutOrStdout(), i18n.Get("InteractiveInitSummaryTitle"), []interactive.SummaryItem{
		{Label: i18n.Get("InteractiveInitSummaryMode"), Value: localizedInitMode(resolved.mode)},
		{Label: i18n.Get("InteractiveInitSummaryToolLocale"), Value: resolved.locale},
		{Label: i18n.Get("InteractiveInitSummarySkillsLocale"), Value: resolved.skillsLocale},
		{Label: i18n.Get("InteractiveInitSummaryAgent"), Value: resolved.agent},
		{Label: i18n.Get("InteractiveInitSummaryAgentTotalParallelism"), Value: fmt.Sprintf("%d", resolved.agentTotalParallelism)},
		{Label: i18n.Get("InteractiveInitSummaryParallelismPlan"), Value: initParallelismPlanSummary(resolved.mode, resolved.agentTotalParallelism, projectCount)},
		{Label: i18n.Get("InteractiveInitSummaryLearningMode"), Value: string(resolved.learningMode)},
		{Label: i18n.Get("InteractiveInitSummaryLearningScope"), Value: string(resolved.learningScope)},
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

func resolveInteractiveExistingInit(cmd *cobra.Command, projectRoot string) (initExistingAction, error) {
	interactive.PrintBanner(cmd.OutOrStdout(), "skills-seed", "Project skills for AI agents", bannerTags())
	configPath := filepath.Join(projectRoot, ".skills-seed", "config.yaml")
	action, err := interactive.Select(i18n.Get("InteractiveInitAlreadyInitializedTitle"), []interactive.Option[initExistingAction]{
		{Value: initExistingInspect, Title: i18n.Get("InteractiveInitInspect"), Description: i18n.GetWithParams("InteractiveInitInspectDesc", map[string]interface{}{"Path": configPath})},
		{Value: initExistingReset, Title: i18n.Get("InteractiveInitReset"), Description: i18n.Get("InteractiveInitResetDesc")},
		{Value: "", Title: i18n.Get("InteractiveCancel")},
	}, initExistingInspect)
	if err != nil {
		return "", err
	}
	if action == "" {
		return "", interactive.ErrCanceled
	}
	return action, nil
}

func detectedWorkspaceProjectCount(mode string) int {
	if mode != domain.ModeWorkspace {
		return 0
	}
	projectRoot, err := os.Getwd()
	if err != nil {
		return 0
	}
	return len(workspacediscovery.DiscoverProjects(projectRoot))
}

func initParallelismPlanSummary(mode string, totalParallelism, projectCount int) string {
	if mode != domain.ModeWorkspace {
		return i18n.GetWithParams("InteractiveInitParallelismPlanProject", map[string]interface{}{
			"Total":           totalParallelism,
			"UnitParallelism": totalParallelism,
		})
	}
	workspaceParallelism, unitParallelism := allocateWorkspaceParallelism(totalParallelism, projectCount)
	return i18n.GetWithParams("InteractiveInitParallelismPlanWorkspace", map[string]interface{}{
		"Total":                totalParallelism,
		"Projects":             projectCount,
		"WorkspaceParallelism": workspaceParallelism,
		"UnitParallelism":      unitParallelism,
	})
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
	if opts.agentTotalParallelism <= 0 {
		opts.agentTotalParallelism = 1
	}
	if opts.learningMode == "" {
		opts.learningMode = config.LearningModeNormal
	}
	opts.learningMode = config.NormalizeLearningMode(string(opts.learningMode))
	if opts.learningScope == "" {
		opts.learningScope = config.LearningScopeFlow
	}
	opts.learningScope = config.NormalizeLearningScope(string(opts.learningScope))
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

func learningModeOptions() []interactive.Option[config.LearningMode] {
	return []interactive.Option[config.LearningMode]{
		{Value: config.LearningModeNormal, Title: "normal", Description: i18n.Get("InteractiveInitLearningModeNormalDesc")},
		{Value: config.LearningModeFast, Title: "fast", Description: i18n.Get("InteractiveInitLearningModeFastDesc")},
		{Value: config.LearningModeDeep, Title: "deep", Description: i18n.Get("InteractiveInitLearningModeDeepDesc")},
	}
}

func learningScopeOptions() []interactive.Option[config.LearningScope] {
	return []interactive.Option[config.LearningScope]{
		{Value: config.LearningScopeFlow, Title: "flow", Description: i18n.Get("InteractiveInitLearningScopeFlowDesc")},
		{Value: config.LearningScopeDomain, Title: "domain", Description: i18n.Get("InteractiveInitLearningScopeDomainDesc")},
		{Value: config.LearningScopeModule, Title: "module", Description: i18n.Get("InteractiveInitLearningScopeModuleDesc")},
	}
}

func localizedInitMode(mode string) string {
	if mode == domain.ModeWorkspace {
		return i18n.Get("InteractiveInitModeWorkspace")
	}
	return i18n.Get("InteractiveInitModeProject")
}
