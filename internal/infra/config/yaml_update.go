package config

import (
	"bytes"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// replaceConfigValues 通过 yaml.Node 更新配置值，保证注释继续附着在描述的节点上。
// 该方法用于测试和模板渲染失败后的后备保存路径。
func (r *Repository) replaceConfigValues(content string, cfg *Config) string {
	if cfg != nil {
		normalized := *cfg
		r.normalizeConfig(&normalized)
		cfg = &normalized
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return content
	}
	applyConfigNodeValues(&root, cfg)

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		_ = encoder.Close()
		return content
	}
	if err := encoder.Close(); err != nil {
		return content
	}
	return formatTopLevelModuleSpacing(buf.String())
}

func applyConfigNodeValues(root *yaml.Node, cfg *Config) {
	doc := configDocument(root)
	if doc == nil || cfg == nil {
		return
	}

	promoteLineComments(doc)

	setYAMLString(doc, []string{"profile", "name"}, cfg.Project.Name)
	setYAMLString(doc, []string{"profile", "mode"}, cfg.Project.Mode)
	setYAMLString(doc, []string{"profile", "language"}, cfg.Project.Language)
	setYAMLString(doc, []string{"profile", "locale"}, cfg.Project.Locale)
	setYAMLString(doc, []string{"profile", "git_remote"}, cfg.Project.GitRemote)
	setYAMLString(doc, []string{"profile", "root_path"}, cfg.Project.RootPath)
	if cfg.Project.InitializedAt != "" {
		setYAMLString(doc, []string{"profile", "initialized_at"}, cfg.Project.InitializedAt)
	}

	setYAMLWorkspaceConfig(doc, cfg.Workspace)

	setYAMLString(doc, []string{"learning", "current", "mode"}, string(cfg.Learning.Current.Mode))
	setYAMLString(doc, []string{"learning", "current", "scope"}, string(cfg.Learning.Current.Scope))
	setYAMLInt(doc, []string{"learning", "current", "parallelism"}, cfg.Learning.Current.Parallelism)
	setYAMLInt(doc, []string{"learning", "current", "max_units_per_call"}, cfg.Learning.Current.MaxUnitsPerCall)
	setYAMLBool(doc, []string{"learning", "current", "select_relevant_files"}, cfg.Learning.Current.SelectRelevantFiles)
	setYAMLInt(doc, []string{"learning", "current", "select_relevant_files_min_candidates"}, cfg.Learning.Current.SelectRelevantFilesMinCandidates)
	setYAMLInt(doc, []string{"learning", "current", "budget", "max_patterns_per_unit"}, cfg.Learning.Current.Budget.MaxPatternsPerUnit)
	setYAMLInt(doc, []string{"learning", "current", "budget", "max_patterns_per_run"}, cfg.Learning.Current.Budget.MaxPatternsPerRun)
	setYAMLInt(doc, []string{"learning", "current", "budget", "micro_change_new_patterns"}, cfg.Learning.Current.Budget.MicroChangeNewPatterns)
	setYAMLInt(doc, []string{"learning", "current", "budget", "minor_change_new_patterns"}, cfg.Learning.Current.Budget.MinorChangeNewPatterns)
	setYAMLFloat(doc, []string{"learning", "current", "budget", "min_confidence"}, cfg.Learning.Current.Budget.MinConfidence)
	setYAMLBool(doc, []string{"learning", "current", "budget", "update_existing_first"}, cfg.Learning.Current.Budget.UpdateExistingFirst)
	setYAMLBool(doc, []string{"learning", "current", "budget", "require_routeable_evidence"}, cfg.Learning.Current.Budget.RequireRouteableEvidence)
	setYAMLBool(doc, []string{"learning", "current", "structural", "enabled"}, cfg.Learning.Current.Structural.Enabled)
	setYAMLInt(doc, []string{"learning", "current", "structural", "max_symbols"}, cfg.Learning.Current.Structural.MaxSymbols)
	setYAMLInt(doc, []string{"learning", "current", "structural", "max_file_size"}, cfg.Learning.Current.Structural.MaxFileSize)

	setYAMLString(doc, []string{"agent", "engine"}, cfg.Agent.Engine)
	setYAMLStringMap(doc, []string{"agent", "commands"}, cfg.Agent.Commands)
	setYAMLInt(doc, []string{"agent", "timeout"}, cfg.Agent.Timeout)
	setYAMLBool(doc, []string{"agent", "allow_user_plugins"}, cfg.Agent.AllowUserPlugins)
	setYAMLInt(doc, []string{"agent", "parallelism"}, cfg.Agent.Parallelism)

	setYAMLInt(doc, []string{"learning", "history", "max_commits"}, cfg.Learning.History.MaxCommits)
	setYAMLInt(doc, []string{"learning", "history", "batch_size"}, cfg.Learning.History.BatchSize)

	setYAMLString(doc, []string{"autofix", "strategy"}, cfg.AutoFix.Strategy)
	setYAMLString(doc, []string{"autofix", "backup_path"}, cfg.AutoFix.BackupPath)

	setYAMLString(doc, []string{"skills", "target"}, cfg.Skills.Target)
	setYAMLString(doc, []string{"skills", "locale"}, cfg.Skills.Locale)
	setYAMLStringMap(doc, []string{"skills", "paths"}, cfg.Skills.Paths)

	setYAMLString(doc, []string{"logging", "level"}, cfg.Logging.Level)
	setYAMLString(doc, []string{"logging", "logs_path"}, cfg.Logging.LogsPath)
	setYAMLInt(doc, []string{"logging", "max_log_files"}, cfg.Logging.MaxLogFiles)

	setYAMLBool(doc, []string{"exclude", "gitignore"}, cfg.Exclude.GitIgnore)
	setYAMLStringList(doc, []string{"exclude", "paths"}, cfg.Exclude.Paths)

	promoteLineComments(doc)
}

