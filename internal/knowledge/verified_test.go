package knowledge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/repositoryscope"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/stretchr/testify/require"
)

type knowledgeResolverStub struct {
	catalog sourcecode.Catalog
	err     error
	refs    []sourcecode.Reference
}

func (r *knowledgeResolverStub) Resolve(_ context.Context, _ string, refs []sourcecode.Reference) (sourcecode.Catalog, error) {
	r.refs = append([]sourcecode.Reference(nil), refs...)
	return r.catalog, r.err
}

func TestVerifyProjectKnowledgeHandlesEmptyRootAndResolverErrors(t *testing.T) {
	profile := &domain.ProjectProfile{ProjectName: "demo"}
	patterns := []domain.Pattern{{ID: "p1"}}

	gotProfile, gotPatterns, err := VerifyProjectKnowledge(context.Background(), profile, patterns, " ", nil, repositoryscope.New(nil))
	require.NoError(t, err)
	require.Same(t, profile, gotProfile)
	require.Equal(t, patterns, gotPatterns)

	gotProfile, gotPatterns, err = VerifyProjectKnowledge(context.Background(), profile, patterns, t.TempDir(), nil, repositoryscope.New(nil))
	require.EqualError(t, err, "symbol resolver is required")
	require.Nil(t, gotProfile)
	require.Nil(t, gotPatterns)

	wantErr := errors.New("index unavailable")
	_, _, err = VerifyProjectKnowledge(context.Background(), profile, patterns, t.TempDir(), &knowledgeResolverStub{err: wantErr}, repositoryscope.New(nil))
	require.ErrorIs(t, err, wantErr)
	require.Contains(t, err.Error(), "resolve project knowledge symbols")
}

