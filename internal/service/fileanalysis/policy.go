package fileanalysis

import (
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/repositoryscope"
	"github.com/silaswei-io/skills-seed/internal/service/repositoryscopeconfig"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
)

type SkipReason string

const (
	SkipReasonNone       SkipReason = ""
	SkipReasonExcluded   SkipReason = "excluded"
	SkipReasonDocument   SkipReason = "document"
	SkipReasonNonSource  SkipReason = "non-source"
	SkipReasonProcedure  SkipReason = "procedure"
	SkipReasonOutOfFocus SkipReason = "out-of-focus"
	SkipReasonUnreadable SkipReason = "unreadable"
)

type SelectionPolicy struct {
	Scope      repositoryscope.Scope
	SourceOnly bool
}

func NewSelectionPolicy(excludePatterns []string) SelectionPolicy {
	return SelectionPolicy{
		Scope:      repositoryscope.New(excludePatterns),
		SourceOnly: config.DefaultAnalyzeSourceFilesOnly,
	}
}

type Decision struct {
	Path    string
	Include bool
	Reason  SkipReason
}

func NewConfiguredSelectionPolicy(configRepo config.Reader, projectRoot string) SelectionPolicy {
	return SelectionPolicy{
		Scope:      repositoryscopeconfig.KnowledgeScope(configRepo, projectRoot),
		SourceOnly: config.DefaultAnalyzeSourceFilesOnly,
	}
}

func (p SelectionPolicy) Decide(path string) Decision {
	if p.IsExcluded(path) {
		return Decision{Path: path, Include: false, Reason: SkipReasonExcluded}
	}
	if !p.SourceOnly {
		return Decision{Path: path, Include: true, Reason: SkipReasonNone}
	}
	if sourcecode.IsProcedureSource(path) {
		return Decision{Path: path, Include: false, Reason: SkipReasonProcedure}
	}
	if sourcecode.IsAnalyzable(path) {
		return Decision{Path: path, Include: true, Reason: SkipReasonNone}
	}
	if sourcecode.IsDocument(path) {
		return Decision{Path: path, Include: false, Reason: SkipReasonDocument}
	}
	return Decision{Path: path, Include: false, Reason: SkipReasonNonSource}
}

func (p SelectionPolicy) IsExcluded(path string) bool {
	return !p.Scope.AllowsKnowledge(path)
}
