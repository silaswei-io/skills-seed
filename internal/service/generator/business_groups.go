package generator

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

func businessPatternItems(patterns []domain.Pattern) []patternRenderModel {
	items := patternsForTemplate(patterns)
	for i := range items {
		items[i].ReferencePath = "./business/items/" + referenceFileID(items[i].ID) + ".md"
	}
	return items
}

type businessResponsibilityGroup struct {
	Title         string
	ReferencePath string
	Items         []patternRenderModel
}

// groupBusinessResponsibilities 仅用已审查标题的共同主题组织导航，不改写知识正文。
func groupBusinessResponsibilities(items []patternRenderModel) ([]businessResponsibilityGroup, []patternRenderModel) {
	themes := make([]string, len(items))
	for i := range items {
		for j := range items {
			if i == j {
				continue
			}
			prefix := sharedTitlePrefix(items[i].Name, items[j].Name)
			if len([]rune(prefix)) > len([]rune(themes[i])) {
				themes[i] = prefix
			}
		}
	}
	byTitle := make(map[string]*businessResponsibilityGroup)
	var singles []patternRenderModel
	for i, item := range items {
		if themes[i] == "" {
			singles = append(singles, item)
			continue
		}
		group := byTitle[themes[i]]
		if group == nil {
			group = &businessResponsibilityGroup{
				Title: themes[i], ReferencePath: "./business/groups/" + referenceFileID(themes[i])[:16] + ".md",
			}
			byTitle[themes[i]] = group
		}
		item.ReferencePath = "../items/" + referenceFileID(item.ID) + ".md"
		group.Items = append(group.Items, item)
	}
	groups := make([]businessResponsibilityGroup, 0, len(byTitle))
	for _, group := range byTitle {
		if len(group.Items) < 2 {
			item := group.Items[0]
			item.ReferencePath = "./business/items/" + referenceFileID(item.ID) + ".md"
			singles = append(singles, item)
			continue
		}
		groups = append(groups, *group)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Title < groups[j].Title })
	sort.SliceStable(singles, func(i, j int) bool { return singles[i].Name < singles[j].Name })
	return groups, singles
}

func sharedTitlePrefix(left, right string) string {
	var prefix []rune
	a, b := []rune(strings.TrimSpace(left)), []rune(strings.TrimSpace(right))
	numericDivergence := false
	for i, j := 0, 0; i < len(a) && j < len(b); {
		if unicode.IsSpace(a[i]) {
			i++
			continue
		}
		if unicode.IsSpace(b[j]) {
			j++
			continue
		}
		if a[i] != b[j] || unicode.IsPunct(a[i]) {
			numericDivergence = unicode.IsDigit(a[i]) && unicode.IsDigit(b[j])
			break
		}
		prefix = append(prefix, a[i])
		i++
		j++
	}
	if numericDivergence || len(prefix) < 2 || (len(prefix) < 3 && !unicode.Is(unicode.Han, prefix[0])) {
		return ""
	}
	return string(prefix)
}

func referenceFileID(identity string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))
}

func businessPatternItemPath(pattern patternRenderModel) string {
	return filepath.Join("references", "patterns", "business", "items", referenceFileID(pattern.ID)+".md")
}
