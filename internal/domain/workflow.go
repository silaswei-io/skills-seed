package domain

import "time"

// Workflow 描述用户沉淀的任务工作流。
type Workflow struct {
	ID        string
	Name      string
	Content   string
	Scripts   []WorkflowScript
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WorkflowScript 描述工作流关联脚本。
type WorkflowScript struct {
	Path   string
	SHA256 string
	Mode   string
}

// WorkflowRepository 保存和读取工作流资源。
type WorkflowRepository interface {
	List() ([]Workflow, error)
	Get(id string) (*Workflow, error)
	Save(workflow Workflow) error
	ScriptsDir(id string) string
	Scripts(id string) ([]WorkflowScript, error)
}