func TestVerifyProjectKnowledgeFiltersAndEnrichesKnowledge(t *testing.T) {
	root := t.TempDir()
	writeKnowledgeFile(t, root, "main.go", "package demo\nfunc Execute() {}\n")
	writeKnowledgeFile(t, root, "internal/core/service.go", "package core\n\nfunc Run() {}\n")
	writeKnowledgeFile(t, root, "pkg/shared/shared.go", "package shared\n")
	writeKnowledgeFile(t, root, "config/app.yaml", "enabled: true\n")
	writeKnowledgeFile(t, root, "excluded/secret.go", "package excluded\n")
	writeKnowledgeFile(t, root, "tests/helper.go", "package tests\n")

	profile := &domain.ProjectProfile{
		BusinessMethods: []domain.BusinessMethod{{Name: "Legacy", Function: "func Legacy()"}},
		CommonUtils:     []domain.UtilityFunction{{Name: "LegacyUtil"}},
		Layers: []domain.ArchitectureLayer{{
			Name:  "core",
			Files: []string{"main.go:2", "missing.go", "excluded/secret.go", "tests/helper.go"},
		}},
		KeyModules: []domain.ModuleInfo{
			{
				Name:         "Core",
				DisplayName:  "Core Service",
				Path:         "internal/core",
				Dependencies: []string{"Shared", "pkg/shared", "Core", "missing"},
				Dependents:   []string{"Shared"},
				KeyMethods:   []string{"Legacy"},
			},
			{
				Name:       "Shared",
				Path:       "pkg/shared",
				Dependents: []string{"Core", "core service"},
			},
			{Name: "Missing", Path: "missing/module"},
		},
	}
	updatedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	patterns := []domain.Pattern{
		{
			ID:          "verified",
			Name:        "Verified flow",
			Category:    domain.CategoryBusiness,
			Source:      domain.SourceLearnedCurrent,
			GoodExample: "func Run() {}",
			ScopePath:   "internal/core",
			UpdatedAt:   updatedAt,
			EvidenceLocations: []domain.PatternEvidenceLocation{
				{Path: "main.go", Line: 99, Symbol: "Execute", Kind: "function"},
				{Path: "config/app.yaml", Kind: "file"},
				{Path: "missing.go", Kind: "file"},
				{Path: "excluded/secret.go", Kind: "file"},
				{Path: "tests/helper.go", Kind: "file"},
				{Path: "main.go", Symbol: "Missing", Kind: "function"},
			},
			BusinessMethod: &domain.BusinessMethod{
				Name:          "Core.Run",
				CodeLocation:  domain.CodeLocation{CurrentLocation: "internal/core/service.go:20"},
				Description:   " Run the core workflow. ",
				Usage:         " Use for core work. ",
				Type:          " domain ",
				Function:      "stale signature",
				Prerequisites: " Initialized service. ",
				Returns:       " Result or error. ",
			},
		},
		{
			ID:                "invalid-learned",
			Name:              "No evidence",
			Category:          domain.CategoryStructure,
			Source:            domain.SourceLearned,
			GoodExample:       "not in source",
			ScopePath:         "missing/scope",
			EvidenceLocations: []domain.PatternEvidenceLocation{{Path: "missing.go", Kind: "file"}},
		},
		{
			ID:       "maintained",
			Name:     "Maintained rule",
			Category: domain.CategoryConfig,
			Source:   domain.SourceUserDefined,
			Rule:     "Keep this explicit rule.",
		},
	}
	resolver := &knowledgeResolverStub{catalog: sourcecode.Catalog{
		"main.go": {
			{Name: "Execute", Kind: "function", Line: 2, Signature: "func Execute()"},
		},
		"internal/core/service.go": {
			{Name: "Run", Kind: "function", Line: 3, Signature: "func Run()"},
		},
	}}

	gotProfile, gotPatterns, err := VerifyProjectKnowledge(
		context.Background(), profile, patterns, root, resolver, repositoryscope.New([]string{"excluded/**"}),
	)

	require.NoError(t, err)
	require.NotSame(t, profile, gotProfile)
	require.Len(t, gotPatterns, 2)
	require.Equal(t, "verified", gotPatterns[0].ID)
	require.Equal(t, updatedAt, gotPatterns[0].UpdatedAt)
	require.Equal(t, 2, gotPatterns[0].Frequency)
	require.Equal(t, []domain.PatternEvidenceLocation{
		{Path: "main.go", Line: 2, Symbol: "Execute", Kind: "function", Confidence: 1},
		{Path: "config/app.yaml", Kind: "file"},
	}, gotPatterns[0].EvidenceLocations)
	require.Equal(t, "func Run() {}", gotPatterns[0].GoodExample)
	require.Equal(t, "internal/core", gotPatterns[0].ScopePath)
	require.NotNil(t, gotPatterns[0].BusinessMethod)
	require.Equal(t, "Run", gotPatterns[0].BusinessMethod.Name)
	require.Equal(t, "func Run()", gotPatterns[0].BusinessMethod.Function)
	require.Equal(t, "internal/core/service.go:3", gotPatterns[0].BusinessMethod.DisplayLocation())
	require.Equal(t, "Run the core workflow.", gotPatterns[0].BusinessMethod.Description)
	require.Equal(t, "maintained", gotPatterns[1].ID)

	require.Nil(t, gotProfile.CommonUtils)
	require.Len(t, gotProfile.BusinessMethods, 1)
	require.Equal(t, "Run", gotProfile.BusinessMethods[0].Name)
	require.Equal(t, []string{"main.go"}, gotProfile.Layers[0].Files)
	require.Len(t, gotProfile.KeyModules, 2)
	require.Equal(t, []string{"pkg/shared"}, gotProfile.KeyModules[0].Dependencies)
	require.Equal(t, []string{"pkg/shared"}, gotProfile.KeyModules[0].Dependents)
	require.Equal(t, []string{"Run"}, gotProfile.KeyModules[0].KeyMethods)
	require.Equal(t, []string{"internal/core"}, gotProfile.KeyModules[1].Dependents)
	require.Empty(t, gotProfile.KeyModules[1].KeyMethods)

	for _, ref := range resolver.refs {
		require.NotEqual(t, "tests/helper.go", pathOnly(ref.Path))
	}
}

