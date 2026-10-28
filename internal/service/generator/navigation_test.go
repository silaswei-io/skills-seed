package generator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestLocalKnowledgeDropsChangedEvidenceWithoutChangingBaseline(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	cfg, err := config.NewRepository(filepath.Join(root, ".skills-seed"), "en-US")
	require.NoError(t, err)
	repo, err := boltdb.NewPatternRepository(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	defer repo.Close()
	source := filepath.Join(root, "main.go")
	require.NoError(t, os.WriteFile(source, []byte("package sample\nconst Limit = 10\n"), 0600))
	changes, err := fileanalysis.PrepareCurrentChanges(ctx, repo, cfg, root, root, domain.FileAnalysisScope{}, nil)
	require.NoError(t, err)
	require.NoError(t, fileanalysis.CommitCurrentChanges(ctx, repo, changes))
	pattern := domain.NewPattern("limit", "Limit", domain.CategoryBusiness)
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "main.go"}}
	service := NewGeneratorService(repo, repo, nil, skills.NewLoader("en-US"), cfg, nil, nil)
	_, patterns, err := service.localKnowledge(ctx, root, nil, []domain.Pattern{*pattern})
	require.NoError(t, err)
	require.Len(t, patterns, 1)
	require.NoError(t, os.WriteFile(source, []byte("package sample\nconst Limit = 20\n"), 0600))
	_, patterns, err = service.localKnowledge(ctx, root, nil, []domain.Pattern{*pattern})
	require.NoError(t, err)
	require.Empty(t, patterns)
	pending, err := fileanalysis.PrepareCurrentChanges(ctx, repo, cfg, root, root, domain.FileAnalysisScope{}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"main.go"}, pending.AddedOrModified)
}

func TestLocalGenerationWithoutProfile(t *testing.T) {
	if _, err := os.Stat("../../../internal/codegraph/assets/runtime.zip"); err != nil && os.Getenv("SKILLS_SEED_CODEGRAPH_HOME") == "" {
		t.Skip("run make bundle for the SDK integration test")
	}
	for _, locale := range []string{"zh-CN", "en-US"} {
		t.Run(locale, func(t *testing.T) {
			root := t.TempDir()
			cfg, err := config.NewRepository(filepath.Join(root, ".skills-seed"), locale)
			require.NoError(t, err)
			value := cfg.Get()
			value.Project.RootPath = root
			value.Project.Name = "sample"
			require.NoError(t, cfg.Update(value))
			require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package sample\nfunc SharedClient() {}\n"), 0600))
			service := NewGeneratorService(&mocks.MockPatternRepository{}, nil, nil, skills.NewLoader(locale), cfg, nil, nil)
			output := filepath.Join(root, ".agents/skills/sample-dev")
			require.NoError(t, service.GenerateSkills(context.Background(), output))
			entry, err := os.ReadFile(filepath.Join(output, "SKILL.md"))
			require.NoError(t, err)
			require.Contains(t, string(entry), "references/code-navigation.md")
			require.NotContains(t, string(entry), "references/project-overview.md")
			require.NoFileExists(t, filepath.Join(output, "references/project-overview.md"))
			nav, err := os.ReadFile(filepath.Join(output, "references/code-navigation.md"))
			require.NoError(t, err)
			require.Contains(t, string(nav), "skills-seed codegraph query")
			require.NoFileExists(t, filepath.Join(output, "references/capabilities.md"))
			require.NoDirExists(t, filepath.Join(output, "references/capabilities"))
			assertNoBrokenMarkdownLinks(t, output)
			pages, err := filepath.Glob(filepath.Join(output, "references/code-navigation/*.md"))
			require.NoError(t, err)
			require.Empty(t, pages)
		})
	}
}

func TestLocalCapabilityLinksReviewedSemanticDetail(t *testing.T) {
	if _, err := os.Stat("../../../internal/codegraph/assets/runtime.zip"); err != nil && os.Getenv("SKILLS_SEED_CODEGRAPH_HOME") == "" {
		t.Skip("run make bundle for the SDK integration test")
	}
	root := t.TempDir()
	path := "sm9.go"
	require.NoError(t, os.WriteFile(filepath.Join(root, path), []byte("package crypto\nfunc EncryptSM9(key, data []byte) ([]byte, error) { return data, nil }\n"), 0600))
	hash, err := sourcecode.EvidenceDigest(root, path)
	require.NoError(t, err)
	pattern := domain.NewPattern("sm9-encryption", "SM9 encryption", domain.CategoryUtils)
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: path, Line: 2, ContentHash: hash}}
	pattern.BusinessMethod = &domain.BusinessMethod{
		Name: "EncryptSM9", CodeLocation: domain.CodeLocation{CurrentLocation: "sm9.go:2"},
		Function:    "func EncryptSM9(key, data []byte) ([]byte, error)",
		Description: "Encrypts data using SM9.", Usage: "SM9 encryption requests",
		Prerequisites: "SM9 key", Returns: "ciphertext or error", Type: "common",
	}
	cfg, err := config.NewRepository(filepath.Join(root, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	value := cfg.Get()
	value.Project.RootPath = root
	value.Project.Name = "crypto"
	require.NoError(t, cfg.Update(value))
	service := NewGeneratorService(&mocks.MockPatternRepository{GetAllFn: func(context.Context) ([]domain.Pattern, error) {
		return []domain.Pattern{*pattern}, nil
	}}, nil, nil, skills.NewLoader("zh-CN"), cfg, nil, nil)
	output := filepath.Join(root, ".agents/skills/crypto-dev")
	require.NoError(t, service.GenerateSkills(context.Background(), output))
	nav := readGeneratedFile(t, output, "references", "code-navigation.md")
	require.Contains(t, nav, "./business-methods.md")
	methodIndex := readGeneratedFile(t, output, "references", "business-methods.md")
	require.Contains(t, methodIndex, "./business-methods/"+businessMethodDetailName(*pattern.BusinessMethod))
	require.NoFileExists(t, filepath.Join(output, "references", "capabilities.md"))
	assertNoBrokenMarkdownLinks(t, output)
}
