package generator

import (
	"context"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	workflowstore "github.com/silaswei-io/skills-seed/internal/infra/storage/workflow"
	workflowsvc "github.com/silaswei-io/skills-seed/internal/service/workflow"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSkillsDoesNotSkipWhenWorkflowScriptChanges(t *testing.T) {
	ctx := context.Background()
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")

	seedPath := t.TempDir()
	dbPath := filepath.Join(seedPath, "project.db")
	patternRepo, err := boltdb.NewPatternRepository(dbPath)
	require.NoError(t, err)
	defer patternRepo.Close()
	require.NoError(t, patternRepo.Save(ctx, pattern))

	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	workflowRepo := workflowstore.NewRepository(seedPath)
	workflowSvc := workflowsvc.NewService(workflowRepo, mockAgent, "go")
	_, err = workflowSvc.UpsertWorkflow(ctx, workflowsvc.UpsertRequest{
		Name:    "deploy",
		Context: "发布前检查环境变量",
	})
	require.NoError(t, err)
	scriptPath := filepath.Join(seedPath, "workflows", "deploy", "scripts", "smoke-test.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho first\n"), 0755))
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "test",
				Language:    "go",
				Summary:     "Profile-backed project overview",
				GeneratedAt: "2026-05-19 12:00:00",
			}, nil
		},
	}
	svc := NewGeneratorService(patternRepo, mockProfile, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}, workflowRepo)
	outputPath := t.TempDir()

	require.NoError(t, svc.GenerateSkills(ctx, outputPath))
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho second\n"), 0755))
	require.NoError(t, svc.GenerateSkills(ctx, outputPath))

	generatedScript := readGeneratedFile(t, outputPath, "scripts", "workflows", "deploy", "smoke-test.sh")
	require.Contains(t, generatedScript, "echo second")
}

func TestGenerateSkillsRemovesDeletedWorkflowOutputs(t *testing.T) {
	ctx := context.Background()
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")

	seedPath := t.TempDir()
	dbPath := filepath.Join(seedPath, "project.db")
	patternRepo, err := boltdb.NewPatternRepository(dbPath)
	require.NoError(t, err)
	defer patternRepo.Close()
	require.NoError(t, patternRepo.Save(ctx, pattern))
	workflowRepo := workflowstore.NewRepository(seedPath)
	workflowSvc := workflowsvc.NewService(workflowRepo, &mocks.MockAgent{NameVal: "test", AvailableVal: true}, "go")
	_, err = workflowSvc.UpsertWorkflow(ctx, workflowsvc.UpsertRequest{Name: "deploy", Context: "发布前检查环境变量"})
	require.NoError(t, err)
	_, err = workflowSvc.UpsertWorkflow(ctx, workflowsvc.UpsertRequest{Name: "test", Context: "发布后执行 smoke test"})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "workflows", "test", "scripts", "smoke-test.sh"), []byte("#!/bin/sh\necho ok\n"), 0755))

	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{ProjectName: "test", Language: "go", Summary: "demo"}, nil
		},
	}
	svc := NewGeneratorService(patternRepo, mockProfile, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}, workflowRepo)
	outputPath := t.TempDir()

	require.NoError(t, svc.GenerateSkills(ctx, outputPath))
	require.FileExists(t, filepath.Join(outputPath, "workflows", "deploy.md"))
	require.FileExists(t, filepath.Join(outputPath, "workflows", "test.md"))
	require.FileExists(t, filepath.Join(outputPath, "scripts", "workflows", "test", "smoke-test.sh"))

	require.NoError(t, os.RemoveAll(filepath.Join(seedPath, "workflows", "test")))
	require.NoError(t, svc.GenerateSkills(ctx, outputPath))

	require.FileExists(t, filepath.Join(outputPath, "workflows", "deploy.md"))
	require.NoFileExists(t, filepath.Join(outputPath, "workflows", "test.md"))
	require.NoDirExists(t, filepath.Join(outputPath, "scripts", "workflows", "test"))
	skill := readGeneratedFile(t, outputPath, "SKILL.md")
	require.Contains(t, skill, "./workflows/deploy.md")
	require.NotContains(t, skill, "./workflows/test.md")
}

func TestGenerateSkillsWritesUserWorkflowsAndScripts(t *testing.T) {
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{ProjectName: "test", Language: "go", Summary: "demo"}, nil
		},
	}
	seedPath := t.TempDir()
	workflowRepo := workflowstore.NewRepository(seedPath)
	workflowSvc := workflowsvc.NewService(workflowRepo, &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		OptimizeWorkflowFn: func(ctx context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeWorkflowResult, error) {
			return &agent.OptimizeWorkflowResult{
				Title:   req.Name,
				Content: "# " + req.Name + "\n\n## 适用场景\n发布流程覆盖上线前环境与产物核验，以及上线后的冒烟验证。\n",
			}, nil
		},
	}, "go")
	_, err := workflowSvc.UpsertWorkflow(context.Background(), workflowsvc.UpsertRequest{
		Name:    "deploy",
		Context: "发布前检查环境变量和构建产物，发布后执行 smoke test",
	})
	require.NoError(t, err)
	scriptPath := filepath.Join(seedPath, "workflows", "deploy", "scripts", "smoke-test.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho ok\n"), 0755))

	svc := NewGeneratorService(mockPattern, mockProfile, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}, workflowRepo)
	outputPath := t.TempDir()

	require.NoError(t, svc.GenerateSkills(context.Background(), outputPath))

	skill := readGeneratedFile(t, outputPath, "SKILL.md")
	require.Contains(t, skill, "## 用户工作流")
	require.Contains(t, skill, "./workflows/deploy.md")
	require.Contains(t, skill, "发布流程覆盖上线前环境与产物核验，以及上线后的冒烟验证。")
	require.NotContains(t, skill, "发布前检查环境变量和构建产物，发布后执行 smoke test")
	workflowContent := readGeneratedFile(t, outputPath, "workflows", "deploy.md")
	require.Contains(t, workflowContent, "## 适用场景")
	require.Contains(t, workflowContent, "发布流程覆盖上线前环境与产物核验")
	require.FileExists(t, filepath.Join(outputPath, "scripts", "workflows", "deploy", "smoke-test.sh"))
	assertNoBrokenMarkdownLinks(t, outputPath)
}
