package sourcecode

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type symbolFile struct {
	path    string
	symbols []Symbol
}

// Verifier 使用已解析符号目录校验领域对象中的源码事实。
type Verifier struct {
	files map[string]symbolFile
}

// NewVerifier 创建不负责文件读取和 parser 选择的源码事实校验器。
func NewVerifier(catalog Catalog) *Verifier {
	files := make(map[string]symbolFile, len(catalog))
	for path, symbols := range catalog {
		path = normalizeReferencePath(path)
		if path != "" {
			files[path] = symbolFile{path: path, symbols: symbols}
		}
	}
	return &Verifier{files: files}
}

// VerifyUtilities 校验工具函数的符号身份，并以源码事实替换 AI 字段。
func (v *Verifier) VerifyUtilities(values []domain.UtilityFunction) []domain.UtilityFunction {
	verified := make([]domain.UtilityFunction, 0, len(values))
	for _, value := range values {
		file, line, ok := v.load(value.File)
		if !ok {
			continue
		}
		symbol, ok := findCallable(file.symbols, line, value.Name)
		if !ok {
			continue
		}
		value.Name = symbol.Name
		value.File = displayLocation(file.path, symbol.Line)
		value.Signature = symbol.Signature
		value.Description = ""
		value.Usage = ""
		verified = append(verified, value)
	}
	return verified
}

// VerifyEvidenceLocations 只保留能由结构化符号目录确认的位置。
func (v *Verifier) VerifyEvidenceLocations(values []domain.PatternEvidenceLocation) []domain.PatternEvidenceLocation {
	verified := make([]domain.PatternEvidenceLocation, 0, len(values))
	for _, value := range values {
		file, line, ok := v.load(value.DisplayLocation())
		if !ok {
			continue
		}
		symbol, ok := FindSymbol(file.symbols, value.Symbol, value.Kind, line)
		if !ok {
			continue
		}
		verified = append(verified, domain.PatternEvidenceLocation{
			Path:       file.path,
			Line:       symbol.Line,
			Symbol:     symbol.Name,
			Kind:       symbol.Kind,
			Confidence: 1,
		})
	}
	return verified
}

// VerifyBusinessMethods 校验业务入口的符号身份，并以源码事实替换 AI 字段。
func (v *Verifier) VerifyBusinessMethods(values []domain.BusinessMethod) []domain.BusinessMethod {
	verified := make([]domain.BusinessMethod, 0, len(values))
	now := time.Now()
	for _, value := range values {
		file, line, ok := v.load(value.DisplayLocation())
		if !ok {
			continue
		}
		symbol, ok := findCallable(file.symbols, line, value.Name)
		if !ok {
			continue
		}
		location := displayLocation(file.path, symbol.Line)
		value.Name = symbol.Name
		value.Function = symbol.Signature
		value.Description = ""
		value.Usage = ""
		value.Prerequisites = ""
		value.Returns = ""
		value.Type = ""
		value.CodeLocation = domain.CodeLocation{
			HistoricalLocation: location,
			CurrentLocation:    location,
			Status:             domain.CodeLocationStatusValid,
			Confidence:         1,
			VerifiedAt:         now,
			CreatedAt:          now,
			UpdatedAt:          now,
			Snapshot: &domain.SymbolSnapshot{
				Kind:      symbol.Kind,
				Name:      symbol.Name,
				Signature: symbol.Signature,
			},
		}
		verified = append(verified, value)
	}
	return verified
}

func (v *Verifier) load(location string) (symbolFile, int, bool) {
	path, line := splitLocation(location)
	path = normalizeReferencePath(path)
	if path == "" {
		return symbolFile{}, 0, false
	}
	file, ok := v.files[path]
	if !ok {
		return symbolFile{}, 0, false
	}
	return file, line, len(file.symbols) > 0
}

func findCallable(symbols []Symbol, line int, name string) (Symbol, bool) {
	name = simpleSymbolName(name)
	if name == "" {
		return Symbol{}, false
	}
	return FindSymbol(symbols, name, "function", line)
}

func splitLocation(location string) (string, int) {
	location = strings.TrimSpace(filepath.ToSlash(location))
	if location == "" {
		return "", 0
	}
	colon := strings.LastIndex(location, ":")
	if colon < 0 {
		return location, 0
	}
	line, err := strconv.Atoi(location[colon+1:])
	if err != nil || line <= 0 {
		return location, 0
	}
	return location[:colon], line
}

func displayLocation(path string, line int) string {
	if line <= 0 {
		return path
	}
	return path + ":" + strconv.Itoa(line)
}
