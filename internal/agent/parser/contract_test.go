package parser

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/stretchr/testify/require"
)

func TestValidateContractResultAppliesKnowledgeReviewSemantics(t *testing.T) {
	err := ValidateContractResult(aicontract.ContractReviewKnowledge, aicontract.StructuredOutputOptions{
		CandidateIDs: []string{"candidate-1"},
	}, `{
		"decisions": [{
			"candidate_id": "candidate-1",
			"verdict": "revise",
			"reason_code": "overclaimed",
			"reason": "The boundary needs revision."
		}]
	}`)

	require.ErrorContains(t, err, "revision")
}

func TestValidateContractResultChecksDynamicCandidateCoverage(t *testing.T) {
	err := ValidateContractResult(aicontract.ContractReviewKnowledge, aicontract.StructuredOutputOptions{
		CandidateIDs: []string{"candidate-1", "candidate-2"},
	}, `{
		"decisions": [{
			"candidate_id": "candidate-1",
			"verdict": "accept",
			"reason_code": "accepted",
			"reason": "The evidence supports the candidate."
		}]
	}`)

	require.ErrorContains(t, err, "candidate-2")
}

func TestValidateContractResultRejectsUnknownContract(t *testing.T) {
	require.ErrorContains(t, ValidateContractResult("missing", aicontract.StructuredOutputOptions{}, `{}`), "unknown AI output contract")
}

func TestValidateContractResultSupportsEveryOutputContract(t *testing.T) {
	tests := []struct {
		contract string
		output   string
	}{
		{aicontract.ContractUserDefinePattern, `{"id":"pattern-1","name":"Pattern","category":"business","description":"Description","good_example":"","bad_example":"","rule":"Rule","confidence":0.8,"frequency":1,"knowledge_flags":[]}`},
		{aicontract.ContractProjectProfile, `{}`},
		{aicontract.ContractAuthorityExtraction, `{}`},
		{aicontract.ContractAnalyzeCurrentCodebaseBatch, `{"focuses":[]}`},
		{aicontract.ContractAnalyzeCurrentDeltaBatch, `{"knowledge_changes":[],"profile_refresh_recommended":{"needed":false}}`},
		{aicontract.ContractPlanLearningAgenda, `{}`},
		{aicontract.ContractReviewKnowledge, `{"decisions":[{"candidate_id":"candidate-1","verdict":"accept","reason_code":"accepted","reason":"Supported."}]}`},
		{aicontract.ContractNormalizePatterns, `{}`},
		{aicontract.ContractWorkspaceProfile, `{}`},
		{aicontract.ContractWorkspaceSpec, `{}`},
		{aicontract.ContractOptimizeWorkflow, `{}`},
		{aicontract.ContractOptimizeRule, `{}`},
	}
	for _, test := range tests {
		t.Run(test.contract, func(t *testing.T) {
			require.NoError(t, ValidateContractResult(test.contract, aicontract.StructuredOutputOptions{}, test.output))
		})
	}
}

func TestContractValidatorAppliesSchemaBeforeBusinessSemantics(t *testing.T) {
	schema, err := aicontract.StructuredOutputSchema(aicontract.ContractReviewKnowledge)
	require.NoError(t, err)
	validator, err := CompileContractValidator(aicontract.ContractReviewKnowledge, schema, aicontract.StructuredOutputOptions{})
	require.NoError(t, err)

	require.ErrorContains(t, validator.Validate(`{"decisions":[{"candidate_id":"candidate-1"}]}`), "verdict")
	require.ErrorContains(t, validator.Validate(`{"decisions":[{"candidate_id":"candidate-1","verdict":"revise","reason_code":"overclaimed","reason":"Revise it."}]}`), "revision")
	require.NoError(t, validator.Validate(`{"decisions":[{"candidate_id":"candidate-1","verdict":"accept","reason_code":"accepted","reason":"Supported."}]}`))
	require.ErrorContains(t, (*ContractValidator)(nil).Validate(`{}`), "not configured")
}

func TestCompileContractValidatorRejectsInvalidSchema(t *testing.T) {
	_, err := CompileContractValidator(aicontract.ContractReviewKnowledge, `{"type":7}`, aicontract.StructuredOutputOptions{})
	require.ErrorContains(t, err, "compile structured output schema")
}