func TestKnowledgeVerifierPathAndSnippetChecks(t *testing.T) {
	root := t.TempDir()
	writeKnowledgeFile(t, root, "src/main.go", "package src\nfunc Run() {}\n")
	outside := filepath.Join(t.TempDir(), "outside.go")
	require.NoError(t, os.WriteFile(outside, []byte("package outside\n"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "linked.go")))
	v := knowledgeVerifier{
		root:    root,
		symbols: sourcecode.NewVerifier(nil),
		scope:   repositoryscope.New([]string{"excluded/**"}),
	}

	clean, full, ok := v.path("`src/main.go:2`")
	require.True(t, ok)
	require.Equal(t, "src/main.go", clean)
	expectedFull, err := filepath.EvalSymlinks(filepath.Join(root, "src/main.go"))
	require.NoError(t, err)
	require.Equal(t, expectedFull, full)
	require.False(t, v.exists("missing.go"))
	require.False(t, v.exists("../outside.go"))
	require.False(t, v.exists("linked.go"))
	require.False(t, v.exists("excluded/file.go"))
	require.False(t, (knowledgeVerifier{}).exists("src/main.go"))
	require.True(t, v.snippetExists("", nil))
	require.True(t, v.snippetExists("func Run() {}", []string{"src", "missing.go", "src/main.go"}))
	require.False(t, v.snippetExists("func Missing()", []string{"src/main.go"}))
}

func TestKnowledgeVerificationHelpers(t *testing.T) {
	refs := []sourcecode.Reference{
		{Path: "main.go", Name: "Run"},
		{Path: "tests/helper.go", Name: "Helper"},
		{Path: "../outside.go", Name: "Outside"},
	}
	require.Equal(t, []sourcecode.Reference{{Path: "main.go", Name: "Run"}, {Path: "../outside.go", Name: "Outside"}}, nonProcedureReferences(refs))

	method := domain.BusinessMethod{
		Name:         "Run",
		Function:     "func Run()",
		CodeLocation: domain.CodeLocation{CurrentLocation: "internal/core/run.go:5"},
	}
	duplicate := method
	duplicate.Name = "Duplicate"
	patterns := []domain.Pattern{{BusinessMethod: &duplicate}}
	require.Equal(t, []domain.BusinessMethod{method}, mergeBusinessMethods([]domain.BusinessMethod{
		{Name: "Missing location", Function: "func Missing()"},
		method,
	}, patterns))

	modules := verifiedModuleRelations([]domain.ModuleInfo{
		{Name: "API", DisplayName: "Public API", Path: "internal/api", Dependencies: []string{"Store", "store", "API", "missing"}},
		{Name: "Store", Path: "internal/store", Dependents: []string{"Public API"}},
	})
	require.Equal(t, []string{"internal/store"}, modules[0].Dependencies)
	require.Equal(t, []string{"internal/api"}, modules[1].Dependents)
	require.Empty(t, verifiedModuleRelationList([]string{"unknown"}, "self", nil))
	require.Equal(t, "internal/api", moduleRelationKey("`internal/api:10`"))

	methods := []domain.BusinessMethod{
		{Name: "Run", CodeLocation: domain.CodeLocation{CurrentLocation: "internal/api/run.go:2"}},
		{Name: "run", CodeLocation: domain.CodeLocation{CurrentLocation: "internal/api/other.go:3"}},
		{Name: "Store", CodeLocation: domain.CodeLocation{CurrentLocation: "internal/store/store.go:4"}},
		{Name: "", CodeLocation: domain.CodeLocation{CurrentLocation: "internal/api/empty.go:5"}},
	}
	require.Equal(t, []string{"Run"}, moduleMethods("internal/api", methods))
	require.Nil(t, moduleMethods("", methods))

	pattern := domain.Pattern{
		ScopePath: "internal/api",
		EvidenceLocations: []domain.PatternEvidenceLocation{
			{Path: "main.go"},
			{Path: ""},
		},
		BusinessMethod: &method,
	}
	require.Equal(t, []string{"main.go", "internal/api", "internal/core/run.go:5"}, patternPaths(pattern))
	require.Equal(t, "internal/core/run.go", pathOnly("`internal/core/run.go:5`"))
}

func writeKnowledgeFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
