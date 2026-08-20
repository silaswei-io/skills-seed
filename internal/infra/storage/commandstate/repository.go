package commandstate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/jsonfile"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
)

const schemaVersion = 4

var (
	ErrStateNotFound            = errors.New("command state not found")
	ErrUnsupportedSchemaVersion = errors.New("unsupported command state schema version")
)

// InputSummary 记录创建可恢复计划时的输入规模，用于恢复时展示不可重算的阶段指标。
type InputSummary struct {
	SourceFiles         int `json:"source_files,omitempty"`
	LocalPlanInputFiles int `json:"local_plan_input_files,omitempty"`
	SelectionInputFiles int `json:"selection_input_files,omitempty"`
	SelectedFiles       int `json:"selected_files,omitempty"`
	SkippedFiles        int `json:"skipped_files,omitempty"`
}

// FocusKnowledgeCheckpoint 保存单个证据焦点的分析和审查结果。
// 焦点是独立审查与恢复的最小单元，不写入最终 Pattern 事实。
type FocusKnowledgeCheckpoint struct {
	Focus             domain.EvidenceFocus `json:"focus"`
	Patterns          []domain.Pattern     `json:"patterns,omitempty"`
	RetiredPatternIDs []string             `json:"retired_pattern_ids,omitempty"`
	Reviewed          bool                 `json:"reviewed,omitempty"`
}

// AnalysisCheckpoint 保存高成本分析的焦点结果，供失败后从未完成焦点继续。
type AnalysisCheckpoint struct {
	FocusKnowledge       []FocusKnowledgeCheckpoint `json:"focus_knowledge,omitempty"`
	ProfileRefreshNeeded bool                       `json:"profile_refresh_needed,omitempty"`
	ProfileRefreshReason string                     `json:"profile_refresh_reason,omitempty"`
}

// DecisionCheckpoint 保存一次已完成的规范化决策，等待本地校验和原子提交。
type DecisionCheckpoint struct {
	CandidateHash string          `json:"candidate_hash"`
	Decision      json.RawMessage `json:"decision"`
}

// PatternCommitSummary 保存 Pattern 入库阶段的可恢复摘要。
type PatternCommitSummary struct {
	Found   int `json:"found,omitempty"`
	Saved   int `json:"saved,omitempty"`
	Retired int `json:"retired,omitempty"`
}

// KnowledgeCommitCheckpoint 记录知识事实的分阶段提交结果。
// ID 与本次恢复状态的输入绑定，用于审计和避免将不同调用的提交结果混用。
type KnowledgeCommitCheckpoint struct {
	ID                      string               `json:"id"`
	PatternsCommitted       bool                 `json:"patterns_committed,omitempty"`
	PatternSummary          PatternCommitSummary `json:"pattern_summary,omitempty"`
	SourceBaselineCommitted bool                 `json:"source_baseline_committed,omitempty"`
	ProjectionsCommitted    bool                 `json:"projections_committed,omitempty"`
}

// State 是命令未完成执行的可恢复状态。
type State struct {
	SchemaVersion int    `json:"schema_version"`
	Command       string `json:"command"`
	ProjectName   string `json:"project_name"`
	Language      string `json:"language"`
	Mode          string `json:"mode,omitempty"`
	ChangeProfile string `json:"change_profile,omitempty"`
	UserContext   string `json:"user_context_hash,omitempty"`
	// InvocationHash 绑定影响本轮分析范围的参数，防止不兼容调用复用旧状态。
	InvocationHash string                      `json:"invocation_hash,omitempty"`
	CreatedAt      string                      `json:"created_at"`
	InputSummary   *InputSummary               `json:"input_summary,omitempty"`
	Files          []domain.FileAnalysisRecord `json:"files"`
	Deleted        []string                    `json:"deleted"`
	Agenda         domain.LearningAgenda       `json:"agenda"`
	// Analysis 保存已完成证据焦点及其结果；阶段完成状态由议程覆盖关系推导。
	Analysis *AnalysisCheckpoint `json:"analysis,omitempty"`
	// Decision 保存与当前候选集合绑定的规范化决策。
	Decision *DecisionCheckpoint `json:"decision,omitempty"`
	// KnowledgeCommit 保存可恢复知识提交的细粒度检查点。
	KnowledgeCommit KnowledgeCommitCheckpoint `json:"knowledge_commit"`
}

// Repository 读写某个命令的恢复状态。
type Repository struct {
	path    string
	command string
}

// NewRepository 创建命令状态仓储。
func NewRepository(seedPath, command string) *Repository {
	command = normalizeCommand(command)
	return &Repository{
		path:    layout.New(seedPath).CommandState(command),
		command: command,
	}
}

// Path 返回命令状态文件路径。
func (r *Repository) Path() string {
	return r.path
}

// Command 返回该仓储对应的命令 scope。
func (r *Repository) Command() string {
	return r.command
}

