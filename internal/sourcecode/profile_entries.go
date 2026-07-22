package sourcecode

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

type symbolFile struct {
	path    string
	symbols []Symbol
}

// Verifier 使用本地源码校验领域对象中引用的源码事实。
type Verifier struct {
	root  string
	files map[string]symbolFile
}

// NewVerifier 创建以项目根目录为边界的源码事实校验器。
func NewVerifier(root string) *Verifier {
	return &Verifier{
		root:  filepath.Clean(root),
		files: make(map[string]symbolFile),
	}
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

// VerifyEvidenceLocations 只保留能由本地 AST 确认的符号位置。
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
	if path == "" || v.root == "" || v.root == "." || filepath.IsAbs(path) {
		return symbolFile{}, 0, false
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return symbolFile{}, 0, false
	}
	relative := filepath.ToSlash(clean)
	if cached, ok := v.files[relative]; ok {
		return cached, line, len(cached.symbols) > 0
	}
	fullPath := filepath.Join(v.root, clean)
	resolved, err := utils.CanonicalPathWithinRoot(v.root, fullPath)
	if err != nil {
		v.files[relative] = symbolFile{}
		return symbolFile{}, 0, false
	}
	src, err := os.ReadFile(resolved)
	if err != nil {
		v.files[relative] = symbolFile{}
		return symbolFile{}, 0, false
	}
	symbols, err := ParseSymbols(relative, src)
	if err != nil {
		v.files[relative] = symbolFile{}
		return symbolFile{}, 0, false
	}
	file := symbolFile{path: relative, symbols: symbols}
	v.files[relative] = file
	return file, line, len(symbols) > 0
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
