package generator

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	workflowoutput "github.com/silaswei-io/skills-seed/internal/service/workflow/output"
)

func (s *GeneratorService) loadWorkflowReferences() ([]WorkflowReference, error) {
	return LoadWorkflowReferences(s.workflowRepo, s.skillsLoader.GetLocale())
}

func (s *GeneratorService) writeWorkflowOutputs(outputPath string) error {
	return WriteWorkflowOutputs(s.workflowRepo, outputPath, s.skillsLoader.GetLocale())
}

// LoadWorkflowReferences 读取当前目标的工作流引用，供项目和工作区 skill 共用。
func LoadWorkflowReferences(repo domain.WorkflowRepository, locale string) ([]WorkflowReference, error) {
	refs, err := workflowoutput.LoadReferences(repo, locale)
	if err != nil {
		return nil, err
	}
	result := make([]WorkflowReference, 0, len(refs))
	for _, ref := range refs {
		result = append(result, WorkflowReference{
			ID:          ref.ID,
			Name:        ref.Name,
			Path:        ref.Path,
			Description: ref.Description,
		})
	}
	return result, nil
}

// WriteWorkflowOutputs 把 .skills-seed/workflows 确定性复制到生成的 skill 目录。
func WriteWorkflowOutputs(repo domain.WorkflowRepository, outputPath, locale string) error {
	return workflowoutput.Write(repo, outputPath, locale)
}
