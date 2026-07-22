package generator

import (
	"context"
	"errors"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func newTestService(mockPattern *mocks.MockPatternRepository) *GeneratorService {
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	return newGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)
}

func newGeneratorService(
	patternRepo domain.PatternRepository,
	profileRepo domain.ProjectProfileRepository,
	loader *skills.Loader,
	cfg config.Reader,
) *GeneratorService {
	return NewGeneratorService(patternRepo, profileRepo, loader, cfg, nil)
}

func TestGenerateSkills_NoPatterns(t *testing.T) {
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.NoError(t, err)
}

func TestGenerateSkills_RepoError(t *testing.T) {
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return nil, errors.New("db error")
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.Error(t, err)
}

func TestGenerateSkillsRendersSourceBackedDescriptionWithoutLearnedRule(t *testing.T) {
	pattern := domain.NewPattern("error-wrap", "Error Wrap", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap config load errors")
	pattern.SetRule("Wrap config errors with path context")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/service/config.go", Line: 42, Symbol: "LoadConfig", Kind: "function", Description: "wraps config errors", Confidence: 0.88},
	}

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))
	content := readGeneratedFile(t, tmpDir, "references", "patterns", "error.md")
	require.Contains(t, content, "Error Wrap")
	require.Contains(t, content, "可复用解决方案")
	require.Contains(t, content, "Wrap config load errors")
	require.NotContains(t, content, "Wrap config errors with path context")
}

func TestGenerateSkills_FillsMissingCategorySummaries(t *testing.T) {
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")
	pattern.SetExamples("return fmt.Errorf(\"load: %w\", err)", "return err")
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(tmpDir, "references", "patterns", "error.md"))
	assert.NoError(t, err)
}

func TestResolveOutputPathRejectsPathsOutsideProjectRoot(t *testing.T) {
	parent := t.TempDir()
	projectRoot := filepath.Join(parent, "repo")
	require.NoError(t, os.MkdirAll(projectRoot, 0755))
	svc := newGeneratorService(&mocks.MockPatternRepository{}, &mocks.MockProjectProfileRepository{}, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{RootPath: projectRoot},
	})

	inside, err := svc.resolveOutputPath(".agents/skills/demo")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(projectRoot, ".agents", "skills", "demo"), inside)

	_, err = svc.resolveOutputPath("../outside")
	require.Error(t, err)
}

func TestGenerateSkillsWithHooksReportsProjectSteps(t *testing.T) {
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
			return &domain.ProjectProfile{
				ProjectName: "test",
				Language:    "go",
				Summary:     "Profile-backed project overview",
				GeneratedAt: "2026-05-19 12:00:00",
			}, nil
		},
	}
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := newGeneratorService(mockPattern, mockProfile, loader, cfg)

	var started []string
	var completed []string
	err := svc.GenerateSkillsWithHooks(context.Background(), t.TempDir(), GenerateProgressHooks{
		OnStepStart: func(label string) {
			started = append(started, label)
		},
		OnStepComplete: func(label string) {
			completed = append(completed, label)
		},
	}, GenerateOptions{})

	require.NoError(t, err)
	require.Equal(t, []string{
		"解析输出目录",
		"加载已学习模式",
		"读取项目画像",
		"整理生成数据",
		"写入技能文件",
	}, started)
	require.Equal(t, started, completed)
}

func TestGenerateSkillsRebuildsWhenInputFingerprintUnchanged(t *testing.T) {
	ctx := context.Background()
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")

	dbPath := filepath.Join(t.TempDir(), "project.db")
	patternRepo, err := boltdb.NewPatternRepository(dbPath)
	require.NoError(t, err)
	defer patternRepo.Close()
	require.NoError(t, patternRepo.Save(ctx, pattern))

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
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := newGeneratorService(patternRepo, mockProfile, loader, cfg)
	outputPath := t.TempDir()

	require.NoError(t, svc.GenerateSkills(ctx, outputPath))
	require.NoError(t, svc.GenerateSkills(ctx, outputPath))
	require.FileExists(t, filepath.Join(outputPath, "SKILL.md"))
}

