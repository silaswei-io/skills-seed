package config

import (
	"bytes"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// replaceConfigValues updates config YAML through yaml.Node so comments stay attached
// to the nodes they describe. It exists for tests and fallback rendering paths.
func (r *Repository) replaceConfigValues(content string, cfg *Config) string {
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
	return buf.String()
}

func applyConfigNodeValues(root *yaml.Node, cfg *Config) {
	doc := configDocument(root)
	if doc == nil || cfg == nil {
		return
	}

	promoteLineComments(doc)

	setYAMLString(doc, []string{"project", "name"}, cfg.Project.Name)
	setYAMLString(doc, []string{"project", "mode"}, cfg.Project.Mode)
	setYAMLString(doc, []string{"project", "language"}, cfg.Project.Language)
	setYAMLString(doc, []string{"project", "locale"}, cfg.Project.Locale)
	setYAMLString(doc, []string{"project", "git_remote"}, cfg.Project.GitRemote)
	setYAMLString(doc, []string{"project", "root_path"}, cfg.Project.RootPath)
	if cfg.Project.InitializedAt != "" {
		setYAMLString(doc, []string{"project", "initialized_at"}, cfg.Project.InitializedAt)
	}

	setYAMLWorkspaceConfig(doc, cfg.Workspace)

	setYAMLBool(doc, []string{"analysis", "codegraph", "enabled"}, cfg.Analysis.CodeGraph.Enabled)
	setYAMLBool(doc, []string{"analysis", "codegraph", "required"}, cfg.Analysis.CodeGraph.Required)
	setYAMLString(doc, []string{"analysis", "codegraph", "command"}, cfg.Analysis.CodeGraph.Command)
	setYAMLBool(doc, []string{"analysis", "codegraph", "auto_init"}, cfg.Analysis.CodeGraph.AutoInit)
	setYAMLBool(doc, []string{"analysis", "codegraph", "auto_sync"}, cfg.Analysis.CodeGraph.AutoSync)
	setYAMLInt(doc, []string{"analysis", "codegraph", "max_nodes"}, cfg.Analysis.CodeGraph.MaxNodes)
	setYAMLInt(doc, []string{"analysis", "codegraph", "max_code"}, cfg.Analysis.CodeGraph.MaxCode)

	setYAMLString(doc, []string{"agent", "engine"}, cfg.Agent.Engine)
	setYAMLStringMap(doc, []string{"agent", "commands"}, cfg.Agent.Commands)
	setYAMLInt(doc, []string{"agent", "timeout"}, cfg.Agent.Timeout)
	setYAMLBool(doc, []string{"agent", "allow_user_plugins"}, cfg.Agent.AllowUserPlugins)
	setYAMLInt(doc, []string{"agent", "parallelism"}, cfg.Agent.Parallelism)

	setYAMLInt(doc, []string{"learning", "max_commits"}, cfg.Learning.MaxCommits)
	setYAMLInt(doc, []string{"learning", "batch_size"}, cfg.Learning.BatchSize)

	setYAMLString(doc, []string{"autofix", "strategy"}, cfg.AutoFix.Strategy)
	setYAMLString(doc, []string{"autofix", "backup_path"}, cfg.AutoFix.BackupPath)

	setYAMLString(doc, []string{"skills", "target"}, cfg.Skills.Target)
	setYAMLStringMap(doc, []string{"skills", "paths"}, cfg.Skills.Paths)

	setYAMLString(doc, []string{"logging", "level"}, cfg.Logging.Level)
	setYAMLString(doc, []string{"logging", "logs_path"}, cfg.Logging.LogsPath)
	setYAMLInt(doc, []string{"logging", "max_log_files"}, cfg.Logging.MaxLogFiles)

	setYAMLStringList(doc, []string{"exclude"}, cfg.Exclude)

	promoteLineComments(doc)
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
	setYAMLWorkspacePaths(root, []string{"workspace", "shared"}, workspace.Shared)
	setYAMLWorkspacePaths(root, []string{"workspace", "contracts"}, workspace.Contracts)
	setYAMLWorkspacePaths(root, []string{"workspace", "infra"}, workspace.Infra)
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

func setYAMLWorkspacePaths(root *yaml.Node, path []string, paths []WorkspacePathConfig) {
	node := ensureYAMLPath(root, path)
	comments := yamlCommentsFrom(node)
	moveCollectionLineCommentToKey(root, path, &comments)
	node.Kind = yaml.SequenceNode
	node.Tag = "!!seq"
	node.Value = ""
	node.Style = 0
	node.Content = nil
	if len(paths) == 0 {
		node.Style = yaml.FlowStyle
	}
	applyYAMLComments(node, comments)
	for _, workspacePath := range paths {
		item := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		item.Content = append(item.Content, stringKeyNode("path"), quotedStringNode(workspacePath.Path))
		if workspacePath.Description != "" {
			item.Content = append(item.Content, stringKeyNode("description"), quotedStringNode(workspacePath.Description))
		}
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
