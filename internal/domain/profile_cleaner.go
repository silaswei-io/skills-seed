package domain

import (
	"path/filepath"
	"strings"
	"time"
)

// CleanProjectProfile 清洗 profile 中的占位符和无效数据
func CleanProjectProfile(profile *ProjectProfile) *ProjectProfile {
	if profile == nil {
		return nil
	}

	cleaned := *profile
	cleaned.FrameworkPatterns = cleanProfilePatternList(profile.FrameworkPatterns, 30)
	cleaned.ConfigPatterns = cleanProfilePatternList(profile.ConfigPatterns, 30)

	cleaned.KeyModules = make([]ModuleInfo, 0, len(profile.KeyModules))
	seenModules := make(map[string]int, len(profile.KeyModules))
	for _, module := range profile.KeyModules {
		module.Name = strings.TrimSpace(module.Name)
		module.DisplayName = strings.TrimSpace(module.DisplayName)
		module.Path = normalizeProfileEvidencePath(module.Path)
		module.Description = strings.TrimSpace(module.Description)
		module.KeyMethods = FilterGeneratedPlaceholders(module.KeyMethods)
		module.Responsibilities = FilterGeneratedPlaceholders(module.Responsibilities)
		module.Dependencies = FilterGeneratedPlaceholders(module.Dependencies)
		module.Dependents = FilterGeneratedPlaceholders(module.Dependents)
		if module.Name == "" || module.Path == "" {
			continue
		}
		if module.DisplayName == "" {
			module.DisplayName = firstNonEmptyProfileValue(module.Name, module.Path)
		}
		key := moduleIdentityKey(module)
		if idx, ok := seenModules[key]; ok {
			cleaned.KeyModules[idx] = mergeModuleInfo(cleaned.KeyModules[idx], module)
			continue
		}
		seenModules[key] = len(cleaned.KeyModules)
		cleaned.KeyModules = append(cleaned.KeyModules, module)
	}

	cleaned.BusinessMethods = make([]BusinessMethod, 0, len(profile.BusinessMethods))
	now := time.Now()
	for _, method := range profile.BusinessMethods {
		if strings.TrimSpace(method.Name) != "" {
			method.NormalizeCodeLocation(nil, now)
			cleaned.BusinessMethods = append(cleaned.BusinessMethods, method)
		}
	}

	cleaned.CommonUtils = make([]UtilityFunction, 0, len(profile.CommonUtils))
	for _, utility := range profile.CommonUtils {
		utility.Name = strings.TrimSpace(utility.Name)
		utility.File = normalizeProfileEvidencePath(utility.File)
		utility.Signature = strings.TrimSpace(utility.Signature)
		utility.Description = strings.TrimSpace(utility.Description)
		utility.Usage = strings.TrimSpace(utility.Usage)
		if utility.Name != "" && utility.File != "" {
			cleaned.CommonUtils = append(cleaned.CommonUtils, utility)
		}
	}

	cleaned.ValidationCommands = CleanValidationCommands(profile.ValidationCommands)
	cleaned.EngineeringRules = cleanEngineeringRules(profile.EngineeringRules)

	return &cleaned
}

func cleanEngineeringRules(rules []EngineeringRule) []EngineeringRule {
	cleaned := make([]EngineeringRule, 0, len(rules))
	seen := make(map[string]bool, len(rules))
	for _, rule := range rules {
		rule.Title = strings.TrimSpace(rule.Title)
		rule.Rule = strings.TrimSpace(rule.Rule)
		rule.Source = normalizeProfileEvidencePath(rule.Source)
		rule.Evidence = cleanProfilePaths(rule.Evidence)
		key := rule.Source + "\x00" + strings.ToLower(rule.Title) + "\x00" + rule.Rule
		if rule.Title == "" || rule.Rule == "" || rule.Source == "" || seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, rule)
	}
	return cleaned
}

