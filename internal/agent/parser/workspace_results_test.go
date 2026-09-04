package parser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkspaceParsersRejectInvalidJSON(t *testing.T) {
	for name, parse := range map[string]func(string) error{
		"plan":      func(output string) error { _, err := ParsePlanLearningAgendaResult(output); return err },
		"normalize": func(output string) error { _, err := ParseNormalizePatternsResult(output); return err },
		"profile":   func(output string) error { _, err := ParseWorkspaceProfile(output); return err },
		"spec":      func(output string) error { _, err := ParseWorkspaceSpec(output); return err },
	} {
		t.Run(name, func(t *testing.T) {
			require.Error(t, parse("not json"))
		})
	}
}

func TestParseWorkspaceProfileMapsAllCollections(t *testing.T) {
	profile, err := ParseWorkspaceProfile(`{
  "summary":"workspace",
  "projects":[{"project_id":"ca","responsibility":"CA","frameworks":["go"]}],
  "shared":[{"path":"shared/api.proto","description":"contract","consumers":["ca"],"producers":["ra"],"affected_projects":["ca","ra"]}],
  "contracts":[{"path":"contracts/ca.proto"}],
  "infra":[{"path":"deploy"}],
  "dependencies":[{"from_project_id":"ca","to":{"kind":"project","value":"ra"},"reason":"calls RA"}],
  "impact_routes":[{"path_pattern":"ca/**","project_ids":["ca"],"reason":"CA files"}]
}`)

	require.NoError(t, err)
	require.Equal(t, "workspace", profile.Summary)
	require.Len(t, profile.Projects, 1)
	require.Equal(t, "ca", profile.Projects[0].ID)
	require.Equal(t, []string{"ca"}, profile.Shared[0].Consumers)
	require.Equal(t, "project", string(profile.Dependencies[0].To.Kind))
	require.Len(t, profile.ImpactRoutes, 1)
}

func TestParseWorkspaceProfileInitializesOptionalCollections(t *testing.T) {
	profile, err := ParseWorkspaceProfile(`{"projects":[]}`)

	require.NoError(t, err)
	require.NotNil(t, profile.Shared)
	require.NotNil(t, profile.Contracts)
	require.NotNil(t, profile.Infra)
	require.NotNil(t, profile.Dependencies)
	require.NotNil(t, profile.ImpactRoutes)
}

func TestParseWorkspaceSpecMapsAllCollections(t *testing.T) {
	spec, err := ParseWorkspaceSpec(`{
  "routing":[{"path_pattern":"ca/**","project_ids":["ca"],"reason":"route"}],
  "rules":[{"title":"Rule","description":"Description","applies_to":[{"kind":"role","value":"admin"}],"source":"workspace_profile","evidence":["AGENTS.md"]}],
  "change_order":["one","two"],
  "parallel_agent_guidance":[{"scope":{"kind":"path","value":"shared"},"allowed":true,"condition":"disjoint"}],
  "load_multiple_skills_when":[{"condition":"cross project","project_ids":["ca","ra"],"reason":"shared contract"}]
}`)

	require.NoError(t, err)
	require.Len(t, spec.Routing, 1)
	require.Len(t, spec.Rules[0].AppliesTo, 1)
	require.Equal(t, []string{"one", "two"}, spec.ChangeOrder)
	require.True(t, spec.ParallelAgentGuidance[0].Allowed)
	require.Equal(t, []string{"ca", "ra"}, spec.LoadMultipleSkillsWhen[0].ProjectIDs)
}

func TestParseWorkspaceSpecInitializesOptionalCollections(t *testing.T) {
	spec, err := ParseWorkspaceSpec(`{"routing":[],"rules":[]}`)

	require.NoError(t, err)
	require.NotNil(t, spec.ChangeOrder)
	require.NotNil(t, spec.ParallelAgentGuidance)
	require.NotNil(t, spec.LoadMultipleSkillsWhen)
}

func TestParseNormalizePatternsResultMapsPatternsAndDrops(t *testing.T) {
	result, err := ParseNormalizePatternsResult(`{"patterns":[{"id":"canonical","name":"Canonical","category":"business","description":"description","rule":"rule","confidence":0.9,"source_ids":["a"]}],"dropped":[{"id":"b","reason_code":"unsupported_evidence","reason":"not enough evidence"}]}`)

	require.NoError(t, err)
	require.Len(t, result.Patterns, 1)
	require.Equal(t, []string{"a"}, result.Patterns[0].SourceIDs)
	require.Equal(t, "unsupported_evidence", result.Dropped[0].ReasonCode)
}
