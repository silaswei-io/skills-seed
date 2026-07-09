package analyzer

import (
	"fmt"
	"sort"
	"strings"
)

type structuralProviderName string

const (
	structuralProviderTreeSitter structuralProviderName = "treesitter"
	structuralProviderCodeGraph  structuralProviderName = "codegraph"
)

// structuralContextData 是结构化上下文的统一中间模型。
type structuralContextData struct {
	Source      structuralProviderName
	FilesFound  int
	FilesParsed int
	LangCounts  map[string]int
	Symbols     []structuralSymbol
	Imports     []structuralImport
	EntryPoints []structuralEntryPoint
	Sections    []structuralSection
}

type structuralSymbol struct {
	Path string
	Lang string
	Name string
	Kind string
	Line int
}

type structuralImport struct {
	Path       string
	Lang       string
	ImportPath string
}

type structuralEntryPoint struct {
	Path string
	Kind string
	Name string
	Line int
}

type structuralSection struct {
	Title string
	Body  string
}

type structuralRenderer struct{}

func (r structuralRenderer) Render(data *structuralContextData, maxSymbols int) string {
	if data == nil {
		return ""
	}
	if maxSymbols <= 0 {
		// maxSymbols 默认最多输出 30 个符号，控制提示词上下文体积。
		maxSymbols = 30
	}

	var b strings.Builder
	b.WriteString("## Structural Context\n\n")
	if data.Source != "" {
		b.WriteString(fmt.Sprintf("Source: %s\n\n", data.Source))
	}

	b.WriteString("### Status\n\n")
	b.WriteString(fmt.Sprintf("Files scanned: %d | Files parsed: %d | Languages: %s\n\n",
		data.FilesFound, data.FilesParsed, formatLangCounts(data.LangCounts)))

	r.writeSymbols(&b, data.Symbols, maxSymbols)
	r.writeImports(&b, data.Imports, maxSymbols)
	r.writeEntryPoints(&b, data.EntryPoints)
	r.writeSections(&b, data.Sections)

	return b.String()
}

func (r structuralRenderer) writeSymbols(b *strings.Builder, symbols []structuralSymbol, maxSymbols int) {
	b.WriteString("### Symbols\n\n")
	if len(symbols) == 0 {
		return
	}
	symbols = append([]structuralSymbol(nil), symbols...)
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].Path == symbols[j].Path {
			return symbols[i].Line < symbols[j].Line
		}
		return symbols[i].Path < symbols[j].Path
	})
	printed := 0
	currentPath := ""
	for _, sym := range symbols {
		if printed >= maxSymbols {
			b.WriteString("... (truncated)\n\n")
			return
		}
		if sym.Path != currentPath {
			if currentPath != "" {
				b.WriteByte('\n')
			}
			b.WriteString(fmt.Sprintf("#### %s: %s\n", sym.Lang, sym.Path))
			currentPath = sym.Path
		}
		b.WriteString(fmt.Sprintf("- %s %s (line %d)\n", sym.Kind, sym.Name, sym.Line))
		printed++
	}
	b.WriteByte('\n')
}

func (r structuralRenderer) writeImports(b *strings.Builder, imports []structuralImport, maxImports int) {
	b.WriteString("### Imports\n\n")
	if len(imports) == 0 {
		return
	}
	imports = append([]structuralImport(nil), imports...)
	sort.Slice(imports, func(i, j int) bool {
		if imports[i].Path == imports[j].Path {
			return imports[i].ImportPath < imports[j].ImportPath
		}
		return imports[i].Path < imports[j].Path
	})
	printed := 0
	currentPath := ""
	for _, imp := range imports {
		if printed >= maxImports {
			b.WriteString("... (truncated)\n\n")
			return
		}
		if imp.Path != currentPath {
			if currentPath != "" {
				b.WriteByte('\n')
			}
			b.WriteString(fmt.Sprintf("#### %s: %s\n", imp.Lang, imp.Path))
			currentPath = imp.Path
		}
		b.WriteString(fmt.Sprintf("- %s\n", imp.ImportPath))
		printed++
	}
	b.WriteByte('\n')
}

func (r structuralRenderer) writeEntryPoints(b *strings.Builder, entryPoints []structuralEntryPoint) {
	if len(entryPoints) == 0 {
		return
	}
	entryPoints = append([]structuralEntryPoint(nil), entryPoints...)
	sort.Slice(entryPoints, func(i, j int) bool {
		if entryPoints[i].Path == entryPoints[j].Path {
			return entryPoints[i].Line < entryPoints[j].Line
		}
		return entryPoints[i].Path < entryPoints[j].Path
	})
	b.WriteString("### Entry Points\n\n")
	for _, ep := range entryPoints {
		b.WriteString(fmt.Sprintf("- %s %s (%s:%d)\n", ep.Kind, ep.Name, ep.Path, ep.Line))
	}
	b.WriteByte('\n')
}

func (r structuralRenderer) writeSections(b *strings.Builder, sections []structuralSection) {
	for _, section := range sections {
		body := strings.TrimSpace(section.Body)
		if body == "" {
			continue
		}
		title := strings.TrimSpace(section.Title)
		if title == "" {
			title = "Provider Context"
		}
		b.WriteString("### ")
		b.WriteString(title)
		b.WriteString("\n\n")
		b.WriteString(body)
		b.WriteString("\n\n")
	}
}

func formatLangCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}
	type kv struct {
		lang  string
		count int
	}
	var kvs []kv
	for k, v := range counts {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool {
		if kvs[i].count == kvs[j].count {
			return kvs[i].lang < kvs[j].lang
		}
		return kvs[i].count > kvs[j].count
	})
	parts := make([]string, len(kvs))
	for i, kv := range kvs {
		parts[i] = fmt.Sprintf("%s(%d)", kv.lang, kv.count)
	}
	return strings.Join(parts, ", ")
}