// CleanValidationCommands 清洗从项目证据中学习到的验证命令。
func CleanValidationCommands(commands []ValidationCommand) []ValidationCommand {
	cleaned := make([]ValidationCommand, 0, len(commands))
	seen := make(map[string]bool, len(commands))
	for _, command := range commands {
		command.Command = strings.TrimSpace(command.Command)
		command.When = strings.TrimSpace(command.When)
		command.Source = strings.TrimSpace(command.Source)
		command.Workdir = normalizeProfileEvidencePath(command.Workdir)
		command.ScopePaths = cleanProfilePaths(command.ScopePaths)
		command.Evidence = cleanProfilePaths(command.Evidence)
		command.Type = strings.TrimSpace(command.Type)
		if isInvalidValidationCommand(command.Command) {
			continue
		}
		kind := ClassifyValidationCommand(command)
		if kind == ValidationCommandOther {
			continue
		}
		command.Type = CanonicalValidationCommandType(kind, command.Type)
		key := strings.ToLower(command.Command + "\x00" + command.Workdir + "\x00" + strings.Join(command.ScopePaths, "\x00") + "\x00" + command.When)
		if seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, command)
	}
	return cleaned
}

func cleanProfilePaths(paths []string) []string {
	cleaned := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = normalizeProfileEvidencePath(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		cleaned = append(cleaned, path)
	}
	return cleaned
}

func normalizeProfilePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "." {
		return ""
	}
	return path
}

func normalizeProfileEvidencePath(path string) string {
	path = normalizeProfilePath(path)
	if isUnconfirmedProfileValue(path) {
		return ""
	}
	return path
}

func isInvalidValidationCommand(command string) bool {
	if command == "" {
		return true
	}
	upper := strings.ToUpper(command)
	return strings.Contains(upper, "TODO") || strings.Contains(command, "待确认")
}

func cleanProfilePatternList(values []string, limit int) []string {
	cleaned := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.Contains(strings.ToUpper(value), "TODO") {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, value)
		if limit > 0 && len(cleaned) >= limit {
			break
		}
	}
	return cleaned
}

// FilterGeneratedPlaceholders 过滤掉空值和含 TODO 的占位符
func FilterGeneratedPlaceholders(values []string) []string {
	filtered := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || strings.Contains(strings.ToUpper(trimmed), "TODO") || isUnconfirmedProfileValue(trimmed) {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		filtered = append(filtered, trimmed)
	}
	return filtered
}

func isUnconfirmedProfileValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	switch lower {
	case "unconfirmed", "unknown", "unknown path", "unknown location", "n/a", "na", "none", "null", "-", "未确认", "待确认", "未知", "无":
		return true
	default:
		return strings.Contains(lower, "unconfirmed") || strings.Contains(trimmed, "待确认") || strings.Contains(trimmed, "未确认")
	}
}

func moduleIdentityKey(module ModuleInfo) string {
	if module.Path != "" {
		return "path:" + strings.ToLower(module.Path)
	}
	return "name:" + strings.ToLower(module.Name)
}

func mergeModuleInfo(base, next ModuleInfo) ModuleInfo {
	if base.Name == "" {
		base.Name = next.Name
	}
	if base.DisplayName == "" {
		base.DisplayName = next.DisplayName
	}
	if base.Path == "" {
		base.Path = next.Path
	}
	if base.Description == "" {
		base.Description = next.Description
	}
	base.Responsibilities = mergeStringLists(base.Responsibilities, next.Responsibilities)
	base.Dependencies = mergeStringLists(base.Dependencies, next.Dependencies)
	base.Dependents = mergeStringLists(base.Dependents, next.Dependents)
	base.KeyMethods = mergeStringLists(base.KeyMethods, next.KeyMethods)
	return base
}

func firstNonEmptyProfileValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mergeStringLists(base, next []string) []string {
	values := append(append([]string(nil), base...), next...)
	return FilterGeneratedPlaceholders(values)
}

// ValidBusinessMethods 过滤掉无效的业务方法
func ValidBusinessMethods(methods []*BusinessMethod) []*BusinessMethod {
	valid := make([]*BusinessMethod, 0, len(methods))
	for _, method := range methods {
		if IsUsableBusinessMethod(method) {
			valid = append(valid, method)
		}
	}
	return valid
}

// IsUsableBusinessMethod 判断业务方法是否可用
func IsUsableBusinessMethod(method *BusinessMethod) bool {
	return method != nil &&
		strings.TrimSpace(method.Name) != "" &&
		strings.TrimSpace(method.DisplayLocation()) != ""
}
