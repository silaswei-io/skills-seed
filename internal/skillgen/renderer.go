package skillgen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
)

type Renderer struct {
	loader *skills.Loader
}

func NewRenderer(loader *skills.Loader) *Renderer {
	return &Renderer{loader: loader}
}

func (r *Renderer) Render(ctx context.Context, plan *Plan) error {
	if r == nil || r.loader == nil {
		return fmt.Errorf("%s", i18n.Get("SkillRendererLoaderMissing"))
	}
	if plan == nil {
		return fmt.Errorf("%s", i18n.Get("SkillRendererPlanMissing"))
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	startedAt := time.Now()
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "skill_renderer.render",
		"output_path", plan.OutputPath,
		"files_count", len(plan.Files),
	)

	if err := os.MkdirAll(plan.OutputPath, 0755); err != nil {
		return err
	}
	for _, dir := range plan.CreateDirs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(plan.OutputPath, dir), 0755); err != nil {
			return err
		}
	}
	for _, path := range plan.RemovePaths {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.RemoveAll(filepath.Join(plan.OutputPath, path)); err != nil {
			return err
		}
	}
	for _, file := range plan.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.renderFile(plan.OutputPath, file); err != nil {
			return err
		}
	}
	if plan.AgentMetadataData != nil {
		if err := r.renderAgentMetadata(ctx, plan.OutputPath, plan.AgentMetadataData); err != nil {
			return err
		}
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "skill_renderer.render",
		"duration", time.Since(startedAt),
		"output_path", plan.OutputPath,
		"files_count", len(plan.Files),
	)
	return nil
}

func (r *Renderer) renderFile(outputPath string, file File) error {
	content, err := r.renderContent(file)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(outputPath, file.Path)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		return err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "skill_renderer.render_file",
		"path", targetPath,
		"content_length", len(content),
	)
	return nil
}

func (r *Renderer) renderContent(file File) (string, error) {
	switch file.Kind {
	case CatalogTemplate:
		return r.loader.Render(file.Template, file.Data)
	case ReferenceTemplate:
		return r.loader.RenderReferenceFile(file.Template, file.Data)
	case PatternTemplate:
		return r.loader.RenderPattern(file.Template, file.Data)
	case ProjectOverviewTemplate:
		return r.loader.RenderProjectOverview(file.Data)
	case RelativeTemplate:
		return r.loader.RenderRelative(file.Template, file.Data)
	default:
		return "", fmt.Errorf("%s: %s", i18n.Get("SkillRendererTemplateKindUnsupported"), file.Kind)
	}
}

func (r *Renderer) renderAgentMetadata(ctx context.Context, outputPath string, data any) error {
	files, err := r.loader.RenderAgentMetadataFiles(data)
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		targetPath := filepath.Join(outputPath, file.Path)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, []byte(file.Content), 0644); err != nil {
			return err
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "skill_renderer.render_agent_metadata",
			"path", targetPath,
			"content_length", len(file.Content),
		)
	}
	return nil
}
