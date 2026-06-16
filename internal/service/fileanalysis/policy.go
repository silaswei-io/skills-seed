package fileanalysis

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/gitignore"
	"github.com/silaswei-io/skills-seed/internal/utils/filefilter"
	"github.com/silaswei-io/skills-seed/internal/utils/sourcefiles"
)

type SkipReason string

const (
	SkipReasonNone       SkipReason = ""
	SkipReasonExcluded   SkipReason = "excluded"
	SkipReasonDocument   SkipReason = "document"
	SkipReasonNonSource  SkipReason = "non-source"
	SkipReasonOutOfFocus SkipReason = "out-of-focus"
	SkipReasonUnreadable SkipReason = "unreadable"
)

type SelectionPolicy struct {
	ExcludePatterns []string
	GitIgnore       *gitignore.Matcher
	SourceOnly      bool
}

func NewSelectionPolicy(excludePatterns []string) SelectionPolicy {
	return SelectionPolicy{
		ExcludePatterns: excludePatterns,
		SourceOnly:      config.DefaultAnalyzeSourceFilesOnly,
	}
}

type Decision struct {
	Path    string
	Include bool
	Reason  SkipReason
}

func NewConfiguredSelectionPolicy(configRepo config.Reader, projectRoot string) SelectionPolicy {
	policy := NewSelectionPolicy(ConfiguredLearnExcludes(configRepo, projectRoot))
	if configRepo == nil || !configRepo.GetFileFilterConfig().ApplyGitIgnore {
		return policy
	}
	matcher, err := gitignore.NewMatcher(context.Background(), projectRoot)
	if err == nil {
		policy.GitIgnore = matcher
	}
	return policy
}

func (p SelectionPolicy) Include(path string) (bool, SkipReason) {
	decision := p.Decide(path)
	return decision.Include, decision.Reason
}

func (p SelectionPolicy) Decide(path string) Decision {
	if p.IsExcluded(path) {
		return Decision{Path: path, Include: false, Reason: SkipReasonExcluded}
	}
	if !p.SourceOnly {
		return Decision{Path: path, Include: true, Reason: SkipReasonNone}
	}
	if sourcefiles.IsAnalyzable(path) {
		return Decision{Path: path, Include: true, Reason: SkipReasonNone}
	}
	if sourcefiles.IsDocument(path) {
		return Decision{Path: path, Include: false, Reason: SkipReasonDocument}
	}
	return Decision{Path: path, Include: false, Reason: SkipReasonNonSource}
}

func (p SelectionPolicy) IsExcluded(path string) bool {
	return filefilter.MatchExcluded(path, p.ExcludePatterns) || p.GitIgnore.Match(path)
}
