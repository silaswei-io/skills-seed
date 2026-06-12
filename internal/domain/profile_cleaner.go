package domain

import (
	"strings"
	"time"
)

// CleanProjectProfile 清洗 profile 中的占位符和无效数据
func CleanProjectProfile(profile *ProjectProfile) *ProjectProfile {
	if profile == nil {
		return nil
	}

	cleaned := *profile

	cleaned.KeyModules = make([]ModuleInfo, 0, len(profile.KeyModules))
	for _, module := range profile.KeyModules {
		module.KeyMethods = FilterGeneratedPlaceholders(module.KeyMethods)
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
		if isInvalidValidationCommand(command.Command) {
			continue
		}
		key := strings.ToLower(command.Command + "\x00" + command.When)
		if seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, command)
	}
	return cleaned
}

func isInvalidValidationCommand(command string) bool {
	if command == "" {
		return true
	}
	upper := strings.ToUpper(command)
	return strings.Contains(upper, "TODO") || strings.Contains(command, "待确认")
}

// FilterGeneratedPlaceholders 过滤掉空值和含 TODO 的占位符
func FilterGeneratedPlaceholders(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || strings.Contains(strings.ToUpper(trimmed), "TODO") {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
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
