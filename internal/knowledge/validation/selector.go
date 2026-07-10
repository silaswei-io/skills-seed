package validation

import "strings"

type commandSelector struct {
	commands []Command
}

type commandChoice struct {
	Command    Command
	Match      MatchKind
	Coverage   float64
	Confidence float64
}

func (selector commandSelector) Choose(area Area) commandChoice {
	if choice := selector.chooseByEvidenceForArea(area, true); choice.Command.Command != "" {
		choice.Match = MatchScoped
		return choice
	}
	if choice := selector.chooseByEvidenceForArea(area, false); choice.Command.Command != "" && !commandLooksHeavy(choice.Command) {
		choice.Match = MatchBroad
		choice.Confidence = minFloat(choice.Confidence, 0.55)
		return choice
	}
	if choice := selector.chooseByText(area); choice.Command.Command != "" {
		choice.Match = MatchSemantic
		return choice
	}
	if choice := selector.chooseGenericForArea(area); choice.Command.Command != "" {
		choice.Match = MatchGeneric
		choice.Confidence = minFloat(choice.Confidence, 0.45)
		return choice
	}
	if choice := selector.chooseByEvidenceForArea(area, false); choice.Command.Command != "" {
		choice.Match = MatchBroad
		choice.Confidence = minFloat(choice.Confidence, 0.45)
		return choice
	}
	return commandChoice{}
}

func (selector commandSelector) chooseByEvidenceForArea(area Area, narrowOnly bool) commandChoice {
	best := commandChoice{}
	bestScore := 0
	evidenceRoots := validationPathRoots(area.Evidence)
	for _, command := range selector.commands {
		if !commandAppliesToArea(command, area) {
			continue
		}
		commandPaths := commandScopePaths(command)
		if len(commandPaths) == 0 {
			continue
		}
		if narrowOnly && !commandIsNarrow(command) {
			continue
		}
		if !commandSharesEvidenceRoot(commandPaths, evidenceRoots) {
			continue
		}
		score := commandEvidenceScore(commandPaths, area.Evidence)
		if score == 0 {
			continue
		}
		coverage := commandEvidenceCoverage(commandPaths, area.Evidence)
		if !commandCoverageAllowed(command, coverage, len(area.Evidence)) {
			continue
		}
		score += commandTypeScoreForArea(command, area)
		score += int(coverage * 10)
		if score > bestScore {
			bestScore = score
			best = commandChoice{
				Command:    command,
				Coverage:   coverage,
				Confidence: confidenceFromScore(score, coverage),
			}
		}
	}
	return best
}

func (selector commandSelector) chooseByText(area Area) commandChoice {
	best := commandChoice{}
	bestScore := 0
	evidenceRoots := validationPathRoots(area.Evidence)
	for _, command := range selector.commands {
		if !commandAppliesToArea(command, area) {
			continue
		}
		if commandLooksHeavy(command) {
			continue
		}
		if commandLooksBroadTest(command) {
			continue
		}
		declaredPaths := commandDeclaredScopePaths(command)
		if len(declaredPaths) > 0 && len(evidenceRoots) > 0 && !commandSharesEvidenceRoot(declaredPaths, evidenceRoots) {
			continue
		}
		coverage := commandEvidenceCoverage(declaredPaths, area.Evidence)
		if !commandCoverageAllowed(command, coverage, len(area.Evidence)) {
			continue
		}
		text := commandText(command)
		if !containsAny(text, area.Needles...) {
			continue
		}
		semanticScore := commandSemanticScore(text, area.Needles)
		if len(declaredPaths) == 0 && !commandHasSpecificSemanticMatch(command, semanticScore) {
			continue
		}
		score := commandTypeScoreForArea(command, area) + semanticScore
		if len(declaredPaths) == 0 {
			score += 2
		}
		if score > bestScore {
			bestScore = score
			best = commandChoice{
				Command:    command,
				Coverage:   coverage,
				Confidence: confidenceFromScore(score, coverage),
			}
		}
	}
	return best
}

func (selector commandSelector) chooseGenericForArea(area Area) commandChoice {
	best := commandChoice{}
	bestScore := 0
	for _, command := range selector.commands {
		if strings.TrimSpace(command.Command) == "" || len(commandDeclaredScopePaths(command)) > 0 {
			continue
		}
		if !commandAppliesToArea(command, area) {
			continue
		}
		score := commandTypeScoreForArea(command, area)
		if commandLooksBroadTest(command) {
			score -= 5
		}
		if commandKind(command) == "check" {
			score += 2
		}
		if commandLooksHeavy(command) {
			score--
		}
		if score > bestScore {
			bestScore = score
			best = commandChoice{
				Command:    command,
				Coverage:   0,
				Confidence: confidenceFromScore(score, 0),
			}
		}
	}
	return best
}

func commandEvidenceScore(commandPaths, evidence []string) int {
	score := 0
	for _, commandPath := range commandPaths {
		for _, evidencePath := range evidence {
			if validationPathMatches(commandPath, evidencePath) {
				score += commandPathScore(commandPath)
			}
		}
	}
	return score
}

