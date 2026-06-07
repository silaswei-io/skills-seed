package domain

import (
	"strings"
	"time"
)

// WorkspaceProjectOverride 描述 workspace 子项目覆盖字段，
// 用于 NewProjectSpecFromProfile 中避免 domain → config 依赖。
type WorkspaceProjectOverride struct {
	ID       string
	Path     string
	Type     string
	Language string
}

// NewProjectSpecFromProfile 根据 profile 和 patterns 构建项目规范。
// project 参数为可选的 workspace 子项目覆盖。
func NewProjectSpecFromProfile(profile *ProjectProfile, patterns []Pattern, project WorkspaceProjectOverride) *ProjectSpec {
	if profile == nil {
		return nil
	}

	spec := &ProjectSpec{
		ProjectName:       profile.ProjectName,
		Language:          profile.Language,
		Summary:           profile.Summary,
		ConfigPatterns:    append([]string(nil), profile.ConfigPatterns...),
		FrameworkPatterns: append([]string(nil), profile.FrameworkPatterns...),
		GeneratedAt:       time.Now().Format("2006-01-02 15:04:05"),
	}
	if project.ID != "" {
		spec.ProjectID = project.ID
		spec.ProjectName = project.ID
		spec.ScopePath = project.Path
		spec.WorkspaceRole = project.Type
		if project.Language != "" {
			spec.Language = project.Language
		}
	}
	if spec.ProjectName == "" {
		spec.ProjectName = "project"
	}
	if spec.Language == "" {
		spec.Language = "unknown"
	}

	for _, layer := range profile.Layers {
		spec.Boundaries = append(spec.Boundaries, ProjectSpecBoundary{
			Type:             "layer",
			Name:             layer.Name,
			Description:      layer.Description,
			Responsibilities: append([]string(nil), layer.Responsibilities...),
			Paths:            append([]string(nil), layer.Files...),
		})
	}
	for _, module := range profile.KeyModules {
		spec.Boundaries = append(spec.Boundaries, ProjectSpecBoundary{
			Type:             "module",
			Name:             module.Name,
			Description:      module.Description,
			Responsibilities: append([]string(nil), module.Responsibilities...),
			Paths:            []string{module.Path},
		})
	}

	for _, pattern := range StrongestPatterns(patterns, 12) {
		spec.PatternRules = append(spec.PatternRules, ProjectSpecPatternRule{
			Name:        pattern.Name,
			Category:    string(pattern.Category),
			Description: pattern.Description,
			Rule:        pattern.Rule,
			Confidence:  pattern.Confidence,
			Frequency:   pattern.Frequency,
		})
	}

	for _, method := range profile.BusinessMethods {
		spec.Touchpoints = append(spec.Touchpoints, ProjectSpecTouchpoint{
			Kind:        "business_method",
			Name:        method.Name,
			Path:        method.Location,
			Description: method.Description,
		})
	}
	for _, utility := range profile.CommonUtils {
		spec.Touchpoints = append(spec.Touchpoints, ProjectSpecTouchpoint{
			Kind:        "common_utility",
			Name:        utility.Name,
			Path:        utility.File,
			Description: utility.Description,
		})
	}

	return spec
}

// patternForTemplate 清除不可用的 BusinessMethod，用于模板渲染
func patternForTemplate(pattern Pattern) Pattern {
	if !IsUsableBusinessMethod(pattern.BusinessMethod) {
		pattern.BusinessMethod = nil
	}
	return pattern
}

// sanitizeName 将项目名转为 kebab-case 的 skill 名称
func sanitizeName(name string) string {
	var b strings.Builder
	previousHyphen := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			previousHyphen = false
			continue
		}
		if !previousHyphen {
			b.WriteRune('-')
			previousHyphen = true
		}
	}

	result := strings.Trim(b.String(), "-")
	if result == "" {
		result = "project"
	}
	return result
}
