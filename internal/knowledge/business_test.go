package knowledge

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestBusinessPatternGroupsDoNotNameSingleScopeAsBusinessDomain(t *testing.T) {
	pattern := domain.NewPattern("existing-capability", "Existing Capability", domain.CategoryBusiness)
	pattern.ScopePath = "plugins/capability_lifecycle"

	groups := BusinessPatternGroups("en-US", []domain.Pattern{*pattern})

	require.Len(t, groups, 1)
	require.Equal(t, businessFallbackGroupID, groups[0].ID)
}

func TestBusinessPatternGroupsDoNotInventDomainFromPatternText(t *testing.T) {
	pattern := domain.NewPattern("existing-capability", "Existing Capability", domain.CategoryBusiness)
	pattern.SetDescription("audit log before after")

	groups := BusinessPatternGroups("zh-CN", []domain.Pattern{*pattern})

	require.Len(t, groups, 1)
	require.Equal(t, businessFallbackGroupID, groups[0].ID)
}

func TestBusinessPatternGroupsDoNotInferScopeFromEvidencePath(t *testing.T) {
	pattern := domain.NewPattern("existing-capability", "Existing Capability", domain.CategoryBusiness)
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "src/capability/entry.ext", Symbol: "ExistingEntry"}}

	groups := BusinessPatternGroups("en-US", []domain.Pattern{*pattern})

	require.Len(t, groups, 1)
	require.Equal(t, businessFallbackGroupID, groups[0].ID)
}

func TestBusinessGroupKeywordsDoNotExpandFromBroadPatternSignals(t *testing.T) {
	pattern := domain.NewPattern("domain-action-state", "Domain Action State", domain.CategoryBusiness)
	pattern.ScopePath = "components/billing"

	groups := BusinessPatternGroups("en-US", []domain.Pattern{*pattern})

	require.Len(t, groups, 1)
	require.Equal(t, []string{"Domain Action State", "domain-action-state"}, groups[0].Summary.Keywords)
}

func TestBusinessPatternGroupsDoNotUseSingleScopeOverEvidencePath(t *testing.T) {
	pattern := domain.NewPattern("pattern", "Pattern", domain.CategoryBusiness)
	pattern.ScopePath = "components/identity"
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "src/adapter/entry.ext"}}

	groups := BusinessPatternGroups("en-US", []domain.Pattern{*pattern})

	require.Len(t, groups, 1)
	require.Equal(t, businessFallbackGroupID, groups[0].ID)
}

func TestBusinessPatternGroupsKeepRepeatedScopeInFallbackWithoutFocus(t *testing.T) {
	first := domain.NewPattern("first", "First", domain.CategoryBusiness)
	first.ScopePath = "components/identity"
	second := domain.NewPattern("second", "Second", domain.CategoryBusiness)
	second.ScopePath = "components/identity"

	groups := BusinessPatternGroups("en-US", []domain.Pattern{*first, *second})

	require.Len(t, groups, 1)
	require.Equal(t, businessFallbackGroupID, groups[0].ID)
}

func TestTitleFromWordsPreservesUnicode(t *testing.T) {
	require.Equal(t, "Éclair", TitleFromWords([]string{"éclair"}))
}
