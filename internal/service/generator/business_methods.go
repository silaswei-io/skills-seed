package generator

import (
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/silaswei-io/skills-seed/internal/domain"
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
	Anchor      string
	Module      string
}

var genericBusinessMethodNames = map[string]bool{
	"execute": true,
	"handle":  true,
	"init":    true,
	"main":    true,
	"run":     true,
	"rune":    true,
	"start":   true,
}

func buildBusinessMethodIndex(methods []domain.BusinessMethod, locale string) businessMethodIndex {
	groupsByID := map[string]*businessMethodGroup{}
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
		view := businessMethodView{
			BusinessMethod: method,
			DisplayName:    businessMethodDisplayName(method, module),
			Module:         module,
		}
		view.Anchor = businessMethodAnchor(view.DisplayName)
		group.Methods = append(group.Methods, view)
	}

	sort.SliceStable(order, func(i, j int) bool {
		left := groupsByID[order[i]]
		right := groupsByID[order[j]]
		if len(left.Methods) != len(right.Methods) {
			return len(left.Methods) > len(right.Methods)
		}
		return left.Title < right.Title
	})

	index := businessMethodIndex{Groups: make([]businessMethodGroup, 0, len(order))}
	for _, id := range order {
		group := groupsByID[id]
		sort.SliceStable(group.Methods, func(i, j int) bool {
			return group.Methods[i].DisplayName < group.Methods[j].DisplayName
		})
		group.Summary = businessMethodGroupSummary(*group, locale)
		index.Total += len(group.Methods)
		index.Groups = append(index.Groups, *group)
	}
	return index
}

func businessMethodGroupTitle(module, locale string) string {
	if module == "" || module == "project" {
		return localizedText(locale, "项目入口", "Project Entry Points")
	}
	words := splitBusinessGroupWords(module)
	if len(words) == 0 {
		return module
	}
	return titleFromWords(words)
}

func businessMethodGroupSummary(group businessMethodGroup, locale string) string {
	if group.ID == "project" {
		return localizedText(locale, "未能稳定归属到具体模块的项目级入口。", "Project-level entry points that cannot be assigned to a stable module.")
	}
	return localizedText(locale,
		"同一模块或相邻路径下的入口方法，优先按本组定位复用能力。",
		"Entry points from the same module or nearby paths; use this group first when locating reusable behavior.",
	)
}

func businessMethodDisplayName(method domain.BusinessMethod, module string) string {
	name := strings.TrimSpace(method.Name)
	if name == "" {
		return ""
	}
	if !isGenericBusinessMethodName(name) {
		return name
	}
	context := firstNonEmptyString(businessMethodReceiver(method.Function), module, businessMethodPathStem(method.DisplayLocation()))
	if context == "" {
		return name
	}
	return context + "." + name
}

func businessMethodModule(method domain.BusinessMethod) string {
	location := method.DisplayLocation()
	if location == "" {
		return ""
	}
	path := strings.Split(filepath.ToSlash(location), ":")[0]
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	if filepath.Ext(parts[len(parts)-1]) != "" {
		parts = parts[:len(parts)-1]
	}
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" || isGenericBusinessPathPart(part) {
			continue
		}
		return part
	}
	return ""
}

func businessMethodPathStem(location string) string {
	location = strings.Split(filepath.ToSlash(location), ":")[0]
	base := filepath.Base(location)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if isGenericBusinessPathPart(base) {
		return ""
	}
	return base
}

func businessMethodReceiver(signature string) string {
	signature = strings.TrimSpace(signature)
	start := strings.Index(signature, "func (")
	if start < 0 {
		return ""
	}
	rest := signature[start+len("func ("):]
	end := strings.Index(rest, ")")
	if end < 0 {
		return ""
	}
	fields := strings.Fields(strings.TrimSpace(rest[:end]))
	if len(fields) == 0 {
		return ""
	}
	receiver := fields[len(fields)-1]
	receiver = strings.TrimLeft(receiver, "*")
	receiver = strings.TrimPrefix(receiver, "[]")
	receiver = strings.TrimSpace(receiver)
	if receiver == "" {
		return ""
	}
	if idx := strings.LastIndex(receiver, "."); idx >= 0 {
		receiver = receiver[idx+1:]
	}
	return receiver
}

func isGenericBusinessMethodName(name string) bool {
	return genericBusinessMethodNames[strings.ToLower(strings.TrimSpace(name))]
}

func businessMethodAnchor(value string) string {
	text := strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	previousHyphen := false
	for _, r := range text {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			previousHyphen = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			previousHyphen = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if !previousHyphen && builder.Len() > 0 {
				builder.WriteRune('-')
				previousHyphen = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}
