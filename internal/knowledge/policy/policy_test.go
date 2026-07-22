package policy

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestLearnedPatternWithSourceEvidenceIsReusableSolution(t *testing.T) {
	pattern := domain.NewPattern("existing-capability", "Existing Capability", domain.CategoryBusiness)
	pattern.Source = domain.SourceLearnedCurrent
	pattern.SetDescription("The existing entry handles the capability within its current integration boundary.")
	pattern.SetRule("Inspect and prefer ExistingEntry; extend it only when the current boundary does not fit.")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "src/capability.ext", Line: 10, Symbol: "ExistingEntry", Kind: "function"}}

	assert.Equal(t, StrengthSolution, EvaluatePattern(*pattern).Strength)
	assert.Equal(t, pattern.Description, DisplayPatternText(*pattern))
}

func TestUserPatternRemainsAuthoritativeRule(t *testing.T) {
	pattern := domain.NewPattern("maintained-rule", "Maintained Rule", domain.CategoryStructure)
	pattern.Source = domain.SourceUserDefined
	pattern.SetDescription("Applies to related changes.")
	pattern.SetRule("Use the maintained project boundary.")

	assert.Equal(t, StrengthRule, EvaluatePattern(*pattern).Strength)
	assert.Equal(t, pattern.Rule, DisplayPatternText(*pattern))
}

func TestLearnedPatternWithoutSourceEvidenceIsObservation(t *testing.T) {
	pattern := domain.NewPattern("unverified", "Unverified", domain.CategoryConfig)
	pattern.Source = domain.SourceLearned
	pattern.ScopePath = "src/component"
	pattern.Metrics.EvidenceCount = 8
	pattern.SetDescription("A possible local behavior.")
	pattern.SetRule("Prefer a presumed entry.")

	assert.Equal(t, StrengthObservation, EvaluatePattern(*pattern).Strength)
	assert.Equal(t, pattern.Description, DisplayPatternText(*pattern))
}

func TestPatternEvidenceCountCountsIndependentSourceFiles(t *testing.T) {
	pattern := domain.NewPattern("evidence", "Evidence", domain.CategoryConcurrency)
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "src/component.ext", Line: 10, Symbol: "Entry"},
		{Path: "src/component.ext", Line: 20, Symbol: "Helper"},
		{Path: "src/integration.ext", Line: 30, Symbol: "Adapter"},
	}
	pattern.Metrics.EvidenceCount = 9

	assert.Equal(t, 2, PatternEvidenceCount(*pattern))
}

func TestFileEvidenceIsReusableWithoutSourceExcerpt(t *testing.T) {
	pattern := domain.NewPattern("file-backed", "File Backed", domain.CategoryConfig)
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "config/settings.ext", Kind: "file"}}

	assert.Equal(t, StrengthSolution, EvaluatePattern(*pattern).Strength)
}

func TestDisplayPatternTextPreservesLearnedSemantics(t *testing.T) {
	pattern := domain.NewPattern("learned", "Learned", domain.CategoryStructure)
	pattern.Source = domain.SourceLearnedCurrent
	pattern.SetDescription("所有 Logic 必须严格使用现有结构")

	text := DisplayPatternText(*pattern)

	assert.Equal(t, pattern.Description, text)
}

func TestDisplayPatternTextUsesLearnedRuleWhenDescriptionIsMissing(t *testing.T) {
	pattern := domain.NewPattern("learned-rule", "Learned Rule", domain.CategoryError)
	pattern.Source = domain.SourceLearnedCurrent
	pattern.SetRule("All failures must include operation context")

	text := DisplayPatternText(*pattern)

	assert.Equal(t, pattern.Rule, text)
}
