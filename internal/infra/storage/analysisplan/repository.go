package analysisplan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

var ErrPlanNotFound = errors.New("analysis plan not found")

// FileInput 记录 plan 覆盖文件的输入摘要。
type FileInput struct {
	Path   string `json:"path"`
	Hash   string `json:"hash,omitempty"`
	Status string `json:"status"`
}

// Plan 是 learn current 当前未完成分析计划的可重建缓存。
type Plan struct {
	SchemaVersion int                   `json:"schema_version"`
	ProjectName   string                `json:"project_name"`
	Language      string                `json:"language"`
	UserContext   string                `json:"user_context_hash,omitempty"`
	CreatedAt     string                `json:"created_at"`
	Inputs        []FileInput           `json:"inputs"`
	Units         []domain.AnalysisUnit `json:"units"`
}

// Repository 读写 .skills-seed/cache/analysis/current/plan.json。
type Repository struct {
	path string
}

// NewRepository 创建 analysis plan cache 仓储。
func NewRepository(seedPath string) *Repository {
	return &Repository{path: layout.New(seedPath).CurrentAnalysisPlan()}
}

// Path 返回 plan cache 文件路径。
func (r *Repository) Path() string {
	return r.path
}

// Load 读取当前分析计划。
func (r *Repository) Load(ctx context.Context) (*Plan, error) {
	return planStore(r.path).Get(ctx)
}

// Save 写入当前分析计划。
func (r *Repository) Save(ctx context.Context, plan *Plan) error {
	if plan.SchemaVersion == 0 {
		plan.SchemaVersion = schemaVersion
	}
	if strings.TrimSpace(plan.CreatedAt) == "" {
		plan.CreatedAt = time.Now().Format(time.RFC3339)
	}
	return planStore(r.path).Save(ctx, plan)
}

// Clear 删除当前分析计划。
func (r *Repository) Clear() error {
	if err := os.Remove(r.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// NewPlan 创建规范化 plan。
func NewPlan(projectName, language, userContext string, inputs []FileInput, units []domain.AnalysisUnit) *Plan {
	return &Plan{
		SchemaVersion: schemaVersion,
		ProjectName:   projectName,
		Language:      language,
		UserContext:   HashText(userContext),
		CreatedAt:     time.Now().Format(time.RFC3339),
		Inputs:        normalizeInputs(inputs),
		Units:         normalizeUnits(units),
	}
}

// HashText 返回文本的稳定 SHA-256 摘要。
func HashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func planStore(path string) jsonfile.Store[Plan] {
	return jsonfile.Store[Plan]{
		Path:     path,
		NotFound: ErrPlanNotFound,
		NilValue: errors.New("analysis plan is nil"),
		Labels: jsonfile.Labels{
			Read:      "read analysis plan failed",
			Parse:     "parse analysis plan failed",
			CreateDir: "create analysis plan directory failed",
			Marshal:   "marshal analysis plan failed",
			Write:     "write analysis plan failed",
		},
	}
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

// MarshalJSON keeps nil slices encoded as [] for stable cache files.
func (p Plan) MarshalJSON() ([]byte, error) {
	type alias Plan
	if p.Inputs == nil {
		p.Inputs = []FileInput{}
	}
	if p.Units == nil {
		p.Units = []domain.AnalysisUnit{}
	}
	return json.Marshal(alias(p))
}
