package analyzer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/fileio"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
)

const structuralIndexSchemaVersion = 1

type structuralIndex struct {
	SchemaVersion int                         `json:"schema_version"`
	UpdatedAt     string                      `json:"updated_at"`
	Files         map[string]structuralRecord `json:"files"`
}

type structuralRecord struct {
	Path         string             `json:"path"`
	Hash         string             `json:"hash,omitempty"`
	Language     string             `json:"language,omitempty"`
	Module       string             `json:"module,omitempty"`
	Kind         string             `json:"kind,omitempty"`
	Generated    bool               `json:"generated,omitempty"`
	Test         bool               `json:"test,omitempty"`
	Symbols      []structuralSymbol `json:"symbols,omitempty"`
	Imports      []string           `json:"imports,omitempty"`
	EntrySignals []string           `json:"entry_signals,omitempty"`
	ValueSignals []string           `json:"value_signals,omitempty"`
}

type structuralSymbol struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Line int    `json:"line,omitempty"`
}

type structuralCandidate struct {
	record structuralRecord
	score  int
}

type structuralModuleSummary struct {
	module    string
	files     int
	highValue int
	generated int
	test      int
	unindexed int
}

func (s *AnalyzerService) updateStructuralIndex(ctx context.Context, projectRoot string, changes *fileanalysis.FileChanges) (*structuralIndex, error) {
	if changes == nil {
		return newStructuralIndex(), nil
	}
	indexPath := structuralIndexPath(ctx)
	index, err := loadStructuralIndex(indexPath)
	if err != nil {
		return nil, err
	}

	for _, path := range changes.Deleted {
		delete(index.Files, cleanFileSelectionPath(path))
	}

	recordsByPath := make(map[string]domain.FileAnalysisRecord, len(changes.Records))
	for _, record := range changes.Records {
		path := cleanFileSelectionPath(record.Path)
		if path != "" {
			recordsByPath[path] = record
		}
	}

	updatePaths := append([]string{}, changes.AddedOrModified...)
	for _, path := range changes.Unchanged {
		path = cleanFileSelectionPath(path)
		if path == "" {
			continue
		}
		if _, ok := index.Files[path]; !ok {
			updatePaths = append(updatePaths, path)
		}
	}
	updatePaths = normalizeFileSelectionPaths(updatePaths)
	if len(updatePaths) > 0 {
		collector := s.indexCollector(projectRoot)
		for _, path := range updatePaths {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			record := recordsByPath[path]
			index.Files[path] = s.buildStructuralRecord(projectRoot, collector, path, record)
		}
	}

	index.UpdatedAt = time.Now().Format(time.RFC3339)
	if indexPath != "" {
		if err := saveStructuralIndex(indexPath, index); err != nil {
			return nil, err
		}
	}
	return index, nil
}

func (s *AnalyzerService) indexCollector(projectRoot string) *treesitterCollector {
	if treeCollector, ok := s.structuralCollector.(*treesitterCollector); ok {
		return treeCollector.withPolicy(fileanalysis.NewConfiguredSelectionPolicy(s.configRepo, projectRoot))
	}
	cfg := config.StructuralConfig{}
	if s.configRepo != nil {
		cfg = s.configRepo.GetCurrentLearningConfig().Structural
	}
	return newStructuralCollector(cfg).withPolicy(fileanalysis.NewConfiguredSelectionPolicy(s.configRepo, projectRoot))
}

func (s *AnalyzerService) buildStructuralRecord(projectRoot string, collector *treesitterCollector, path string, analysisRecord domain.FileAnalysisRecord) structuralRecord {
	record := structuralRecord{
		Path:         path,
		Hash:         analysisRecord.Hash,
		Module:       fileSelectionModule(path),
		Kind:         fileSelectionKind(path),
		Generated:    isGeneratedSelectionPath(path),
		Test:         isTestSelectionPath(path),
		EntrySignals: fileSelectionPathSignals(path),
	}
	if record.Hash == "" {
		record.Hash = fileHash(filepath.Join(projectRoot, filepath.FromSlash(path)))
	}
	record.ValueSignals = structuralValueSignals(record)
	if record.Generated || record.Test || !isStructuralSourcePath(path) {
		return record
	}
	if collector == nil {
		return record
	}
	fileResult, ok := collector.collectFile(projectRoot, filepath.Join(projectRoot, filepath.FromSlash(path)))
	if !ok {
		return record
	}
	record.Language = fileResult.langName
	record.Symbols = structuralSymbols(fileResult.symbols)
	record.Imports = structuralImports(fileResult.imports)
	record.ValueSignals = structuralValueSignals(record)
	return record
}

func structuralIndexPath(ctx context.Context) string {
	seedPath := runtimecontext.SeedPath(ctx)
	if seedPath == "" {
		return ""
	}
	return layout.New(seedPath).Cache("structural-index", "current.json")
}

