package workflow

import (
	"crypto/sha256"
	"encoding/hex"
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
	"gopkg.in/yaml.v3"
)

const (
	workflowDirName      = "workflows"
	workflowFileName     = "WORKFLOW.md"
	workflowMetaFileName = "metadata.yaml"
	scriptsDirName       = "scripts"
)

var ErrNotFound = errors.New("workflow not found")

// Repository 把工作流保存为 .skills-seed/workflows/<id>/。
type Repository struct {
	root string
}

type workflowMetadata struct {
	ID        string                    `yaml:"id"`
	Name      string                    `yaml:"name"`
	CreatedAt string                    `yaml:"created_at,omitempty"`
	UpdatedAt string                    `yaml:"updated_at,omitempty"`
	Contexts  []workflowContextMetadata `yaml:"contexts,omitempty"`
}

type workflowContextMetadata struct {
	Content   string `yaml:"content"`
	CreatedAt string `yaml:"created_at,omitempty"`
}

// NewRepository 创建工作流仓储。
func NewRepository(seedPath string) *Repository {
	return &Repository{root: filepath.Join(seedPath, workflowDirName)}
}

// List 读取所有工作流。
func (r *Repository) List() ([]domain.Workflow, error) {
	entries, err := os.ReadDir(r.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	workflows := make([]domain.Workflow, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		workflow, err := r.Get(entry.Name())
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		workflows = append(workflows, *workflow)
	}
	sort.SliceStable(workflows, func(i, j int) bool {
		return workflows[i].ID < workflows[j].ID
	})
	return workflows, nil
}

// Get 读取指定工作流。
func (r *Repository) Get(id string) (*domain.Workflow, error) {
	id = workflowID(id)
	if id == "" {
		return nil, ErrNotFound
	}

	meta, err := r.readMetadata(id)
	if err != nil {
		return nil, err
	}
	content, err := r.readContent(id)
	if err != nil {
		return nil, err
	}
	scripts, err := r.Scripts(id)
	if err != nil {
		return nil, err
	}
	workflow := metadataToWorkflow(meta)
	if workflow.ID == "" {
		workflow.ID = id
	}
	workflow.Content = content
	workflow.Scripts = scripts
	return &workflow, nil
}

// Save 写入指定工作流。
func (r *Repository) Save(workflow domain.Workflow) error {
	workflow.ID = workflowID(workflow.ID)
	if workflow.ID == "" {
		return fmt.Errorf("workflow id is required")
	}
	if strings.TrimSpace(workflow.Name) == "" {
		workflow.Name = workflow.ID
	}
	if workflow.CreatedAt.IsZero() {
		workflow.CreatedAt = time.Now()
	}
	if workflow.UpdatedAt.IsZero() {
		workflow.UpdatedAt = time.Now()
	}
	content := strings.TrimSpace(workflow.Content)
	if content == "" {
		content = "# " + strings.TrimSpace(workflow.Name)
	}
	targetDir := filepath.Join(r.root, workflow.ID)
	return fileio.ReplaceDir(targetDir, func(staging string) error {
		stagingScripts := filepath.Join(staging, scriptsDirName)
		existingScripts := filepath.Join(targetDir, scriptsDirName)
		if _, err := os.Stat(existingScripts); err == nil {
			if err := os.CopyFS(stagingScripts, os.DirFS(existingScripts)); err != nil {
				return err
			}
		} else if os.IsNotExist(err) {
			if err := os.MkdirAll(stagingScripts, 0o755); err != nil {
				return err
			}
		} else {
			return err
		}
		if err := fileio.WriteFileAtomic(filepath.Join(staging, workflowMetaFileName), renderWorkflowMetadata(workflow), 0o644); err != nil {
			return err
		}
		return fileio.WriteFileAtomic(filepath.Join(staging, workflowFileName), []byte(content+"\n"), 0o644)
	})
}

// ScriptsDir 返回工作流脚本目录。
func (r *Repository) ScriptsDir(id string) string {
	return filepath.Join(r.root, workflowID(id), scriptsDirName)
}

// Scripts 读取指定工作流的脚本清单。
func (r *Repository) Scripts(id string) ([]domain.WorkflowScript, error) {
	scriptsRoot := r.ScriptsDir(id)
	var scripts []domain.WorkflowScript
	if _, err := os.Stat(scriptsRoot); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if err := filepath.WalkDir(scriptsRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry == nil || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(scriptsRoot, path)
		if err != nil {
			return err
		}
		script, err := workflowScript(scriptsRoot, rel)
		if err != nil {
			return err
		}
		scripts = append(scripts, script)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.SliceStable(scripts, func(i, j int) bool {
		return scripts[i].Path < scripts[j].Path
	})
	return scripts, nil
}

func (r *Repository) readMetadata(id string) (workflowMetadata, error) {
	data, err := os.ReadFile(r.metadataPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return workflowMetadata{}, ErrNotFound
		}
		return workflowMetadata{}, err
	}
	var meta workflowMetadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return workflowMetadata{}, err
	}
	meta.ID = workflowID(meta.ID)
	return meta, nil
}

func (r *Repository) readContent(id string) (string, error) {
	data, err := os.ReadFile(r.workflowPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (r *Repository) workflowPath(id string) string {
	return filepath.Join(r.root, workflowID(id), workflowFileName)
}

func (r *Repository) metadataPath(id string) string {
	return filepath.Join(r.root, workflowID(id), workflowMetaFileName)
}

func workflowScript(root, rel string) (domain.WorkflowScript, error) {
	path := filepath.Join(root, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.WorkflowScript{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return domain.WorkflowScript{}, err
	}
	sum := sha256.Sum256(data)
	return domain.WorkflowScript{
		Path:   filepath.ToSlash(rel),
		SHA256: hex.EncodeToString(sum[:]),
		Mode:   fmt.Sprintf("%#o", info.Mode().Perm()),
	}, nil
}

func renderWorkflowMetadata(workflow domain.Workflow) []byte {
	data, _ := yaml.Marshal(workflowToMetadata(workflow))
	return data
}

func workflowToMetadata(workflow domain.Workflow) workflowMetadata {
	contexts := make([]workflowContextMetadata, 0, len(workflow.Contexts))
	for _, item := range workflow.Contexts {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		contexts = append(contexts, workflowContextMetadata{
			Content:   content,
			CreatedAt: formatWorkflowTime(item.CreatedAt),
		})
	}
	return workflowMetadata{
		ID:        workflow.ID,
		Name:      strings.TrimSpace(workflow.Name),
		CreatedAt: formatWorkflowTime(workflow.CreatedAt),
		UpdatedAt: formatWorkflowTime(workflow.UpdatedAt),
		Contexts:  contexts,
	}
}

func metadataToWorkflow(meta workflowMetadata) domain.Workflow {
	contexts := make([]domain.WorkflowContext, 0, len(meta.Contexts))
	for _, item := range meta.Contexts {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		contexts = append(contexts, domain.WorkflowContext{
			Content:   content,
			CreatedAt: parseWorkflowTime(item.CreatedAt),
		})
	}
	return domain.Workflow{
		ID:        workflowID(meta.ID),
		Name:      strings.TrimSpace(meta.Name),
		Contexts:  contexts,
		CreatedAt: parseWorkflowTime(meta.CreatedAt),
		UpdatedAt: parseWorkflowTime(meta.UpdatedAt),
	}
}

func workflowID(value string) string {
	return runtimefiles.SafePart(value, "")
}

func parseWorkflowTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, value)
	return t
}

func formatWorkflowTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
