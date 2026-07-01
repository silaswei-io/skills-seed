package domain

import (
	"strings"
	"time"
)

// ProjectProfileDelta 是当前代码学习阶段返回的项目画像增量。
// 它只表达本次证据明确改变的项目事实，不能替代完整 ProjectProfile。
type ProjectProfileDelta struct {
	Frameworks         []string            `json:"frameworks,omitempty"`
	Dependencies       []string            `json:"dependencies,omitempty"`
	Layers             []ArchitectureLayer `json:"layers,omitempty"`
	KeyModules         []ModuleInfo        `json:"key_modules,omitempty"`
	CommonUtils        []UtilityFunction   `json:"common_utils,omitempty"`
	ConfigPatterns     []string            `json:"config_patterns,omitempty"`
	FrameworkPatterns  []string            `json:"framework_patterns,omitempty"`
	BusinessMethods    []BusinessMethod    `json:"business_methods,omitempty"`
	ValidationCommands []ValidationCommand `json:"validation_commands,omitempty"`
	Summary            string              `json:"summary,omitempty"`
	Architecture       string              `json:"architecture,omitempty"`
	Structure          string              `json:"structure,omitempty"`
	DependencyGraph    string              `json:"dependency_graph,omitempty"`
	DataFlow           string              `json:"data_flow,omitempty"`
}

// IsZero 判断 delta 是否没有任何可合并内容。
func (d ProjectProfileDelta) IsZero() bool {
	return len(d.Frameworks) == 0 &&
		len(d.Dependencies) == 0 &&
		len(d.Layers) == 0 &&
		len(d.KeyModules) == 0 &&
		len(d.CommonUtils) == 0 &&
		len(d.ConfigPatterns) == 0 &&
		len(d.FrameworkPatterns) == 0 &&
		len(d.BusinessMethods) == 0 &&
		len(d.ValidationCommands) == 0 &&
		d.Summary == "" &&
		d.Architecture == "" &&
		d.Structure == "" &&
		d.DependencyGraph == "" &&
		d.DataFlow == ""
}

// HasMergeableFacts 判断 delta 是否包含可从局部分析单元安全合并的结构化事实。
func (d ProjectProfileDelta) HasMergeableFacts() bool {
	return len(d.Frameworks) > 0 ||
		len(d.Dependencies) > 0 ||
		len(d.Layers) > 0 ||
		len(d.KeyModules) > 0 ||
		len(d.CommonUtils) > 0 ||
		len(d.ConfigPatterns) > 0 ||
		len(d.FrameworkPatterns) > 0 ||
		len(d.BusinessMethods) > 0 ||
		len(d.ValidationCommands) > 0
}

// ApplyProjectProfileDelta 把本次学习得到的增量事实合并到已有项目画像。
func ApplyProjectProfileDelta(base *ProjectProfile, delta ProjectProfileDelta, projectName, language string) *ProjectProfile {
	if base == nil {
		base = &ProjectProfile{}
	}
	merged := *base
	if projectName != "" {
		merged.ProjectName = projectName
	}
	if merged.ProjectName == "" {
		merged.ProjectName = base.ProjectName
	}
	if language != "" {
		merged.Language = language
	} else if merged.Language == "" {
		merged.Language = base.Language
	}

	merged.Frameworks = mergeStrings(merged.Frameworks, delta.Frameworks)
	merged.Dependencies = mergeStrings(merged.Dependencies, delta.Dependencies)
	merged.ConfigPatterns = mergeStrings(merged.ConfigPatterns, delta.ConfigPatterns)
	merged.FrameworkPatterns = mergeStrings(merged.FrameworkPatterns, delta.FrameworkPatterns)
	merged.Layers = mergeLayers(merged.Layers, delta.Layers)
	merged.KeyModules = mergeModules(merged.KeyModules, delta.KeyModules)
	merged.CommonUtils = mergeUtilities(merged.CommonUtils, delta.CommonUtils)
	merged.BusinessMethods = mergeBusinessMethods(merged.BusinessMethods, delta.BusinessMethods)
	merged.ValidationCommands = CleanValidationCommands(append(merged.ValidationCommands, delta.ValidationCommands...))

	merged.GeneratedAt = time.Now().Format("2006-01-02 15:04:05")
	return CleanProjectProfile(&merged)
}

func mergeStrings(base, delta []string) []string {
	seen := make(map[string]bool, len(base)+len(delta))
	out := make([]string, 0, len(base)+len(delta))
	for _, value := range append(append([]string{}, base...), delta...) {
		key := canonicalTextKey(value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func mergeLayers(base, delta []ArchitectureLayer) []ArchitectureLayer {
	byKey := make(map[string]int, len(base)+len(delta))
	out := append([]ArchitectureLayer{}, base...)
	for i, layer := range out {
		byKey[canonicalTextKey(layer.Name)] = i
	}
	for _, layer := range delta {
		key := canonicalTextKey(layer.Name)
		if key == "" {
			continue
		}
		if idx, ok := byKey[key]; ok {
			out[idx] = layer
			continue
		}
		byKey[key] = len(out)
		out = append(out, layer)
	}
	return out
}

func mergeModules(base, delta []ModuleInfo) []ModuleInfo {
	byKey := make(map[string]int, len(base)+len(delta))
	out := append([]ModuleInfo{}, base...)
	for i, module := range out {
		byKey[moduleKey(module)] = i
	}
	for _, module := range delta {
		key := moduleKey(module)
		if key == "" {
			continue
		}
		if idx, ok := byKey[key]; ok {
			out[idx] = module
			continue
		}
		byKey[key] = len(out)
		out = append(out, module)
	}
	return out
}

func mergeUtilities(base, delta []UtilityFunction) []UtilityFunction {
	byKey := make(map[string]int, len(base)+len(delta))
	out := append([]UtilityFunction{}, base...)
	for i, utility := range out {
		byKey[utilityKey(utility)] = i
	}
	for _, utility := range delta {
		key := utilityKey(utility)
		if key == "" {
			continue
		}
		if idx, ok := byKey[key]; ok {
			out[idx] = utility
			continue
		}
		byKey[key] = len(out)
		out = append(out, utility)
	}
	return out
}

func mergeBusinessMethods(base, delta []BusinessMethod) []BusinessMethod {
	byKey := make(map[string]int, len(base)+len(delta))
	out := append([]BusinessMethod{}, base...)
	for i, method := range out {
		byKey[businessMethodKey(method)] = i
	}
	for _, method := range delta {
		key := businessMethodKey(method)
		if key == "" {
			continue
		}
		if idx, ok := byKey[key]; ok {
			out[idx] = method
			continue
		}
		byKey[key] = len(out)
		out = append(out, method)
	}
	return out
}

func moduleKey(module ModuleInfo) string {
	if key := canonicalTextKey(module.Path); key != "" {
		return key
	}
	return canonicalTextKey(module.Name)
}

func utilityKey(utility UtilityFunction) string {
	if key := canonicalTextKey(utility.File + "\x00" + utility.Name); key != "" {
		return key
	}
	return canonicalTextKey(utility.Signature)
}

func businessMethodKey(method BusinessMethod) string {
	if key := canonicalTextKey(method.DisplayLocation()); key != "" {
		return key
	}
	if key := canonicalTextKey(method.Function); key != "" {
		return key
	}
	return canonicalTextKey(method.Name)
}

func canonicalTextKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