func loadStructuralIndex(path string) (*structuralIndex, error) {
	if path == "" {
		return newStructuralIndex(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newStructuralIndex(), nil
		}
		return nil, err
	}
	var index structuralIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}
	if index.SchemaVersion != structuralIndexSchemaVersion || index.Files == nil {
		return newStructuralIndex(), nil
	}
	return &index, nil
}

func saveStructuralIndex(path string, index *structuralIndex) error {
	if path == "" || index == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return fileio.WriteFileAtomic(path, data, 0644)
}

func newStructuralIndex() *structuralIndex {
	return &structuralIndex{
		SchemaVersion: structuralIndexSchemaVersion,
		Files:         map[string]structuralRecord{},
	}
}

func fileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func structuralSymbols(symbols []symbolInfo) []structuralSymbol {
	out := make([]structuralSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		if strings.TrimSpace(symbol.Name) == "" {
			continue
		}
		out = append(out, structuralSymbol(symbol))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func structuralImports(imports []importInfo) []string {
	seen := make(map[string]bool)
	for _, item := range imports {
		path := strings.TrimSpace(item.Path)
		if path != "" {
			seen[path] = true
		}
	}
	out := make([]string, 0, len(seen))
	for path := range seen {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func structuralValueSignals(record structuralRecord) []string {
	seen := make(map[string]bool)
	add := func(value string) {
		if value != "" {
			seen[value] = true
		}
	}
	for _, signal := range record.EntrySignals {
		switch signal {
		case "api_entry_path", "request_handler_file_name", "route_file_name", "web_entry_file_name":
			add("runtime_entry")
		case "workflow_entry_path", "business_flow_file_name":
			add("business_orchestration")
		case "background_entry_path", "background_file_name", "event_entry_path":
			add("async_entry")
		case "lifecycle_registration_path", "lifecycle_file_name", "module_registration":
			add("registration_boundary")
		case "business_service_file":
			add("business_service")
		case "project_manifest":
			add("project_boundary")
		}
	}
	for _, imp := range record.Imports {
		lower := strings.ToLower(imp)
		if strings.Contains(lower, "medusajs") || strings.HasPrefix(lower, "@") {
			add("framework_or_package_import")
		}
		if strings.HasPrefix(imp, ".") {
			add("local_dependency")
		}
	}
	return sortedFileSelectionStrings(seen)
}

type structuralSelectionContext struct {
	Text  string
	Stats StructuralSelectionStats
}

// StructuralSelectionStats 是 gotree 文件确认阶段的结构化统计。
type StructuralSelectionStats struct {
	CandidateFiles      int
	IndexedFiles        int
	HighValueCandidates int
	LowValueSummarized  int
}

func buildStructuralSelectionContext(index *structuralIndex, candidatePaths []string) structuralSelectionContext {
	candidates := normalizeFileSelectionPaths(candidatePaths)
	if index == nil {
		index = newStructuralIndex()
	}
	candidateSet := make(map[string]bool, len(candidates))
	for _, path := range candidates {
		candidateSet[path] = true
	}

	summaries := make(map[string]*structuralModuleSummary)
	var selected []structuralCandidate
	lowValue := 0
	for _, path := range candidates {
		record, ok := index.Files[path]
		if !ok {
			record = structuralRecord{Path: path, Module: fileSelectionModule(path), Kind: fileSelectionKind(path)}
		}
		summary := summaries[record.Module]
		if summary == nil {
			summary = &structuralModuleSummary{module: record.Module}
			summaries[record.Module] = summary
		}
		summary.files++
		if !ok {
			summary.unindexed++
		}
		if record.Generated {
			summary.generated++
		}
		if record.Test {
			summary.test++
		}
		score := structuralCandidateScore(record)
		if score <= 0 {
			if record.Generated || record.Test {
				lowValue++
			}
			continue
		}
		summary.highValue++
		selected = append(selected, structuralCandidate{record: record, score: score})
	}

	sort.Slice(selected, func(i, j int) bool {
		if selected[i].score != selected[j].score {
			return selected[i].score > selected[j].score
		}
		if selected[i].record.Module != selected[j].record.Module {
			return selected[i].record.Module < selected[j].record.Module
		}
		return selected[i].record.Path < selected[j].record.Path
	})

	var b strings.Builder
	b.WriteString("## File Selection Structural Context\n\n")
	b.WriteString("This compact context is generated from the local tree-sitter structural index. Select from High-Value Candidates; lower-value files are summarized locally and remain available for validation.\n\n")
	b.WriteString("### Status\n\n")
	stats := StructuralSelectionStats{
		CandidateFiles:      len(candidates),
		IndexedFiles:        countIndexedCandidates(index, candidateSet),
		HighValueCandidates: len(selected),
		LowValueSummarized:  lowValue,
	}
	b.WriteString(fmt.Sprintf("Candidate files: %d | Indexed files: %d | High-value candidates: %d | Low-value summarized: %d\n\n",
		stats.CandidateFiles, stats.IndexedFiles, stats.HighValueCandidates, stats.LowValueSummarized))

	b.WriteString("### Module Coverage\n\n")
	for _, summary := range sortedStructuralModuleSummaries(summaries) {
		b.WriteString(fmt.Sprintf("- %s: files=%d, high_value=%d", summary.module, summary.files, summary.highValue))
		if summary.generated > 0 {
			b.WriteString(fmt.Sprintf(", generated=%d", summary.generated))
		}
		if summary.test > 0 {
			b.WriteString(fmt.Sprintf(", test=%d", summary.test))
		}
		if summary.unindexed > 0 {
			b.WriteString(fmt.Sprintf(", unindexed=%d", summary.unindexed))
		}
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	b.WriteString("### High-Value Candidates\n\n")
	if len(selected) == 0 {
		b.WriteString("No high-value structural candidates were detected. Prefer explicit changed metadata and user focus paths.\n")
		return structuralSelectionContext{Text: b.String(), Stats: stats}
	}
	for _, candidate := range selected {
		record := candidate.record
		b.WriteString(fmt.Sprintf("#### %s\n", record.Path))
		b.WriteString(fmt.Sprintf("- module: %s\n", record.Module))
		b.WriteString(fmt.Sprintf("- score: %d\n", candidate.score))
		if len(record.EntrySignals) > 0 {
			b.WriteString(fmt.Sprintf("- entry_signals: %s\n", strings.Join(record.EntrySignals, ", ")))
		}
		if len(record.ValueSignals) > 0 {
			b.WriteString(fmt.Sprintf("- value_signals: %s\n", strings.Join(record.ValueSignals, ", ")))
		}
		if symbols := compactStructuralSymbols(record.Symbols); len(symbols) > 0 {
			b.WriteString(fmt.Sprintf("- symbols: %s\n", strings.Join(symbols, ", ")))
		}
		if imports := compactStructuralImports(record.Imports); len(imports) > 0 {
			b.WriteString(fmt.Sprintf("- imports: %s\n", strings.Join(imports, ", ")))
		}
		b.WriteByte('\n')
	}
	return structuralSelectionContext{Text: b.String(), Stats: stats}
}

func structuralCandidateScore(record structuralRecord) int {
	if record.Path == "" || record.Generated || record.Test {
		return 0
	}
	score := 0
	for _, signal := range record.EntrySignals {
		switch signal {
		case "api_entry_path", "route_file_name", "web_entry_file_name", "command_entry_path", "workflow_entry_path":
			score += 5
		case "event_entry_path", "background_entry_path", "module_registration", "lifecycle_registration_path":
			score += 4
		case "business_service_file", "business_flow_file_name":
			score += 3
		case "project_manifest":
			score += 2
		default:
			score++
		}
	}
	for _, signal := range record.ValueSignals {
		switch signal {
		case "business_orchestration", "runtime_entry", "async_entry":
			score += 3
		case "registration_boundary", "business_service":
			score += 2
		}
	}
	if len(record.Symbols) > 0 {
		score++
	}
	if len(record.Imports) > 0 {
		score++
	}
	if score < 5 {
		return 0
	}
	return score
}

func countIndexedCandidates(index *structuralIndex, candidateSet map[string]bool) int {
	count := 0
	for path := range candidateSet {
		if _, ok := index.Files[path]; ok {
			count++
		}
	}
	return count
}

func sortedStructuralModuleSummaries(summaries map[string]*structuralModuleSummary) []structuralModuleSummary {
	out := make([]structuralModuleSummary, 0, len(summaries))
	for _, summary := range summaries {
		out = append(out, *summary)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].highValue != out[j].highValue {
			return out[i].highValue > out[j].highValue
		}
		if out[i].files != out[j].files {
			return out[i].files > out[j].files
		}
		return out[i].module < out[j].module
	})
	return out
}

func compactStructuralSymbols(symbols []structuralSymbol) []string {
	out := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		if len(out) >= 6 {
			break
		}
		if symbol.Name == "" {
			continue
		}
		out = append(out, fmt.Sprintf("%s %s:%d", symbol.Kind, symbol.Name, symbol.Line))
	}
	return out
}

func compactStructuralImports(imports []string) []string {
	out := make([]string, 0, len(imports))
	for _, imp := range imports {
		if len(out) >= 6 {
			break
		}
		if strings.TrimSpace(imp) == "" {
			continue
		}
		out = append(out, imp)
	}
	return out
}

func isStructuralSourcePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".py", ".java", ".kt", ".rs", ".php", ".rb", ".c", ".cc", ".cpp", ".h", ".hpp", ".cs", ".swift":
		return true
	default:
		return false
	}
}
