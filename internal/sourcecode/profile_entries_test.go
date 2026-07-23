package sourcecode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestVerifierKeepsOnlySourceBackedUtilities(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "helper.go"), []byte(`package helper

func BuildResponse(value any) error { return nil }
`), 0o644))
	verifier := newTestVerifier(t, root)

	utilities := verifier.VerifyUtilities([]domain.UtilityFunction{
		{Name: "BuildResponse", File: "helper.go:99", Signature: "func BuildResponse(value any) error", Description: "unverified description"},
		{Name: "SuccessResp", File: "helper.go:3", Signature: "func SuccessResp()"},
	})

	require.Len(t, utilities, 1)
	require.Equal(t, "BuildResponse", utilities[0].Name)
	require.Equal(t, "helper.go:3", utilities[0].File)
	require.Empty(t, utilities[0].Description)
}

func TestVerifierKeepsOnlyVerifiedMethodFacts(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "component.go"), []byte(`package component

func Check(uuid string) error { return nil }
`), 0o644))
	verifier := newTestVerifier(t, root)

	methods := verifier.VerifyBusinessMethods([]domain.BusinessMethod{
		{
			Name:          "component.Check",
			CodeLocation:  domain.CodeLocation{CurrentLocation: "component.go:99"},
			Function:      "func Check(uuid string) error",
			Description:   "unverified behavior",
			Usage:         "unverified usage",
			Prerequisites: "unverified prerequisite",
			Returns:       "unverified result",
			Type:          "domain",
		},
		{
			Name:         "component.Missing",
			CodeLocation: domain.CodeLocation{CurrentLocation: "component.go:3"},
			Function:     "func Missing() error",
		},
	})

	require.Len(t, methods, 1)
	require.Equal(t, "Check", methods[0].Name)
	require.Equal(t, "component.go:3", methods[0].DisplayLocation())
	require.Equal(t, domain.CodeLocationStatusValid, methods[0].CodeLocation.Status)
	require.Equal(t, "func Check(uuid string) error", methods[0].Function)
	require.Empty(t, methods[0].Description)
	require.Empty(t, methods[0].Usage)
	require.Empty(t, methods[0].Prerequisites)
	require.Empty(t, methods[0].Returns)
	require.Empty(t, methods[0].Type)
}

func TestVerifierReplacesCandidateSignatureWithSourceSignature(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "service.go"), []byte(`package service

type Service struct{}

func (s *Service) Publish(ctx string) (bool, error) { return true, nil }
`), 0o644))
	verifier := newTestVerifier(t, root)

	methods := verifier.VerifyBusinessMethods([]domain.BusinessMethod{
		{
			Name:         "Service.Publish",
			Function:     "Publish(ctx int) (bool, error)",
			CodeLocation: domain.CodeLocation{CurrentLocation: "service.go:5"},
		},
	})

	require.Len(t, methods, 1)
	require.Equal(t, "func (s *Service) Publish(ctx string) (bool, error)", methods[0].Function)
	require.Equal(t, methods[0].Function, methods[0].CodeLocation.Snapshot.Signature)
}

func TestVerifierKeepsOnlyASTBackedEvidenceLocations(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "service.go"), []byte(`package service

func Publish() {}
`), 0o644))
	verifier := newTestVerifier(t, root)

	locations := verifier.VerifyEvidenceLocations([]domain.PatternEvidenceLocation{
		{Path: "service.go", Line: 99, Symbol: "Publish", Kind: "function", Description: "unverified semantics"},
		{Path: "service.go", Line: 3, Symbol: "Missing", Kind: "function"},
	})

	require.Len(t, locations, 1)
	require.Equal(t, domain.PatternEvidenceLocation{
		Path:       "service.go",
		Line:       3,
		Symbol:     "Publish",
		Kind:       "function",
		Confidence: 1,
	}, locations[0])
}

func newTestVerifier(t *testing.T, root string) *Verifier {
	t.Helper()
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	catalog := Catalog{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, entry.Name()))
		require.NoError(t, err)
		symbols, err := parseSymbols(entry.Name(), src)
		require.NoError(t, err)
		catalog[entry.Name()] = symbols
	}
	return NewVerifier(catalog)
}
