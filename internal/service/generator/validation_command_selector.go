package generator

import "strings"

type validationCommandSelector struct {
	commands []validationCommand
}

func (selector validationCommandSelector) Choose(area validationArea) validationCommandChoice {
	if command := selector.chooseByEvidence(area.Evidence, true); command.Command != "" {
		return validationCommandChoice{Command: command, Match: validationCommandMatchScoped}
	}
	if command := selector.chooseByEvidence(area.Evidence, false); command.Command != "" && !validationCommandLooksHeavy(command) {
		return validationCommandChoice{Command: command, Match: validationCommandMatchBroad}
	}
	if command := selector.chooseByText(area); command.Command != "" {
		return validationCommandChoice{Command: command, Match: validationCommandMatchSemantic}
	}
	if command := selector.chooseGeneric(); command.Command != "" {
		return validationCommandChoice{Command: command, Match: validationCommandMatchGeneric}
	}
	if command := selector.chooseByEvidence(area.Evidence, false); command.Command != "" {
		return validationCommandChoice{Command: command, Match: validationCommandMatchBroad}
	}
	return validationCommandChoice{}
}

func (selector validationCommandSelector) chooseByEvidence(evidence []string, narrowOnly bool) validationCommand {
	best := validationCommand{}
	bestScore := 0
	evidenceRoots := validationPathRoots(evidence)
	for _, command := range selector.commands {
		commandPaths := validationCommandScopePaths(command)
		if len(commandPaths) == 0 {
			continue
		}
		if narrowOnly && !validationCommandIsNarrow(command) {
			continue
		}
		if !validationCommandSharesEvidenceRoot(commandPaths, evidenceRoots) {
			continue
		}
		score := validationCommandEvidenceScore(commandPaths, evidence)
		if score == 0 {
			continue
		}
		coverage := validationCommandEvidenceCoverage(commandPaths, evidence)
		if !validationCommandCoverageAllowed(command, coverage, len(evidence)) {
			continue
		}
		score += validationCommandTypeScore(command)
		score += int(coverage * 10)
		if score > bestScore {
			bestScore = score
			best = command
		}
	}
	return best
}

func (selector validationCommandSelector) chooseByText(area validationArea) validationCommand {
	best := validationCommand{}
	bestScore := 0
	evidenceRoots := validationPathRoots(area.Evidence)
	for _, command := range selector.commands {
		if validationCommandLooksHeavy(command) {
			continue
		}
		declaredPaths := validationCommandDeclaredScopePaths(command)
		if len(declaredPaths) > 0 && len(evidenceRoots) > 0 && !validationCommandSharesEvidenceRoot(declaredPaths, evidenceRoots) {
			continue
		}
		coverage := validationCommandEvidenceCoverage(declaredPaths, area.Evidence)
		if !validationCommandCoverageAllowed(command, coverage, len(area.Evidence)) {
			continue
		}
		text := validationCommandText(command)
		if !containsAny(text, area.Needles...) {
			continue
		}
		semanticScore := validationCommandSemanticScore(text, area.Needles)
		if len(declaredPaths) == 0 && !validationCommandHasSpecificSemanticMatch(command, semanticScore) {
			continue
		}
		score := validationCommandTypeScore(command) + semanticScore
		if len(declaredPaths) == 0 {
			score += 2
		}
		if score > bestScore {
			bestScore = score
			best = command
		}
	}
	return best
}

func (selector validationCommandSelector) chooseGeneric() validationCommand {
	best := validationCommand{}
	bestScore := 0
	for _, command := range selector.commands {
		if strings.TrimSpace(command.Command) == "" || len(validationCommandDeclaredScopePaths(command)) > 0 {
			continue
		}
		score := validationCommandTypeScore(command)
		if validationCommandLooksHeavy(command) {
			score--
		}
		if score > bestScore {
			bestScore = score
			best = command
		}
	}
	return best
}

func validationCommandEvidenceScore(commandPaths, evidence []string) int {
	score := 0
	for _, commandPath := range commandPaths {
		for _, evidencePath := range evidence {
			if validationPathMatches(commandPath, evidencePath) {
				score += validationCommandPathScore(commandPath)
			}
		}
	}
	return score
}

func validationCommandText(command validationCommand) string {
	return strings.ToLower(command.Command + " " + command.When + " " + command.Source + " " + command.Type + " " + strings.Join(command.ScopePaths, " ") + " " + strings.Join(command.Evidence, " "))
}

func validationCommandHasSpecificSemanticMatch(command validationCommand, semanticScore int) bool {
	if semanticScore <= 0 {
		return false
	}
	text := strings.ToLower(command.Type + " " + command.Command + " " + command.When)
	if containsAny(text, "test", "verify", "测试", "验证") {
		return true
	}
	return semanticScore >= 2
}

func validationCommandEvidenceCoverage(commandPaths, evidence []string) float64 {
	if len(evidence) == 0 {
		return 1
	}
	if len(commandPaths) == 0 {
		return 0
	}
	matchedEvidence := map[string]bool{}
	for _, commandPath := range commandPaths {
		for _, evidencePath := range evidence {
			if validationPathMatches(commandPath, evidencePath) {
				matchedEvidence[evidencePath] = true
			}
		}
	}
	return float64(len(matchedEvidence)) / float64(len(evidence))
}