func commandText(command Command) string {
	return strings.ToLower(command.Command + " " + command.When + " " + command.Source + " " + command.Type + " " + strings.Join(command.ScopePaths, " ") + " " + strings.Join(command.Evidence, " "))
}

func commandHasSpecificSemanticMatch(command Command, semanticScore int) bool {
	if semanticScore <= 0 {
		return false
	}
	text := strings.ToLower(command.Type + " " + command.Command + " " + command.When)
	if containsAny(text, "test", "verify", "测试", "验证") {
		return true
	}
	return semanticScore >= 2
}

func commandEvidenceCoverage(commandPaths, evidence []string) float64 {
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

func commandCoverageAllowed(command Command, coverage float64, evidenceCount int) bool {
	if evidenceCount == 0 || !commandIsNarrow(command) {
		return true
	}
	return coverage >= 0.5
}

func commandLooksHeavy(command Command) bool {
	text := strings.ToLower(command.Command + " " + command.Type)
	return containsAny(text, " build", "go build", "docker build", "serverless build", "打包", "构建")
}

func commandLooksBroadTest(command Command) bool {
	if commandKind(command) != "test" {
		return false
	}
	if len(commandDeclaredScopePaths(command)) > 0 {
		return false
	}
	text := strings.ToLower(command.Command)
	return strings.Contains(text, "./...") || strings.Contains(text, "-race")
}

func commandSemanticScore(text string, needles []string) int {
	score := 0
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			score++
		}
	}
	return score
}

func commandIsNarrow(command Command) bool {
	paths := commandScopePaths(command)
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

func commandPathScore(path string) int {
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

func commandTypeScore(command Command) int {
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

func commandTypeScoreForArea(command Command, area Area) int {
	score := commandTypeScore(command)
	if area.Kind == "" {
		return score
	}
	commandKind := commandKind(command)
	switch area.Kind {
	case AreaBusiness, AreaPersistence:
		if commandKind == "test" {
			score += 4
		}
		if commandKind == "check" {
			score += 1
		}
	case AreaRuntime:
		if commandKind == "test" || commandKind == "check" {
			score += 2
		}
	case AreaAPI:
		if commandKind == "test" || commandKind == "check" || commandKind == "generate" {
			score += 2
		}
	}
	return score
}

func commandAppliesToArea(command Command, area Area) bool {
	if area.Kind == "" {
		return true
	}
	commandKind := commandKind(command)
	if commandKind == "generate" && area.Kind != AreaAPI {
		return false
	}
	if commandKind == "generate" {
		text := commandText(command)
		return containsAny(text,
			"api", "contract", "schema", "proto", "swagger", "openapi", "route", "handler", "desc",
			"generate", "generated", "codegen", "gen", "契约", "接口", "生成", "路由",
		)
	}
	return true
}

func commandKind(command Command) string {
	text := strings.ToLower(command.Type + " " + command.Command + " " + command.When)
	switch {
	case containsAny(text, "test", "测试"):
		return "test"
	case containsAny(text, "check", "lint", "vet", "检查"):
		return "check"
	case containsAny(text, "generate", " gen", "codegen", "生成"):
		return "generate"
	case containsAny(text, "build", "编译", "构建"):
		return "build"
	default:
		return "other"
	}
}

func commandScopePaths(command Command) []string {
	paths := make([]string, 0, len(command.ScopePaths)+len(command.Evidence)+1)
	paths = append(paths, command.ScopePaths...)
	paths = append(paths, command.Evidence...)
	if command.Workdir != "" {
		paths = append(paths, command.Workdir)
	}
	paths = append(paths, CommandPaths(command.Command)...)
	return uniqueStrings(paths)
}

func commandDeclaredScopePaths(command Command) []string {
	paths := make([]string, 0, len(command.ScopePaths)+1)
	paths = append(paths, command.ScopePaths...)
	if command.Workdir != "" {
		paths = append(paths, command.Workdir)
	}
	paths = append(paths, CommandPaths(command.Command)...)
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

func commandSharesEvidenceRoot(commandPaths []string, evidenceRoots map[string]bool) bool {
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

func CommandPaths(command string) []string {
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

func validationPathMatches(scopePath, evidencePath string) bool {
	scopePath = normalizeValidationPath(scopePath)
	evidencePath = normalizeValidationPath(evidencePath)
	if scopePath == "" || evidencePath == "" {
		return false
	}
	return evidencePath == scopePath || strings.HasPrefix(evidencePath, scopePath+"/")
}

func normalizeValidationPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "`'\"")
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimSuffix(path, "/...")
	path = strings.TrimSuffix(path, "/")
	if idx := strings.Index(path, ":"); idx > 0 && strings.Contains(path[:idx], "/") {
		path = path[:idx]
	}
	return strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
}

func confidenceFromScore(score int, coverage float64) float64 {
	confidence := 0.35 + float64(score)*0.04 + coverage*0.25
	if confidence > 0.95 {
		return 0.95
	}
	if confidence < 0 {
		return 0
	}
	return confidence
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
