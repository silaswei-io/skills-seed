package generator

import (
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type businessMethodIndex struct {
	Sections []businessMethodSection
	Groups   []businessMethodGroup
	Total    int
}

type businessMethodSection struct {
	ID          string
	Title       string
	Description string
	Groups      []businessMethodGroup
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
	Role        businessMethodRole
	Score       int
}

type businessMethodRole string

const (
	businessMethodRoleBusiness   businessMethodRole = "business"
	businessMethodRoleSupporting businessMethodRole = "supporting"
)

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
	sections := []businessMethodSection{
		{
			ID:          string(businessMethodRoleBusiness),
			Title:       generatorText(locale, "GeneratorBusinessEntrySectionTitle"),
			Description: generatorText(locale, "GeneratorBusinessEntrySectionDescription"),
		},
		{
			ID:          string(businessMethodRoleSupporting),
			Title:       generatorText(locale, "GeneratorSupportingEntrySectionTitle"),
			Description: generatorText(locale, "GeneratorSupportingEntrySectionDescription"),
		},
	}
	groupsBySection := map[businessMethodRole]map[string]*businessMethodGroup{
		businessMethodRoleBusiness:   {},
		businessMethodRoleSupporting: {},
	}
	orderBySection := map[businessMethodRole][]string{
		businessMethodRoleBusiness:   {},
		businessMethodRoleSupporting: {},
	}
	for _, method := range methods {
		if strings.TrimSpace(method.Name) == "" {
			continue
		}
		module := businessMethodModule(method)
		id := module
		if id == "" {
			id = "project"
		}
		role, score := businessMethodClassify(method)
		groupsByID := groupsBySection[role]
		group, ok := groupsByID[id]
		if !ok {
			group = &businessMethodGroup{
				ID:    id,
				Title: businessMethodGroupTitle(module, locale),
			}
			groupsByID[id] = group
			orderBySection[role] = append(orderBySection[role], id)
		}
		view := businessMethodView{
			BusinessMethod: method,
			DisplayName:    businessMethodDisplayName(method, module),
			Module:         module,
			Role:           role,
			Score:          score,
		}
		view.Anchor = businessMethodAnchor(view.DisplayName)
		group.Methods = append(group.Methods, view)
	}

	index := businessMethodIndex{}
	for sectionIndex := range sections {
		role := businessMethodRole(sections[sectionIndex].ID)
		groupsByID := groupsBySection[role]
		order := orderBySection[role]
		sort.SliceStable(order, func(i, j int) bool {
			left := groupsByID[order[i]]
			right := groupsByID[order[j]]
			leftScore := businessMethodGroupScore(left)
			rightScore := businessMethodGroupScore(right)
			if leftScore != rightScore {
				return leftScore > rightScore
			}
			if len(left.Methods) != len(right.Methods) {
				return len(left.Methods) > len(right.Methods)
			}
			return left.Title < right.Title
		})
		for _, id := range order {
			group := groupsByID[id]
			sort.SliceStable(group.Methods, func(i, j int) bool {
				if group.Methods[i].Score != group.Methods[j].Score {
					return group.Methods[i].Score > group.Methods[j].Score
				}
				return group.Methods[i].DisplayName < group.Methods[j].DisplayName
			})
			group.Summary = businessMethodGroupSummary(*group, locale)
			index.Total += len(group.Methods)
			sections[sectionIndex].Groups = append(sections[sectionIndex].Groups, *group)
			index.Groups = append(index.Groups, *group)
		}
		if len(sections[sectionIndex].Groups) > 0 {
			index.Sections = append(index.Sections, sections[sectionIndex])
		}
	}
	return index
}

func businessMethodGroupScore(group *businessMethodGroup) int {
	if group == nil {
		return 0
	}
	score := 0
	for _, method := range group.Methods {
		score += method.Score
	}
	return score
}

func businessMethodClassify(method domain.BusinessMethod) (businessMethodRole, int) {
	text := strings.ToLower(strings.Join([]string{
		method.Name,
		method.Description,
		method.Usage,
		method.Type,
		method.Function,
		method.Prerequisites,
		method.Returns,
		method.DisplayLocation(),
	}, " "))
	score := 0
	if strings.EqualFold(strings.TrimSpace(method.Type), "domain") {
		score += 4
	}
	if containsAny(text,
		"business", "domain", "service", "workflow", "process", "orchestr", "state", "status",
		"permission", "auth", "access", "policy", "rule", "validate", "repository", "transaction",
		"usecase", "业务", "领域", "流程", "编排", "状态", "权限", "校验", "规则", "事务", "持久化",
	) {
		score += 3
	}
	if containsAny(text, "handler", "controller", "endpoint", "route", "api") {
		score += 1
	}
	if businessMethodHasInfrastructureSignal(text) {
		score -= 4
	}
	if containsAny(text, "/test", "_test.go", "mock", "fixture", "tools/", "cmd/", "script", "generate", "gen_", "codegen", "swagger", "curl") {
		score -= 3
	}
	if score >= 2 {
		return businessMethodRoleBusiness, score
	}
	return businessMethodRoleSupporting, score
}

func businessMethodHasInfrastructureSignal(text string) bool {
	return containsAny(text,
		" util", "utils", "helper", "common", "client", "rest", "http client", "rpc", "sdk",
		"middleware", "config", "bootstrap", "startup", "route", "router", "model layer",
		"工具", "通用", "客户端", "中间件", "配置", "启动", "路由", "生成",
	)
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