func formatTopLevelModuleSpacing(content string) string {
	// banner 是配置模板中分隔顶层模块的固定横线。
	const banner = "########################################################################"
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}
	formatted := make([]string, 0, len(lines)+8)
	for i, line := range lines {
		if line == banner && i > 0 && nextLineIsModuleTitle(lines, i) && lastFormattedLine(formatted) != "" {
			formatted = append(formatted, "")
		}
		formatted = append(formatted, line)
	}
	return strings.Join(formatted, "\n")
}

func nextLineIsModuleTitle(lines []string, index int) bool {
	return index+1 < len(lines) && strings.HasPrefix(lines[index+1], "# ")
}

func lastFormattedLine(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

func configDocument(root *yaml.Node) *yaml.Node {
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			root.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
		}
		return root.Content[0]
	}
	if root.Kind == yaml.MappingNode {
		return root
	}
	return nil
}

func setYAMLString(root *yaml.Node, path []string, value string) {
	node := ensureYAMLPath(root, path)
	setScalarNode(node, "!!str", value, yaml.DoubleQuotedStyle)
}

func setYAMLBool(root *yaml.Node, path []string, value bool) {
	node := ensureYAMLPath(root, path)
	setScalarNode(node, "!!bool", strconv.FormatBool(value), 0)
}

func setYAMLInt(root *yaml.Node, path []string, value int) {
	node := ensureYAMLPath(root, path)
	setScalarNode(node, "!!int", strconv.Itoa(value), 0)
}

func setYAMLFloat(root *yaml.Node, path []string, value float64) {
	node := ensureYAMLPath(root, path)
	setScalarNode(node, "!!float", strconv.FormatFloat(value, 'f', -1, 64), 0)
}

func setYAMLStringMap(root *yaml.Node, path []string, values map[string]string) {
	node := ensureYAMLPath(root, path)
	comments := yamlCommentsFrom(node)
	moveCollectionLineCommentToKey(root, path, &comments)
	node.Kind = yaml.MappingNode
	node.Tag = "!!map"
	node.Value = ""
	node.Style = 0
	node.Content = nil
	applyYAMLComments(node, comments)
	for _, key := range sortedStringKeys(values) {
		node.Content = append(node.Content, stringKeyNode(key), quotedStringNode(values[key]))
	}
}

func setYAMLStringList(root *yaml.Node, path []string, values []string) {
	node := ensureYAMLPath(root, path)
	comments := yamlCommentsFrom(node)
	itemComments := sequenceScalarComments(node)
	moveCollectionLineCommentToKey(root, path, &comments)
	node.Kind = yaml.SequenceNode
	node.Tag = "!!seq"
	node.Value = ""
	node.Style = 0
	node.Content = nil
	if len(values) == 0 {
		node.Style = yaml.FlowStyle
	}
	applyYAMLComments(node, comments)
	for _, value := range values {
		item := quotedStringNode(value)
		applyYAMLComments(item, itemComments[value])
		node.Content = append(node.Content, item)
	}
}

func setYAMLWorkspaceConfig(root *yaml.Node, workspace WorkspaceConfig) {
	setYAMLProjects(root, []string{"workspace", "projects"}, workspace.Projects)
	removeYAMLMappingKeys(root, []string{"workspace"}, "shared", "contracts", "infra")
}

func setYAMLProjects(root *yaml.Node, path []string, projects []WorkspaceProjectConfig) {
	node := ensureYAMLPath(root, path)
	comments := yamlCommentsFrom(node)
	moveCollectionLineCommentToKey(root, path, &comments)
	node.Kind = yaml.SequenceNode
	node.Tag = "!!seq"
	node.Value = ""
	node.Style = 0
	node.Content = nil
	if len(projects) == 0 {
		node.Style = yaml.FlowStyle
	}
	applyYAMLComments(node, comments)
	for _, project := range projects {
		item := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		item.Content = append(item.Content,
			stringKeyNode("id"), quotedStringNode(project.ID),
			stringKeyNode("path"), quotedStringNode(project.Path),
			stringKeyNode("type"), quotedStringNode(project.Type),
			stringKeyNode("language"), quotedStringNode(project.Language),
		)
		node.Content = append(node.Content, item)
	}
}

