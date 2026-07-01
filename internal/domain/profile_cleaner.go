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
		module.Path = strings.TrimSpace(module.Path)
		module.Description = strings.TrimSpace(module.Description)
		module.KeyMethods = FilterGeneratedPlaceholders(module.KeyMethods)
		module.Responsibilities = FilterGeneratedPlaceholders(module.Responsibilities)
		module.Dependencies = FilterGeneratedPlaceholders(module.Dependencies)
		module.Dependents = FilterGeneratedPlaceholders(module.Dependents)
		if module.Name == "" && module.Path == "" {
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
		if strings.TrimSpace(utility.Name) != "" {
			cleaned.CommonUtils = append(cleaned.CommonUtils, utility)
		}
	}

	cleaned.ValidationCommands = CleanValidationCommands(profile.ValidationCommands)

	return &cleaned
}

// CleanValidationCommands 清洗从项目证据中学习到的验证命令。
func CleanValidationCommands(commands []ValidationCommand) []ValidationCommand {
	cleaned := make([]ValidationCommand, 0, len(commands))
	seen := make(map[string]bool, len(commands))
	for _, command := range commands {
		command.Command = strings.TrimSpace(command.Command)
		command.When = strings.TrimSpace(command.When)
		command.Source = strings.TrimSpace(command.Source)
		command.Workdir = normalizeProfilePath(command.Workdir)
		command.ScopePaths = cleanProfilePaths(command.ScopePaths)
		command.Evidence = cleanProfilePaths(command.Evidence)
		command.Type = strings.TrimSpace(command.Type)
		if isInvalidValidationCommand(command.Command) {
			continue
		}
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
		path = normalizeProfilePath(path)
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
		key := profilePatternFamilyKey(value)
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

func profilePatternFamilyKey(value string) string {
	text := strings.ToLower(value)
	switch {
	case strings.Contains(text, "handler") && (strings.Contains(text, "httpx.parse") || strings.Contains(text, "errorctx") || strings.Contains(text, "okjsonctx") || strings.Contains(text, "请求")):
		return "handler-http-flow"
	case (strings.Contains(text, "httpx.parse") || strings.Contains(text, "errorctx") || strings.Contains(text, "okjsonctx")) && (strings.Contains(text, "响应") || strings.Contains(text, "请求") || strings.Contains(text, "context")):
		return "handler-http-flow"
	case strings.Contains(text, "logic") && (strings.Contains(text, "logx") || strings.Contains(text, "svcctx") || strings.Contains(text, "context") || strings.Contains(text, "构造")):
		return "logic-constructor"
	case strings.Contains(text, "logx") && (strings.Contains(text, "withcontext") || strings.Contains(text, "logger") || strings.Contains(text, "日志")):
		return "logic-logging"
	case strings.Contains(text, "status.wrap") || strings.Contains(text, "错误包装") || strings.Contains(text, "包装api错误"):
		return "status-wrap"
	case strings.Contains(text, "condition") && (strings.Contains(text, "查询") || strings.Contains(text, "database") || strings.Contains(text, "数据库")):
		return "condition-query"
	case strings.Contains(text, "kmip") && strings.Contains(text, "baserequeststandard"):
		return "kmip-base-request"
	case strings.Contains(text, "kmip") && (strings.Contains(text, "success") || strings.Contains(text, "响应解析")):
		return "kmip-response-parse"
	default:
		return strings.ToLower(value)
	}
}

// FilterGeneratedPlaceholders 过滤掉空值和含 TODO 的占位符
func FilterGeneratedPlaceholders(values []string) []string {
	filtered := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || strings.Contains(strings.ToUpper(trimmed), "TODO") {
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
