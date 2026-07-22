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

const schemaVersion = 1

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

// FileInput 记录命令状态覆盖文件的输入摘要。
type FileInput struct {
	Path   string `json:"path"`
	Hash   string `json:"hash,omitempty"`
	Status string `json:"status"`
}

// AnalysisCheckpoint 保存高成本分析的阶段结果，供失败后从未完成单元继续。
type AnalysisCheckpoint struct {
	Complete             bool                  `json:"complete,omitempty"`
	Patterns             []domain.Pattern      `json:"patterns,omitempty"`
	CompletedUnits       []domain.AnalysisUnit `json:"completed_units,omitempty"`
	ProfileRefreshNeeded bool                  `json:"profile_refresh_needed,omitempty"`
	ProfileRefreshReason string                `json:"profile_refresh_reason,omitempty"`
}

// State 是命令未完成执行的可恢复状态。
type State struct {
	SchemaVersion int    `json:"schema_version"`
	Command       string `json:"command"`
	ProjectName   string `json:"project_name"`
	Language      string `json:"language"`
	Mode          string `json:"mode,omitempty"`
	UserContext   string `json:"user_context_hash,omitempty"`
	// InvocationHash 绑定影响分析范围和计划的命令参数及配置，防止不兼容调用复用旧状态。
	InvocationHash string                `json:"invocation_hash,omitempty"`
	CreatedAt      string                `json:"created_at"`
	InputSummary   *InputSummary         `json:"input_summary,omitempty"`
	Inputs         []FileInput           `json:"inputs"`
	Units          []domain.AnalysisUnit `json:"units"`
	// Analysis 保存已完成单元及其结果；Complete 表示整个分析阶段已经完成。
	Analysis *AnalysisCheckpoint `json:"analysis,omitempty"`
	// ProfileCommitted 表示本轮需要刷新的项目画像已经持久化。
	ProfileCommitted bool `json:"profile_committed,omitempty"`
	// ArtifactsCommitted 表示本轮 patterns 已成功持久化，恢复时只需提交快照与指纹。
	ArtifactsCommitted bool `json:"artifacts_committed,omitempty"`
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
func NewState(command, projectName, language, userContext string, inputs []FileInput, units []domain.AnalysisUnit) *State {
	return NewStateWithMode(command, projectName, language, "", userContext, inputs, units)
}

// NewStateWithMode 创建包含学习模式的命令状态。
func NewStateWithMode(command, projectName, language, mode, userContext string, inputs []FileInput, units []domain.AnalysisUnit) *State {
	return &State{
		SchemaVersion: schemaVersion,
		Command:       normalizeCommand(command),
		ProjectName:   projectName,
		Language:      language,
		Mode:          strings.TrimSpace(mode),
		UserContext:   HashText(userContext),
		CreatedAt:     time.Now().Format(time.RFC3339),
		Inputs:        normalizeInputs(inputs),
		Units:         normalizeUnits(units),
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

func normalizeInputs(inputs []FileInput) []FileInput {
	out := make([]FileInput, 0, len(inputs))
	for _, input := range inputs {
		input.Path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(input.Path)))
		input.Status = strings.TrimSpace(input.Status)
		if input.Path == "" || input.Path == "." {
			continue
		}
		out = append(out, input)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func normalizeUnits(units []domain.AnalysisUnit) []domain.AnalysisUnit {
	out := make([]domain.AnalysisUnit, 0, len(units))
	for _, unit := range units {
		unit.ID = strings.TrimSpace(unit.ID)
		unit.Name = strings.TrimSpace(unit.Name)
		unit.EntryPaths = normalizePaths(unit.EntryPaths)
		unit.RelatedPaths = normalizePaths(unit.RelatedPaths)
		unit.RouteTerms = normalizeStrings(unit.RouteTerms)
		if unit.ID == "" || len(unitPaths(unit)) == 0 {
			continue
		}
		out = append(out, unit)
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

func unitPaths(unit domain.AnalysisUnit) []string {
	paths := append([]string{}, unit.EntryPaths...)
	paths = append(paths, unit.RelatedPaths...)
	return normalizePaths(paths)
}

// MarshalJSON keeps nil slices encoded as [] for stable state files.
func (s State) MarshalJSON() ([]byte, error) {
	type alias State
	if s.Inputs == nil {
		s.Inputs = []FileInput{}
	}
	if s.Units == nil {
		s.Units = []domain.AnalysisUnit{}
	}
	return json.Marshal(alias(s))
}
