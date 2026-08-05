package generator

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
)

type validationCommand = knowledge.ValidationCommand

func skillTriggerDescription(projectName, language, locale string, profile *domain.ProjectProfile) string {
	project := strings.TrimSpace(projectName)
	if profile != nil && strings.TrimSpace(profile.ProjectName) != "" {
		project = strings.TrimSpace(profile.ProjectName)
	}
	if project == "" {
		project = generatorText(locale, "GeneratorDefaultProjectName")
	}
	lang := strings.TrimSpace(language)
	if profile != nil && strings.TrimSpace(profile.Language) != "" {
		lang = strings.TrimSpace(profile.Language)
	}
	if lang == "" {
		lang = generatorText(locale, "GeneratorDefaultLanguageName")
	}
	return generatorTextWithParams(locale, "GeneratorSkillDescriptionDefault", map[string]interface{}{
		"Project":  project,
		"Language": lang,
	})
}

func validationCommands(profile *domain.ProjectProfile) []validationCommand {
	return knowledge.ValidationCommands(profile)
}

func validationCommandsForGeneration(profile *domain.ProjectProfile, discovered []domain.ValidationCommand) []domain.ValidationCommand {
	commands := make([]domain.ValidationCommand, 0, len(discovered))
	if profile != nil {
		for _, command := range domain.CleanValidationCommands(profile.ValidationCommands) {
			if validationCommandHasUserContextEvidence(command) {
				continue
			}
			commands = append(commands, command)
		}
	}
	commands = append(commands, discovered...)
	return domain.CleanValidationCommands(commands)
}

func validationCommandHasUserContextEvidence(command domain.ValidationCommand) bool {
	values := append([]string{command.Source}, command.Evidence...)
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if strings.Contains(value, "用户上下文") || strings.Contains(value, "user context") {
			return true
		}
	}
	return false
}

func validationGaps(profile *domain.ProjectProfile, matrix []ValidationMatrixItem, tests sourcecode.GoTestInventory, locale string) []string {
	commands := validationCommands(profile)
	hasTest := tests.HasModules()
	hasStaticCheck := false
	for _, command := range commands {
		switch knowledge.ValidationCommandKind(command) {
		case domain.ValidationCommandTest:
			hasTest = true
		case domain.ValidationCommandStaticCheck:
			hasStaticCheck = true
		}
	}

	gaps := make([]string, 0, 3)
	if !hasTest {
		gaps = append(gaps, generatorText(locale, "GeneratorValidationGapTest"))
	}
	if !hasStaticCheck {
		gaps = append(gaps, generatorText(locale, "GeneratorValidationGapStaticCheck"))
	}
	if len(matrix) == 0 && !tests.HasModules() {
		gaps = append(gaps, generatorText(locale, "GeneratorValidationGapScopedMatrix"))
	}
	return gaps
}

func testingCoverageGaps(profile *domain.ProjectProfile, patterns []domain.Pattern, tests sourcecode.GoTestInventory, locale string) []string {
	gaps := make([]string, 0, 8)
	testFiles := goTestFiles(tests)
	if profile != nil {
		for _, module := range profile.KeyModules {
			path := normalizedCoveragePath(module.Path)
			if path == "" || pathHasGoTest(path, testFiles) {
				continue
			}
			gaps = append(gaps, testingGapKeyModule(path, locale))
		}
	}
	for _, pattern := range patterns {
		if !patternNeedsCoverageGap(pattern) {
			continue
		}
		for _, path := range patternEvidenceDirs(pattern) {
			if pathHasGoTest(path, testFiles) {
				continue
			}
			gaps = append(gaps, testingGapPattern(path, pattern.Name, locale))
		}
	}
	gaps = uniqueLimited(gaps, 8)
	if len(gaps) == 0 {
		if testingInventoryHasExplicitGap(tests) {
			return nil
		}
		return []string{testingGapNone(locale)}
	}
	return gaps
}

func testingInventoryHasExplicitGap(tests sourcecode.GoTestInventory) bool {
	if len(tests.UnownedTestFiles) > 0 {
		return true
	}
	for _, module := range tests.Modules {
		if len(module.TestFiles) == 0 {
			return true
		}
	}
	return false
}

func goTestFiles(tests sourcecode.GoTestInventory) []string {
	files := make([]string, 0)
	for _, module := range tests.Modules {
		files = append(files, module.TestFiles...)
	}
	files = append(files, tests.UnownedTestFiles...)
	return files
}

func patternNeedsCoverageGap(pattern domain.Pattern) bool {
	switch pattern.Category {
	case domain.CategoryBusiness, domain.CategoryConcurrency, domain.CategoryDatabase, domain.CategoryAPI:
		return true
	default:
		return domain.IsHighRiskOperationalPattern(pattern)
	}
}

func patternEvidenceDirs(pattern domain.Pattern) []string {
	dirs := make([]string, 0, len(pattern.EvidenceLocations))
	for _, location := range pattern.EvidenceLocations {
		path := normalizedCoveragePath(location.Path)
		if path == "" {
			continue
		}
		dirs = append(dirs, coverageDir(path))
	}
	return uniqueLimited(dirs, 4)
}

func pathHasGoTest(path string, testFiles []string) bool {
	path = normalizedCoveragePath(path)
	if path == "" {
		return false
	}
	for _, testFile := range testFiles {
		testFile = normalizedCoveragePath(testFile)
		if testFile == "" {
			continue
		}
		if strings.HasPrefix(testFile, strings.TrimSuffix(path, "/")+"/") {
			return true
		}
		if coverageDir(testFile) == path {
			return true
		}
	}
	return false
}

func coverageDir(path string) string {
	path = normalizedCoveragePath(path)
	if path == "" || !strings.Contains(path, "/") {
		return path
	}
	lastSlash := strings.LastIndex(path, "/")
	name := path[lastSlash+1:]
	if strings.HasSuffix(path, ".go") || strings.Contains(name, ".") {
		return path[:lastSlash]
	}
	return path
}

func normalizedCoveragePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "`'\"")
	path = strings.TrimPrefix(path, "./")
	path = strings.ReplaceAll(path, "\\", "/")
	if idx := strings.Index(path, ":"); idx > 0 && strings.Contains(path[:idx], "/") {
		path = path[:idx]
	}
	path = strings.Trim(path, "/")
	if path == "." || path == "" || strings.HasPrefix(path, "../") {
		return ""
	}
	return path
}

func uniqueLimited(values []string, limit int) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func testingGapNone(locale string) string {
	if locale == "en-US" {
		return "No deterministic coverage gaps were found from the current source tree."
	}
	return "未发现可由当前源码确定的覆盖缺口。"
}

func testingGapKeyModule(path, locale string) string {
	if locale == "en-US" {
		return "Key module `" + path + "` has no same-directory or child `_test.go`; add focused tests or record manual verification when changing it."
	}
	return "关键模块 `" + path + "` 没有同目录或子目录 `_test.go`；修改时需要补充聚焦测试或记录人工验证风险。"
}

func testingGapPattern(path, name, locale string) string {
	if locale == "en-US" {
		return "Pattern `" + name + "` is evidenced under `" + path + "` without nearby `_test.go`; verify behavior explicitly when changing that path."
	}
	return "模式 `" + name + "` 的证据位于 `" + path + "`，附近没有 `_test.go`；修改该路径时需要显式验证行为。"
}
