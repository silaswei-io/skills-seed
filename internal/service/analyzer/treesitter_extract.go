package analyzer

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// importInfo 表示从源码中提取出的 import 或依赖路径。
type importInfo struct {
	Path string
}

// extractImports 从 AST 中提取 import 路径。
func extractImports(root *gotreesitter.Node, lang *gotreesitter.Language, src []byte, langName string) []importInfo {
	switch langName {
	case "go":
		return extractGoImports(root, lang, src)
	case "python":
		return extractPythonImports(root, lang, src)
	case "java":
		return extractJavaImports(root, lang, src)
	case "typescript", "javascript":
		return extractJSImports(root, lang, src)
	case "rust":
		return extractRustImports(root, lang, src)
	default:
		return nil
	}
}

func extractGoImports(root *gotreesitter.Node, lang *gotreesitter.Language, src []byte) []importInfo {
	var imports []importInfo
	walkNodes(root, lang, func(node *gotreesitter.Node) {
		if node.Type(lang) != "import_spec" {
			return
		}
		pathNode := node.ChildByFieldName("path", lang)
		if pathNode == nil {
			return
		}
		path := pathNode.Text(src)
		path = strings.Trim(path, `"`)
		imports = append(imports, importInfo{Path: path})
	})
	return imports
}

func extractPythonImports(root *gotreesitter.Node, lang *gotreesitter.Language, src []byte) []importInfo {
	var imports []importInfo
	walkNodes(root, lang, func(node *gotreesitter.Node) {
		switch node.Type(lang) {
		case "import_statement":
			// Python: import X, Y
			for i := 0; i < node.NamedChildCount(); i++ {
				child := node.NamedChild(i)
				if child != nil {
					imports = append(imports, importInfo{Path: child.Text(src)})
				}
			}
		case "import_from_statement":
			// Python: from X import Y
			modNode := node.ChildByFieldName("module_name", lang)
			if modNode != nil {
				imports = append(imports, importInfo{Path: modNode.Text(src)})
			}
		}
	})
	return imports
}

func extractJavaImports(root *gotreesitter.Node, lang *gotreesitter.Language, src []byte) []importInfo {
	var imports []importInfo
	walkNodes(root, lang, func(node *gotreesitter.Node) {
		if node.Type(lang) != "import_declaration" {
			return
		}
		text := strings.TrimSpace(node.Text(src))
		text = strings.TrimPrefix(text, "import ")
		text = strings.TrimSuffix(text, ";")
		text = strings.TrimSpace(text)
		if text != "" {
			imports = append(imports, importInfo{Path: text})
		}
	})
	return imports
}

func extractJSImports(root *gotreesitter.Node, lang *gotreesitter.Language, src []byte) []importInfo {
	var imports []importInfo
	walkNodes(root, lang, func(node *gotreesitter.Node) {
		switch node.Type(lang) {
		case "import_statement":
			sourceNode := node.ChildByFieldName("source", lang)
			if sourceNode != nil {
				path := sourceNode.Text(src)
				path = strings.Trim(path, `"'`)
				imports = append(imports, importInfo{Path: path})
			}
		}
	})
	return imports
}

func extractRustImports(root *gotreesitter.Node, lang *gotreesitter.Language, src []byte) []importInfo {
	var imports []importInfo
	walkNodes(root, lang, func(node *gotreesitter.Node) {
		if node.Type(lang) != "use_declaration" {
			return
		}
		// 读取 use 声明的参数作为导入路径。
		for i := 0; i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child != nil {
				text := child.Text(src)
				imports = append(imports, importInfo{Path: text})
			}
		}
	})
	return imports
}

// isEntryPoint 判断符号是否看起来像入口函数。
func isEntryPoint(name string) bool {
	return name == "main"
}

// walkNodes 以前序递归访问所有节点。
func walkNodes(node *gotreesitter.Node, lang *gotreesitter.Language, visit func(*gotreesitter.Node)) {
	if node == nil {
		return
	}
	visit(node)
	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		walkNodes(child, lang, visit)
	}
}
