package learncurrent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	"github.com/silaswei-io/skills-seed/internal/knowledge"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/repositoryscopeconfig"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

func (r *learnCurrentProjectRun) saveProfileIfNeeded() error {
	profileStartedAt := time.Now()
	var profile *domain.ProjectProfile
	if r.projectionsCommitted() {
		return nil
	}
	if !r.refreshProfile {
		r.observer.noteSkip(skipProfileUnchanged)
	}
	if r.refreshProfile {
		label := i18n.Get("ProgressLearnCurrentSaveProfile")
		if err := r.steps.Run(label, func() error {
			var err error
			profile, err = r.refreshProjectProfile()
			if err != nil {
				return err
			}
			profile, err = r.verifyProjectProfile(r.ctx, profile)
			if err != nil {
				return err
			}
			if err := r.cont.ProfileRepo.Save(r.ctx, profile); err != nil {
				return err
			}
			return r.markProjectionsCommitted()
		}); err != nil {
			logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "command.learn_current.save_project_profile",
				"duration", time.Since(profileStartedAt),
				"error", err,
			)
			return fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentProfileFailed", map[string]interface{}{"Error": err.Error()}))
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.save_project_profile",
			"duration", time.Since(profileStartedAt),
			"profile_mode", r.opts.profileMode,
			"incremental_profile", r.existingProfile != nil && len(r.resolvedFocusPaths) > 0,
		)
		if r.opts.showDetailedLogs {
			logger.Info(i18n.Get("LearnCurrentProfileSaved"))
		}
	} else {
		label := i18n.Get("ProgressLearnCurrentSkipProfile")
		if err := r.steps.Run(label, func() error { return nil }); err != nil {
			return err
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.skip_project_profile",
			"duration", time.Since(profileStartedAt),
			"profile_mode", r.opts.profileMode,
		)
		if r.opts.showDetailedLogs {
			logger.Info(i18n.Get("LearnCurrentProfileSkipped"))
		}
		profile = r.existingProfile
		if profile == nil {
			var err error
			profile, err = r.cont.ProfileRepo.Get(r.ctx)
			if err != nil && !errors.Is(err, profilestore.ErrProfileNotFound) {
				return err
			}
		}
		var err error
		profile, err = r.verifyProjectProfile(r.ctx, profile)
		if err != nil {
			return err
		}
		if profile != nil && r.cont.ProfileRepo != nil {
			if err := r.cont.ProfileRepo.Save(r.ctx, profile); err != nil {
				return err
			}
		}
		if err := r.markProjectionsCommitted(); err != nil {
			return err
		}
	}
	return nil
}

func (r *learnCurrentProjectRun) markProjectionsCommitted() error {
	if r.analysisState == nil {
		return nil
	}
	r.analysisState.MarkProjectionsCommitted()
	return r.stateRepo.Save(r.ctx, r.analysisState)
}

func (r *learnCurrentProjectRun) verifyProjectProfile(ctx context.Context, profile *domain.ProjectProfile) (*domain.ProjectProfile, error) {
	if profile == nil || r.cont == nil || r.cont.PatternReader == nil {
		return profile, nil
	}
	patterns, err := r.cont.PatternReader.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	structuralConfig := config.StructuralConfig{Provider: config.StructuralProviderAuto}
	if r.cont.ConfigRepo != nil {
		structuralConfig = r.cont.ConfigRepo.GetCurrentLearningConfig().Structural
	}
	scope := repositoryscopeconfig.DefaultKnowledgeScope()
	if r.cont.ConfigRepo != nil {
		scope = repositoryscopeconfig.KnowledgeScope(r.cont.ConfigRepo, r.projectRoot)
	}
	verified, _, err := knowledge.VerifyProjectKnowledge(ctx, profile, domain.ActivePatterns(patterns), r.projectRoot, sourcecode.NewResolver(structuralConfig), scope)
	return verified, err
}

func (r *learnCurrentProjectRun) refreshProjectProfile() (*domain.ProjectProfile, error) {
	options := analyzer.AnalyzeProjectOptions{}
	if r.existingProfile != nil && len(r.effectiveFocusPaths) > 0 {
		options.ExistingProfile = r.existingProfile
		options.FocusPaths = r.effectiveFocusPaths
	}
	options.OnStage = func(label string) {
		r.patternStageDetail(i18n.Get("ProgressLearnCurrentSaveProfile"), label)
	}
	return r.cont.AnalyzerSvc.RefreshProjectProfile(r.ctx, r.projectRoot, r.projectName, r.currentLanguage, options)
}