// Load 读取命令状态。
func (r *Repository) Load(ctx context.Context) (*State, error) {
	state, err := stateStore(r.path).Get(ctx)
	if err != nil {
		return nil, err
	}
	if state.SchemaVersion != schemaVersion {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrUnsupportedSchemaVersion, state.SchemaVersion, schemaVersion)
	}
	state.ensureKnowledgeCommitID()
	return state, nil
}

// Save 写入命令状态。
func (r *Repository) Save(ctx context.Context, state *State) error {
	if state == nil {
		return errors.New("command state is nil")
	}
	if state.SchemaVersion == 0 {
		state.SchemaVersion = schemaVersion
	}
	if state.SchemaVersion != schemaVersion {
		return fmt.Errorf("%w: got %d, want %d", ErrUnsupportedSchemaVersion, state.SchemaVersion, schemaVersion)
	}
	if strings.TrimSpace(state.Command) == "" {
		state.Command = r.command
	}
	if strings.TrimSpace(state.CreatedAt) == "" {
		state.CreatedAt = time.Now().Format(time.RFC3339)
	}
	state.ensureKnowledgeCommitID()
	return stateStore(r.path).Save(ctx, state)
}

// Clear 删除命令状态。
func (r *Repository) Clear() error {
	if err := os.Remove(r.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// NewState 创建规范化命令状态。
func NewState(command, projectName, language, userContext string, files []domain.FileAnalysisRecord, deleted []string, focuses []domain.EvidenceFocus) *State {
	return NewStateWithMode(command, projectName, language, "", userContext, files, deleted, focuses)
}

// NewStateWithMode 创建包含学习模式的命令状态。
func NewStateWithMode(command, projectName, language, mode, userContext string, files []domain.FileAnalysisRecord, deleted []string, focuses []domain.EvidenceFocus) *State {
	return &State{
		SchemaVersion: schemaVersion,
		Command:       normalizeCommand(command),
		ProjectName:   projectName,
		Language:      language,
		Mode:          strings.TrimSpace(mode),
		UserContext:   HashText(userContext),
		CreatedAt:     time.Now().Format(time.RFC3339),
		Files:         normalizeFiles(files),
		Deleted:       normalizePaths(deleted),
		Agenda:        domain.LearningAgenda{Focuses: normalizeFocuses(focuses)},
	}
}

// WithInputSummary 设置创建计划时的输入规模摘要。
func (s *State) WithInputSummary(summary InputSummary) *State {
	if s == nil {
		return nil
	}
	s.InputSummary = &summary
	return s
}

// WithInvocationHash 设置创建分析计划时的调用上下文摘要。
func (s *State) WithInvocationHash(hash string) *State {
	if s == nil {
		return nil
	}
	s.InvocationHash = strings.TrimSpace(hash)
	return s
}

// WithChangeProfile 设置本轮 learn current 的增量类型。
func (s *State) WithChangeProfile(profile string) *State {
	if s == nil {
		return nil
	}
	s.ChangeProfile = strings.TrimSpace(profile)
	return s
}

// KnowledgeCommitCheckpoint 返回状态关联的知识提交检查点。
func (s *State) KnowledgeCommitCheckpoint() *KnowledgeCommitCheckpoint {
	if s == nil {
		return nil
	}
	s.ensureKnowledgeCommitID()
	return &s.KnowledgeCommit
}

// PatternsCommitComplete 报告 Pattern mutation 是否已提交。
func (s *State) PatternsCommitComplete() bool {
	checkpoint := s.KnowledgeCommitCheckpoint()
	return checkpoint != nil && checkpoint.PatternsCommitted
}

// SourceBaselineCommitComplete 报告源码快照与文件指纹是否均已提交。
func (s *State) SourceBaselineCommitComplete() bool {
	checkpoint := s.KnowledgeCommitCheckpoint()
	return checkpoint != nil && checkpoint.SourceBaselineCommitted
}

// ProjectionsCommitComplete 报告画像、规范与源码事实投影是否均已提交。
func (s *State) ProjectionsCommitComplete() bool {
	checkpoint := s.KnowledgeCommitCheckpoint()
	return checkpoint != nil && checkpoint.ProjectionsCommitted
}

// MarkPatternsCommitted 标记 Pattern mutation 已完成，并保存本次提交摘要。
func (s *State) MarkPatternsCommitted(summary PatternCommitSummary) {
	if checkpoint := s.KnowledgeCommitCheckpoint(); checkpoint != nil {
		checkpoint.PatternsCommitted = true
		checkpoint.PatternSummary = normalizePatternCommitSummary(summary)
	}
}

// CommittedPatternSummary 返回已提交 Pattern mutation 的摘要。
func (s *State) CommittedPatternSummary() PatternCommitSummary {
	checkpoint := s.KnowledgeCommitCheckpoint()
	if checkpoint == nil || !checkpoint.PatternsCommitted {
		return PatternCommitSummary{}
	}
	return normalizePatternCommitSummary(checkpoint.PatternSummary)
}

// MarkSourceBaselineCommitted 标记源码快照与文件指纹均已完成。
func (s *State) MarkSourceBaselineCommitted() {
	if checkpoint := s.KnowledgeCommitCheckpoint(); checkpoint != nil {
		checkpoint.SourceBaselineCommitted = true
	}
}

// MarkProjectionsCommitted 标记项目画像和规范投影均已完成。
func (s *State) MarkProjectionsCommitted() {
	if checkpoint := s.KnowledgeCommitCheckpoint(); checkpoint != nil {
		checkpoint.ProjectionsCommitted = true
	}
}

func (s *State) ensureKnowledgeCommitID() {
	if s == nil {
		return
	}
	if strings.TrimSpace(s.KnowledgeCommit.ID) == "" {
		s.KnowledgeCommit.ID = s.knowledgeCommitID()
	}
}

func normalizePatternCommitSummary(summary PatternCommitSummary) PatternCommitSummary {
	if summary.Found < 0 {
		summary.Found = 0
	}
	if summary.Saved < 0 {
		summary.Saved = 0
	}
	if summary.Retired < 0 {
		summary.Retired = 0
	}
	return summary
}

func (s *State) knowledgeCommitID() string {
	if s == nil {
		return ""
	}
	type identity struct {
		Command        string                      `json:"command"`
		ProjectName    string                      `json:"project_name"`
		Language       string                      `json:"language"`
		Mode           string                      `json:"mode"`
		UserContext    string                      `json:"user_context_hash"`
		InvocationHash string                      `json:"invocation_hash"`
		Files          []domain.FileAnalysisRecord `json:"files"`
		Deleted        []string                    `json:"deleted"`
		Agenda         domain.LearningAgenda       `json:"agenda"`
	}
	data, _ := json.Marshal(identity{
		Command:        s.Command,
		ProjectName:    s.ProjectName,
		Language:       s.Language,
		Mode:           s.Mode,
		UserContext:    s.UserContext,
		InvocationHash: s.InvocationHash,
		Files:          s.Files,
		Deleted:        s.Deleted,
		Agenda:         s.Agenda,
	})
	return HashText(string(data))
}

// HashText 返回文本的稳定 SHA-256 摘要。
func HashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func stateStore(path string) jsonfile.Store[State] {
	return jsonfile.Store[State]{
		Path:     path,
		NotFound: ErrStateNotFound,
		NilValue: errors.New("command state is nil"),
		Labels: jsonfile.Labels{
			Read:      "read command state failed",
			Parse:     "parse command state failed",
			CreateDir: "create command state directory failed",
			Marshal:   "marshal command state failed",
			Write:     "write command state failed",
		},
	}
}

func normalizeCommand(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return "default"
	}
	var b strings.Builder
	b.Grow(len(command))
	lastDash := false
	for _, r := range strings.ToLower(command) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-_.")
	if out == "" {
		return "default"
	}
	return out
}

func normalizeFiles(files []domain.FileAnalysisRecord) []domain.FileAnalysisRecord {
	out := make([]domain.FileAnalysisRecord, 0, len(files))
	for _, record := range files {
		record.Path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(record.Path)))
		if record.Path == "" || record.Path == "." {
			continue
		}
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func normalizeFocuses(focuses []domain.EvidenceFocus) []domain.EvidenceFocus {
	out := make([]domain.EvidenceFocus, 0, len(focuses))
	for _, focus := range focuses {
		focus.ID = strings.TrimSpace(focus.ID)
		focus.Name = strings.TrimSpace(focus.Name)
		focus.EntryPaths = normalizePaths(focus.EntryPaths)
		focus.RelatedPaths = normalizePaths(focus.RelatedPaths)
		focus.RouteTerms = normalizeStrings(focus.RouteTerms)
		focus.Attributes = normalizeStrings(focus.Attributes)
		focus.RiskSignals = normalizeStrings(focus.RiskSignals)
		if focus.AnalysisDepth != "" {
			focus.AnalysisDepth = focus.EffectiveAnalysisDepth()
		}
		if focus.ID == "" || len(focusPaths(focus)) == 0 {
			continue
		}
		out = append(out, focus)
	}
	return out
}

func normalizePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
		if path == "" || path == "." || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func normalizeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func focusPaths(focus domain.EvidenceFocus) []string {
	paths := append([]string{}, focus.EntryPaths...)
	paths = append(paths, focus.RelatedPaths...)
	return normalizePaths(paths)
}

// MarshalJSON keeps nil slices encoded as [] for stable state files.
func (s State) MarshalJSON() ([]byte, error) {
	type alias State
	if s.Files == nil {
		s.Files = []domain.FileAnalysisRecord{}
	}
	if s.Deleted == nil {
		s.Deleted = []string{}
	}
	if s.Agenda.Focuses == nil {
		s.Agenda.Focuses = []domain.EvidenceFocus{}
	}
	return json.Marshal(alias(s))
}
