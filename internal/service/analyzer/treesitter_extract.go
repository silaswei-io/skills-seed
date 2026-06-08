package analyzer

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// symbolInfo represents an extracted code symbol.
type symbolInfo struct {
	Name string
	Kind string // func, method, type, class, interface, struct, enum, trait, protocol, module, object
	Line int
}

// importInfo represents an extracted import/dependency.
type importInfo struct {
	Path string
}

// symbolNodeTypes maps (language name) → (AST node type → display kind).
// Only node types that have a "name" field in the grammar are listed.
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

// typeSpecParentKinds lists languages where the type name lives inside a
// type_spec child (e.g. Go: type_declaration > type_spec > name).
// For these languages we look at type_spec nodes instead of the outer declaration.
var typeSpecLanguages = map[string]string{
	"go": "type_spec",
}

// extractSymbols walks the AST and extracts named symbols.
func extractSymbols(root *gotreesitter.Node, lang *gotreesitter.Language, src []byte, langName string) []symbolInfo {
	typeMap := symbolNodeTypes[langName]
	if len(typeMap) == 0 {
		// Unknown language — try generic extraction using common node types.
		typeMap = genericSymbolTypes
	}

	// Check if this language uses nested type_spec nodes.
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

		// For Go type_spec, the parent is the actual declaration — use parent's line.
		line := int(node.StartPoint().Row) + 1

		symbols = append(symbols, symbolInfo{
			Name: name,
			Kind: kind,
			Line: line,
		})
	})

	// For languages with nested type_spec, also extract type-level declarations.
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

			// Determine the concrete type kind (struct, interface, etc.)
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

// genericSymbolTypes is a fallback for unregistered languages.
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

// extractImports extracts import paths from the AST.
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
			// import X, Y
			for i := 0; i < node.NamedChildCount(); i++ {
				child := node.NamedChild(i)
				if child != nil {
					imports = append(imports, importInfo{Path: child.Text(src)})
				}
			}
		case "import_from_statement":
			// from X import Y
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
		// Get the argument (the imported path)
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

// isEntryPoint returns true if the symbol looks like an entry point (main).
func isEntryPoint(name string) bool {
	return name == "main"
}

// walkNodes recursively visits all named nodes in pre-order.
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
