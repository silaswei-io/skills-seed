package generator

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/routing"
)

type businessMethodIndex struct {
	Groups []businessMethodGroup
	Total  int
}

type businessMethodGroup struct {
	ID      string
	Title   string
	Summary string
	Methods []businessMethodView
}

type businessMethodView struct {
	domain.BusinessMethod
	DisplayName string
	Module      string
}

func buildBusinessMethodIndex(methods []domain.BusinessMethod, locale string) businessMethodIndex {
	groupsByID := make(map[string]*businessMethodGroup)
	order := make([]string, 0)
	for _, method := range methods {
		if strings.TrimSpace(method.Name) == "" {
			continue
		}
		module := businessMethodModule(method)
		id := module
		if id == "" {
			id = "project"
		}
		group, ok := groupsByID[id]
		if !ok {
			group = &businessMethodGroup{
				ID:    id,
				Title: businessMethodGroupTitle(module, locale),
			}
			groupsByID[id] = group
			order = append(order, id)
		}
		group.Methods = append(group.Methods, businessMethodView{
			BusinessMethod: method,
			DisplayName:    strings.TrimSpace(method.Name),
			Module:         module,
		})
	}

	index := businessMethodIndex{}
	sort.SliceStable(order, func(i, j int) bool {
		left := groupsByID[order[i]]
		right := groupsByID[order[j]]
		if len(left.Methods) != len(right.Methods) {
			return len(left.Methods) > len(right.Methods)
		}
		return left.Title < right.Title
	})
	for _, id := range order {
		group := groupsByID[id]
		sort.SliceStable(group.Methods, func(i, j int) bool {
			if group.Methods[i].DisplayName != group.Methods[j].DisplayName {
				return group.Methods[i].DisplayName < group.Methods[j].DisplayName
			}
			return group.Methods[i].DisplayLocation() < group.Methods[j].DisplayLocation()
		})
		group.Summary = businessMethodGroupSummary(*group, locale)
		index.Total += len(group.Methods)
		index.Groups = append(index.Groups, *group)
	}
	return index
}

func businessMethodGroupTitle(module, locale string) string {
	if module == "" || module == "project" {
		return generatorText(locale, "GeneratorBusinessMethodProjectGroupTitle")
	}
	words := routing.SplitBusinessGroupWords(module)
	if len(words) == 0 {
		return module
	}
	return routing.TitleFromWords(words)
}

func businessMethodGroupSummary(group businessMethodGroup, locale string) string {
	if group.ID == "project" {
		return generatorText(locale, "GeneratorBusinessMethodProjectGroupSummary")
	}
	return generatorText(locale, "GeneratorBusinessMethodModuleGroupSummary")
}

func businessMethodModule(method domain.BusinessMethod) string {
	path := referencePathOnly(method.DisplayLocation())
	if path == "" || path == "." {
		return ""
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		return filepath.Base(dir)
	}
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return base
}