func ensureYAMLPath(root *yaml.Node, path []string) *yaml.Node {
	current := root
	for i, key := range path {
		last := i == len(path)-1
		next := mappingValue(current, key)
		if next == nil {
			if last {
				next = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str"}
			} else {
				next = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			}
			appendMappingValue(current, key, next)
		}
		if !last && next.Kind != yaml.MappingNode {
			comments := yamlCommentsFrom(next)
			next.Kind = yaml.MappingNode
			next.Tag = "!!map"
			next.Value = ""
			next.Style = 0
			next.Content = nil
			applyYAMLComments(next, comments)
		}
		current = next
	}
	return current
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func mappingKey(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i]
		}
	}
	return nil
}

func mappingKeyForPath(root *yaml.Node, path []string) *yaml.Node {
	if len(path) == 0 {
		return nil
	}
	current := root
	for _, key := range path[:len(path)-1] {
		current = mappingValue(current, key)
		if current == nil {
			return nil
		}
	}
	return mappingKey(current, path[len(path)-1])
}

func removeYAMLMappingKeys(root *yaml.Node, path []string, keys ...string) {
	node := root
	for _, key := range path {
		node = mappingValue(node, key)
		if node == nil {
			return
		}
	}
	if node.Kind != yaml.MappingNode {
		return
	}
	remove := make(map[string]bool, len(keys))
	for _, key := range keys {
		remove[key] = true
	}
	content := node.Content[:0]
	for i := 0; i+1 < len(node.Content); i += 2 {
		if remove[node.Content[i].Value] {
			continue
		}
		content = append(content, node.Content[i], node.Content[i+1])
	}
	node.Content = content
}

func appendMappingValue(node *yaml.Node, key string, value *yaml.Node) {
	if node.Kind != yaml.MappingNode {
		node.Kind = yaml.MappingNode
		node.Tag = "!!map"
		node.Value = ""
		node.Style = 0
		node.Content = nil
	}
	node.Content = append(node.Content, stringKeyNode(key), value)
}

func setScalarNode(node *yaml.Node, tag, value string, style yaml.Style) {
	comments := yamlCommentsFrom(node)
	node.Kind = yaml.ScalarNode
	node.Tag = tag
	node.Value = value
	node.Style = style
	node.Content = nil
	applyYAMLComments(node, comments)
}

func stringKeyNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func quotedStringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value, Style: yaml.DoubleQuotedStyle}
}

type yamlComments struct {
	Head string
	Line string
	Foot string
}

func yamlCommentsFrom(node *yaml.Node) yamlComments {
	if node == nil {
		return yamlComments{}
	}
	return yamlComments{
		Head: node.HeadComment,
		Line: node.LineComment,
		Foot: node.FootComment,
	}
}

func applyYAMLComments(node *yaml.Node, comments yamlComments) {
	node.HeadComment = comments.Head
	node.LineComment = comments.Line
	node.FootComment = comments.Foot
}

func moveCollectionLineCommentToKey(root *yaml.Node, path []string, comments *yamlComments) {
	if comments == nil || comments.Line == "" {
		return
	}
	key := mappingKeyForPath(root, path)
	if key != nil {
		appendHeadComment(key, comments.Line)
		comments.Line = ""
	}
}

func promoteLineComments(node *yaml.Node) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			promoteLineComments(child)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			if key.LineComment != "" {
				appendHeadComment(key, key.LineComment)
				key.LineComment = ""
			}
			if value.LineComment != "" {
				appendHeadComment(key, value.LineComment)
				value.LineComment = ""
			}
			promoteLineComments(value)
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.LineComment != "" {
				appendHeadComment(item, item.LineComment)
				item.LineComment = ""
			}
			promoteLineComments(item)
		}
	}
}

func appendHeadComment(node *yaml.Node, comment string) {
	if node == nil {
		return
	}
	for _, line := range strings.Split(comment, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || headCommentContainsLine(node.HeadComment, line) {
			continue
		}
		if node.HeadComment == "" {
			node.HeadComment = line
			continue
		}
		node.HeadComment += "\n" + line
	}
}

func headCommentContainsLine(headComment, line string) bool {
	for _, existing := range strings.Split(headComment, "\n") {
		if strings.TrimSpace(existing) == line {
			return true
		}
	}
	return false
}

func sequenceScalarComments(node *yaml.Node) map[string]yamlComments {
	comments := map[string]yamlComments{}
	if node == nil || node.Kind != yaml.SequenceNode {
		return comments
	}
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Value == "" {
			continue
		}
		itemComments := yamlCommentsFrom(item)
		if itemComments == (yamlComments{}) {
			continue
		}
		comments[item.Value] = itemComments
	}
	return comments
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