func TestGenerateSkillsRebuildsGeneratedOutputWithoutReadingExistingSkill(t *testing.T) {
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")
	pattern.SetExamples("const secretGoodExample = true", "const secretBadExample = true")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := newGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte("<!-- generated-by: skills-seed v0.0.4 -->\nconst secretExistingSkillContent = true"), 0644))

	err := svc.GenerateSkills(context.Background(), tmpDir)

	require.NoError(t, err)
	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.NotContains(t, skill, "secretExistingSkillContent")
	assert.NotContains(t, skill, "secretGoodExample")
	assert.NotContains(t, skill, "secretBadExample")
}

func TestGenerateSkills_OverwritesExistingUnmarkedSkill(t *testing.T) {
	pattern := domain.NewPattern("p1", "Manual Rule", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("existing skill rule")
	pattern.SetRule("replace existing skill output")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte("# Existing Skill\n"), 0644))

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))
	require.NotEqual(t, "# Existing Skill\n", readGeneratedFile(t, tmpDir, "SKILL.md"))
	require.FileExists(t, filepath.Join(tmpDir, "references", "project-spec.md"))
}

func TestGenerateSkills_OverwritesGeneratedSkill(t *testing.T) {
	pattern := domain.NewPattern("p1", "Generated Rule", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("generated skill rule")
	pattern.SetRule("overwrite generated skill")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte("<!-- generated-by: skills-seed v0.0.4 -->\nold generated skill\n"), 0644))

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	businessPattern := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	assert.Contains(t, businessPattern, "./business/generated-rule.md")
	assert.NotContains(t, businessPattern, "#### ✅ 代码证据")
	assert.NotContains(t, businessPattern, "```go")
	assert.Contains(t, readGeneratedFile(t, tmpDir, "references", "patterns", "business", "generated-rule.md"), "Generated Rule")
	assert.NotContains(t, readGeneratedFile(t, tmpDir, "SKILL.md"), "old generated skill")
}

func TestGenerateSkills_CodexWritesOpenAIMetadata(t *testing.T) {
	pattern := domain.NewPattern("p1", "Business Flow", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("Use the existing business flow")
	pattern.SetRule("Reuse the documented flow")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	loader := skills.NewLoaderForAgent("codex", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
		AgentCfg:   config.AgentConfig{Engine: "codex"},
	}
	svc := newGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)

	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.NoError(t, err)

	openAIPath := filepath.Join(tmpDir, "agents", "openai.yaml")
	content, err := os.ReadFile(openAIPath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "display_name")
	assert.Contains(t, string(content), "$test-dev")
}

func readGeneratedFile(t *testing.T, root string, parts ...string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
	require.NoError(t, err)
	return string(content)
}

func businessPattern(id, name, description, rule, example string) domain.Pattern {
	pattern := domain.NewPattern(id, name, domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.Frequency = 1
	pattern.SetDescription(description)
	pattern.SetRule(rule)
	pattern.SetExamples(example, "")
	return *pattern
}

func businessPatternWithLocation(id, name, description, rule, example, location string) domain.Pattern {
	pattern := businessPattern(id, name, description, rule, example)
	pattern.SetBusinessMethod(&domain.BusinessMethod{
		Name:         name,
		CodeLocation: domain.CodeLocation{CurrentLocation: location},
		Description:  description,
		Type:         "domain",
		Function:     strings.Split(example, "{")[0],
	})
	return pattern
}

func assertNoBrokenMarkdownLinks(t *testing.T, root string) {
	t.Helper()
	linkPattern := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		require.NoError(t, walkErr)
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		content, err := os.ReadFile(path)
		require.NoError(t, err)

		for _, match := range linkPattern.FindAllStringSubmatch(string(content), -1) {
			target := strings.TrimSpace(match[1])
			if target == "" || strings.HasPrefix(target, "#") || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			if fragmentIndex := strings.Index(target, "#"); fragmentIndex >= 0 {
				target = target[:fragmentIndex]
			}
			if target == "" {
				continue
			}

			targetPath := filepath.Clean(filepath.Join(filepath.Dir(path), target))
			_, err := os.Stat(targetPath)
			assert.NoErrorf(t, err, "broken link in %s: %s", path, match[1])
		}
		return nil
	})
	require.NoError(t, err)
}

func assertNoExcessiveBlankLines(t *testing.T, root string) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		require.NoError(t, walkErr)
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotContainsf(t, string(content), "\n\n\n", "excessive blank lines in %s", path)
		return nil
	})
	require.NoError(t, err)
}
