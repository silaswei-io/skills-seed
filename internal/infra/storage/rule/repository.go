package rule

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/fileio"
	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
	"gopkg.in/yaml.v3"
)

const (
	ruleDirName      = "rules"
	ruleFileName     = "RULE.md"
	ruleMetaFileName = "metadata.yaml"
)

var ErrNotFound = errors.New("rule not found")

// Repository 把规则保存为 .skills-seed/rules/<id>/。
type Repository struct {
	root string
}

type ruleMetadata struct {
	ID               string   `yaml:"id"`
	Name             string   `yaml:"name"`
	Summary          string   `yaml:"summary,omitempty"`
	RouteTerms       []string `yaml:"route_terms,omitempty"`
	AffectedProjects []string `yaml:"affected_projects,omitempty"`
	Paths            []string `yaml:"paths,omitempty"`
	CreatedAt        string   `yaml:"created_at,omitempty"`
	UpdatedAt        string   `yaml:"updated_at,omitempty"`
}

// NewRepository 创建规则仓储。
func NewRepository(seedPath string) *Repository {
	return &Repository{root: filepath.Join(seedPath, ruleDirName)}
}

// List 读取所有规则。
func (r *Repository) List() ([]domain.Rule, error) {
	entries, err := os.ReadDir(r.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	rules := make([]domain.Rule, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rule, err := r.Get(entry.Name())
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
	}
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return rules, nil
}

// Get 读取指定规则。
func (r *Repository) Get(id string) (*domain.Rule, error) {
	id = ruleID(id)
	if id == "" {
		return nil, ErrNotFound
	}
	meta, err := r.readMetadata(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(r.root, id, ruleFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rule := metadataToRule(meta)
	if rule.ID == "" {
		rule.ID = id
	}
	rule.Content = strings.TrimSpace(string(data))
	return &rule, nil
}

// Save 写入指定规则。
func (r *Repository) Save(rule domain.Rule) error {
	rule.ID = ruleID(rule.ID)
	if rule.ID == "" {
		return fmt.Errorf("rule id is required")
	}
	if strings.TrimSpace(rule.Name) == "" {
		rule.Name = rule.ID
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}
	if rule.UpdatedAt.IsZero() {
		rule.UpdatedAt = time.Now()
	}
	content := strings.TrimSpace(rule.Content)
	if content == "" {
		return fmt.Errorf("rule content is required")
	}
	targetDir := filepath.Join(r.root, rule.ID)
	return fileio.ReplaceDir(targetDir, func(staging string) error {
		meta, err := yaml.Marshal(ruleToMetadata(rule))
		if err != nil {
			return err
		}
		if err := fileio.WriteFileAtomic(filepath.Join(staging, ruleMetaFileName), meta, 0o644); err != nil {
			return err
		}
		return fileio.WriteFileAtomic(filepath.Join(staging, ruleFileName), []byte(content+"\n"), 0o644)
	})
}

func (r *Repository) readMetadata(id string) (ruleMetadata, error) {
	data, err := os.ReadFile(filepath.Join(r.root, id, ruleMetaFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return ruleMetadata{}, ErrNotFound
		}
		return ruleMetadata{}, err
	}
	var meta ruleMetadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return ruleMetadata{}, err
	}
	meta.ID = ruleID(meta.ID)
	return meta, nil
}

func ruleToMetadata(rule domain.Rule) ruleMetadata {
	return ruleMetadata{
		ID:               rule.ID,
		Name:             strings.TrimSpace(rule.Name),
		Summary:          strings.TrimSpace(rule.Summary),
		RouteTerms:       stringx.UniqueNonBlank(rule.RouteTerms),
		AffectedProjects: stringx.UniqueNonBlank(rule.AffectedProjects),
		Paths:            stringx.UniqueNonBlank(rule.Paths),
		CreatedAt:        formatTime(rule.CreatedAt),
		UpdatedAt:        formatTime(rule.UpdatedAt),
	}
}

func metadataToRule(meta ruleMetadata) domain.Rule {
	return domain.Rule{
		ID:               ruleID(meta.ID),
		Name:             strings.TrimSpace(meta.Name),
		Summary:          strings.TrimSpace(meta.Summary),
		RouteTerms:       stringx.UniqueNonBlank(meta.RouteTerms),
		AffectedProjects: stringx.UniqueNonBlank(meta.AffectedProjects),
		Paths:            stringx.UniqueNonBlank(meta.Paths),
		CreatedAt:        parseTime(meta.CreatedAt),
		UpdatedAt:        parseTime(meta.UpdatedAt),
	}
}

func ruleID(value string) string { return runtimefiles.SafePart(value, "") }

func parseTime(value string) time.Time {
	t, _ := time.Parse(time.RFC3339, value)
	return t
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
