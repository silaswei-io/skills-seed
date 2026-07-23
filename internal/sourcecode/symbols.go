// Package sourcecode 提供可切换 provider 的只读源码事实提取与校验。
package sourcecode

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// Symbol 是 tree-sitter tags query 返回的统一符号定义。
type Symbol struct {
	Name      string
	Kind      string
	Line      int
	Signature string
}

// parseSymbols 使用 grammar 自带或自动推导的 tags query 提取符号。
func parseSymbols(filename string, src []byte) ([]Symbol, error) {
	entry := grammars.DetectLanguage(filepath.Base(filename))
	if entry == nil {
		return nil, fmt.Errorf("unsupported file type: %s", filename)
	}
	query := grammars.ResolveTagsQuery(*entry)
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("tags query unavailable: %s", filename)
	}
	lang := entry.Language()
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, err
	}
	defer tree.Release()
	return extractSymbols(tree.RootNode(), lang, query, src, true), nil
}

// ExtractSymbols 从已有语法树提取符号，供结构扫描与事实校验共同使用。
func ExtractSymbols(root *gotreesitter.Node, lang *gotreesitter.Language, filename string, src []byte) []Symbol {
	entry := grammars.DetectLanguage(filepath.Base(filename))
	if root == nil || lang == nil || entry == nil {
		return nil
	}
	query := grammars.ResolveTagsQuery(*entry)
	if strings.TrimSpace(query) == "" {
		return nil
	}
	return extractSymbols(root, lang, query, src, false)
}

// FindSymbol 按名称、统一类型和期望行号匹配最接近的真实符号。
func FindSymbol(symbols []Symbol, name, kind string, line int) (Symbol, bool) {
	wanted := simpleSymbolName(name)
	var match Symbol
	found := false
	for _, symbol := range symbols {
		if symbol.Name != wanted || !compatibleKind(kind, symbol.Kind) {
			continue
		}
		if !found || line > 0 && lineDistance(symbol.Line, line) < lineDistance(match.Line, line) {
			match = symbol
			found = true
		}
	}
	return match, found
}

func simpleSymbolName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "() \t\r\n") {
		return ""
	}
	if index := strings.LastIndex(value, "."); index >= 0 {
		value = value[index+1:]
	}
	if index := strings.LastIndex(value, "::"); index >= 0 {
		value = value[index+2:]
	}
	return strings.TrimSpace(value)
}

func compatibleKind(requested, actual string) bool {
	requested = canonicalKind(requested)
	actual = canonicalKind(actual)
	if requested == "" || requested == actual {
		return true
	}
	if requested == "function" || requested == "method" {
		return actual == "function" || actual == "method"
	}
	if requested != "type" {
		return false
	}
	switch actual {
	case "type", "class", "interface", "struct", "enum", "trait", "protocol", "module", "object":
		return true
	default:
		return false
	}
}

func extractSymbols(root *gotreesitter.Node, lang *gotreesitter.Language, query string, src []byte, includeSignature bool) []Symbol {
	compiled, err := gotreesitter.NewQuery(query, lang)
	if err != nil {
		return nil
	}
	cursor := compiled.Exec(root, lang, src)
	var symbols []Symbol
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		var definition *gotreesitter.Node
		var name *gotreesitter.Node
		var kind string
		for _, capture := range match.Captures {
			switch {
			case capture.Name == "name":
				name = capture.Node
			case strings.HasPrefix(capture.Name, "definition."):
				definition = capture.Node
				kind = capture.Name
			}
		}
		if definition == nil || name == nil || int(definition.EndByte()) > len(src) {
			continue
		}
		symbol := Symbol{
			Name: strings.TrimSpace(name.Text(src)),
			Kind: canonicalKind(kind),
			Line: int(name.StartPoint().Row) + 1,
		}
		if includeSignature {
			symbol.Signature = definitionSignature(definition, lang, src)
		}
		symbols = append(symbols, symbol)
	}
	return symbols
}

// definitionSignature 通过语法定义的 body 字段截取声明部分。
// 没有 body 的定义保留完整文本，避免引入语言或节点类型分支。
func definitionSignature(definition *gotreesitter.Node, lang *gotreesitter.Language, src []byte) string {
	end := definition.EndByte()
	if body := definition.ChildByFieldName("body", lang); body != nil {
		end = body.StartByte()
	}
	if definition.StartByte() > end || int(end) > len(src) {
		return ""
	}
	return strings.TrimSpace(string(src[definition.StartByte():end]))
}

func canonicalKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	kind = strings.TrimPrefix(kind, "definition.")
	switch kind {
	case "func":
		return "function"
	default:
		return kind
	}
}

func lineDistance(left, right int) int {
	if left < right {
		return right - left
	}
	return left - right
}
