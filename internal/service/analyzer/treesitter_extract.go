package analyzer

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// symbolInfo 表示从源码中提取出的代码符号。
type symbolInfo struct {
	Name string
	Kind string // func, method, type, class, interface, struct, enum, trait, protocol, module, object
	Line int
}

// importInfo 表示从源码中提取出的 import 或依赖路径。
type importInfo struct {
	Path string
}

// symbolNodeTypes 映射语言名到“AST 节点类型 -> 展示类型”。
// 这里只登记语法中带有 name 字段的节点，避免无法稳定取名的结构进入结果。
var symbolNodeTypes = map[string]map[string]string{
	"go": {
		"function_declaration": "func",
		"method_declaration":   "method",
	},
	"python": {
		"function_definition": "func",
		"class_definition":    "class",
	},
	"java": {
		"method_declaration":    "func",
		"class_declaration":     "class",
		"interface_declaration": "interface",
		"enum_declaration":      "enum",
	},
	"typescript": {
		"function_declaration":   "func",
		"class_declaration":      "class",
		"method_definition":      "method",
		"interface_declaration":  "interface",
		"type_alias_declaration": "type",
		"enum_declaration":       "enum",
	},
	"javascript": {
		"function_declaration": "func",
		"class_declaration":    "class",
		"method_definition":    "method",
	},
	"rust": {
		"function_item": "func",
		"struct_item":   "struct",
		"enum_item":     "enum",
		"trait_item":    "trait",
	},
	"c": {
		"function_definition": "func",
		"struct_specifier":    "struct",
	},
	"cpp": {
		"function_definition": "func",
		"class_specifier":     "class",
		"struct_specifier":    "struct",
	},
	"ruby": {
		"method":           "func",
		"singleton_method": "method",
		"class":            "class",
		"module":           "module",
	},
	"kotlin": {
		"function_declaration":  "func",
		"class_declaration":     "class",
		"object_declaration":    "object",
		"interface_declaration": "interface",
	},
	"swift": {
		"function_declaration": "func",
		"class_declaration":    "class",
		"struct_declaration":   "struct",
		"protocol_declaration": "protocol",
		"enum_declaration":     "enum",
	},
}

// typeSpecLanguages 记录类型名称位于内层 type_spec 子节点的语言。
// 例如 Go 的结构是 type_declaration > type_spec > name，因此要读取 type_spec 而不是外层声明。
var typeSpecLanguages = map[string]string{
	"go": "type_spec",
}

// extractSymbols 遍历 AST 并提取具名符号。
func extractSymbols(root *gotreesitter.Node, lang *gotreesitter.Language, src []byte, langName string) []symbolInfo {
	typeMap := symbolNodeTypes[langName]
	if len(typeMap) == 0 {
		// 未登记语言时，使用常见节点类型做通用提取。
		typeMap = genericSymbolTypes
	}

	// 检查该语言是否使用内嵌 type_spec 节点表达类型声明。
	typeSpecKind := typeSpecLanguages[langName]

	var symbols []symbolInfo
	walkNodes(root, lang, func(node *gotreesitter.Node) {
		nodeType := node.Type(lang)
		kind, ok := typeMap[nodeType]
		if !ok {
			return
		}

		nameNode := node.ChildByFieldName("name", lang)
		if nameNode == nil {
			return
		}
		name := nameNode.Text(src)
		if name == "" || name == "_" {
			return
		}

		// 普通声明节点直接使用自身起始行。
		line := int(node.StartPoint().Row) + 1

		symbols = append(symbols, symbolInfo{
			Name: name,
			Kind: kind,
			Line: line,
		})
	})

	// 对使用内嵌 type_spec 的语言，额外提取类型级声明。
	if typeSpecKind != "" {
		walkNodes(root, lang, func(node *gotreesitter.Node) {
			if node.Type(lang) != typeSpecKind {
				return
			}
			nameNode := node.ChildByFieldName("name", lang)
			if nameNode == nil {
				return
			}
			name := nameNode.Text(src)
			if name == "" || name == "_" {
				return
			}

			// 根据子节点判断更具体的类型种类，如 struct、interface 等。
			kind := "type"
			for i := 0; i < node.NamedChildCount(); i++ {
				child := node.NamedChild(i)
				if child == nil {
					continue
				}
				childType := child.Type(lang)
				switch childType {
				case "struct_type":
					kind = "struct"
				case "interface_type":
					kind = "interface"
				case "type_identifier":
					kind = "type"
				}
			}

			symbols = append(symbols, symbolInfo{
				Name: name,
				Kind: kind,
				Line: int(node.StartPoint().Row) + 1,
			})
		})
	}

	return symbols
}

// genericSymbolTypes 是未登记语言的通用符号节点 fallback。
var genericSymbolTypes = map[string]string{
	"function_declaration":  "func",
	"function_definition":   "func",
	"method_declaration":    "method",
	"method_definition":     "method",
	"class_declaration":     "class",
	"class_definition":      "class",
	"interface_declaration": "interface",
	"struct_declaration":    "struct",
	"struct_specifier":      "struct",
	"enum_declaration":      "enum",
	"enum_item":             "enum",
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
