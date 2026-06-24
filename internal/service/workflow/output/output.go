package output

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/fileio"
)

const (
	workflowOutputDirName = "workflows"
	scriptOutputDirName   = "scripts"
)

// Reference 描述生成 skill 中可引用的工作流文档。
type Reference struct {
	ID          string
	Name        string
	Path        string
	Description string
}

// LoadReferences 读取当前目标的工作流引用。
func LoadReferences(repo domain.WorkflowRepository) ([]Reference, error) {
	if repo == nil {
		return nil, nil
	}
	workflows, err := repo.List()
	if err != nil {
		return nil, err
	}
	refs := make([]Reference, 0, len(workflows))
	for _, workflow := range workflows {
		if strings.TrimSpace(workflow.ID) == "" {
			continue
		}
		refs = append(refs, Reference{
			ID:          workflow.ID,
			Name:        workflowDisplayName(workflow),
			Path:        workflowReferencePath(workflow.ID),
			Description: workflowDescription(workflow),
		})
	}
	return refs, nil
}

// Write 把 .skills-seed/workflows 确定性复制到生成的 skill 目录。
func Write(repo domain.WorkflowRepository, outputPath, locale string) error {
	if repo == nil {
		return nil
	}
	workflows, err := repo.List()
	if err != nil {
		return err
	}
	workflowsDir := filepath.Join(outputPath, workflowOutputDirName)
	scriptsRoot := workflowScriptsOutputRoot(outputPath)
	if err := os.RemoveAll(workflowsDir); err != nil {
		return err
	}
	if err := os.RemoveAll(scriptsRoot); err != nil {
		return err
	}
	if len(workflows) == 0 {
		return nil
	}
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(scriptsRoot, 0755); err != nil {
		return err
	}
	for _, workflow := range workflows {
		if strings.TrimSpace(workflow.ID) == "" {
			continue
		}
		if err := fileio.WriteFileAtomic(workflowOutputPath(outputPath, workflow.ID), []byte(renderWorkflowOutput(workflow, locale)), 0644); err != nil {
			return err
		}
		if err := copyWorkflowScripts(repo.ScriptsDir(workflow.ID), workflowScriptOutputDir(outputPath, workflow.ID)); err != nil {
			return err
		}
	}
	return nil
}

func renderWorkflowOutput(workflow domain.Workflow, locale string) string {
	if strings.TrimSpace(workflow.Content) != "" {
		return renderWorkflowContentWithScripts(workflow, locale)
	}
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(workflowDisplayName(workflow))
	b.WriteString("\n\n")
	if description := workflowDescription(workflow); description != "" {
		b.WriteString(description)
		b.WriteString("\n\n")
	}
	if len(workflow.Contexts) > 0 {
		b.WriteString("## ")
		b.WriteString(localizedText(locale, "上下文", "Context"))
		b.WriteString("\n\n")
		for _, item := range workflow.Contexts {
			if strings.TrimSpace(item.Content) == "" {
				continue
			}
			b.WriteString(strings.TrimSpace(item.Content))
			b.WriteString("\n\n")
		}
	}
	appendWorkflowScripts(&b, workflow, locale)
	return b.String()
}

func renderWorkflowContentWithScripts(workflow domain.Workflow, locale string) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(workflow.Content))
	b.WriteString("\n\n")
	appendWorkflowScripts(&b, workflow, locale)
	return b.String()
}

func appendWorkflowScripts(b *strings.Builder, workflow domain.Workflow, locale string) {
	if len(workflow.Scripts) == 0 {
		return
	}
	b.WriteString("## ")
	b.WriteString(localizedText(locale, "脚本", "Scripts"))
	b.WriteString("\n\n")
	for _, script := range workflow.Scripts {
		if strings.TrimSpace(script.Path) == "" {
			continue
		}
		b.WriteString("- `")
		b.WriteString(workflowScriptReferencePath(workflow.ID, script.Path))
		b.WriteString("`\n")
	}
}

func copyWorkflowScripts(srcDir, dstDir string) error {
	if err := os.RemoveAll(dstDir); err != nil {
		return err
	}
	if _, err := os.Stat(srcDir); err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(dstDir, 0755)
		}
		return err
	}
	return filepath.WalkDir(srcDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dstDir, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func workflowDisplayName(workflow domain.Workflow) string {
	if strings.TrimSpace(workflow.Name) != "" {
		return strings.TrimSpace(workflow.Name)
	}
	return workflow.ID
}

func workflowDescription(workflow domain.Workflow) string {
	for _, item := range workflow.Contexts {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		return firstLine(content)
	}
	return ""
}

func firstLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func workflowReferencePath(id string) string {
	return "./" + filepath.ToSlash(filepath.Join(workflowOutputDirName, id+".md"))
}

func workflowOutputPath(outputPath, id string) string {
	return filepath.Join(outputPath, workflowOutputDirName, id+".md")
}

func workflowScriptsOutputRoot(outputPath string) string {
	return filepath.Join(outputPath, scriptOutputDirName, workflowOutputDirName)
}

func workflowScriptOutputDir(outputPath, id string) string {
	return filepath.Join(workflowScriptsOutputRoot(outputPath), id)
}

func workflowScriptReferencePath(id, scriptPath string) string {
	return "../" + filepath.ToSlash(filepath.Join(scriptOutputDirName, workflowOutputDirName, id, scriptPath))
}

func localizedText(locale, zh, en string) string {
	if strings.HasPrefix(strings.ToLower(locale), "en") {
		return en
	}
	return zh
}