func validationCommandCoverageAllowed(command validationCommand, coverage float64, evidenceCount int) bool {
	if evidenceCount == 0 || !validationCommandIsNarrow(command) {
		return true
	}
	return coverage >= 0.5
}

func validationCommandLooksHeavy(command validationCommand) bool {
	text := strings.ToLower(command.Command + " " + command.Type)
	return containsAny(text, " build", "go build", "docker build", "serverless build", "打包", "构建")
}

func validationCommandSemanticScore(text string, needles []string) int {
	score := 0
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			score++
		}
	}
	return score
}

func validationCommandIsNarrow(command validationCommand) bool {
	paths := validationCommandScopePaths(command)
	if len(paths) == 0 {
		return false
	}
	for _, path := range paths {
		path = normalizeValidationPath(path)
		if path == "" || path == "." || path == "..." {
			return false
		}
		if strings.Count(path, "/") < 1 {
			return false
		}
	}
	return true
}

func validationCommandPathScore(path string) int {
	path = strings.Trim(strings.TrimSpace(path), "./")
	if path == "" {
		return 0
	}
	score := len(strings.Split(path, "/")) + 1
	if path == "..." || path == "." {
		return 1
	}
	if strings.Contains(path, "plugins/") || strings.Contains(path, "internal/") {
		score += 2
	}
	return score
}

func validationCommandTypeScore(command validationCommand) int {
	text := strings.ToLower(command.Type + " " + command.Command + " " + command.When)
	switch {
	case containsAny(text, "test", "测试"):
		return 5
	case containsAny(text, "check", "lint", "vet", "检查"):
		return 4
	case containsAny(text, "generate", "gen", "生成"):
		return 3
	case containsAny(text, "build", "编译", "构建"):
		return 1
	default:
		return 0
	}
}

func validationCommandScopePaths(command validationCommand) []string {
	paths := make([]string, 0, len(command.ScopePaths)+len(command.Evidence)+1)
	paths = append(paths, command.ScopePaths...)
	paths = append(paths, command.Evidence...)
	if command.Workdir != "" {
		paths = append(paths, command.Workdir)
	}
	paths = append(paths, validationCommandPaths(command.Command)...)
	return uniqueStrings(paths)
}

func validationCommandDeclaredScopePaths(command validationCommand) []string {
	paths := make([]string, 0, len(command.ScopePaths)+1)
	paths = append(paths, command.ScopePaths...)
	if command.Workdir != "" {
		paths = append(paths, command.Workdir)
	}
	paths = append(paths, validationCommandPaths(command.Command)...)
	return uniqueStrings(paths)
}

func validationPathRoots(paths []string) map[string]bool {
	roots := make(map[string]bool, len(paths))
	for _, path := range paths {
		root := validationPathRoot(path)
		if root != "" {
			roots[root] = true
		}
	}
	return roots
}

func validationCommandSharesEvidenceRoot(commandPaths []string, evidenceRoots map[string]bool) bool {
	if len(evidenceRoots) == 0 {
		return true
	}
	for _, commandPath := range commandPaths {
		if evidenceRoots[validationPathRoot(commandPath)] {
			return true
		}
	}
	return false
}

func validationPathRoot(path string) string {
	path = normalizeValidationPath(path)
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	if len(parts) >= 2 && parts[0] == "plugins" {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

func validationCommandPaths(command string) []string {
	fields := strings.Fields(command)
	paths := make([]string, 0)
	for _, field := range fields {
		field = strings.Trim(field, "`'\"")
		if !strings.HasPrefix(field, "./") {
			continue
		}
		field = strings.TrimSuffix(field, "/...")
		field = strings.TrimSuffix(field, "/")
		if field == "." {
			continue
		}
		paths = append(paths, strings.TrimPrefix(field, "./"))
	}
	return uniqueStrings(paths)
}

func validationPathMatches(commandPath, evidencePath string) bool {
	commandPath = normalizeValidationPath(commandPath)
	evidencePath = normalizeValidationPath(evidencePath)
	if commandPath == "" || evidencePath == "" {
		return false
	}
	return evidencePath == commandPath ||
		strings.HasPrefix(evidencePath, commandPath+"/") ||
		strings.HasPrefix(commandPath, evidencePath+"/")
}

func normalizeValidationPath(path string) string {
	path = strings.Trim(strings.TrimSpace(path), "`")
	path = strings.TrimPrefix(path, "./")
	if idx := strings.Index(path, ":"); idx > 0 {
		suffix := path[idx+1:]
		if suffix == "" || isNumericPathSuffix(suffix) {
			path = path[:idx]
		}
	}
	path = strings.TrimSuffix(path, "/...")
	path = strings.Trim(path, "/")
	return path
}

func isNumericPathSuffix(value string) bool {
	for _, part := range strings.Split(value, ":") {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
