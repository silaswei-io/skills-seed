package aicontract

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStructuredOutputValidatorUsesDynamicReviewContract(t *testing.T) {
	schema, err := StructuredOutputSchemaWithOptions(ContractReviewKnowledge, StructuredOutputOptions{
		CandidateIDs: []string{"app-boot-auth-chain-init-guard"},
	})
	require.NoError(t, err)
	validator, err := CompileStructuredOutputValidator(schema)
	require.NoError(t, err)

	require.NoError(t, validator.Validate(`{
		"decisions": [{
			"candidate_id": "app-boot-auth-chain-init-guard",
			"verdict": "accept",
			"reason_code": "accepted",
			"reason": "The evidence supports the candidate."
		}]
	}`))

	err = validator.Validate(`{
		"decisions": [{
			"candidate_id": "app-boot-auth-chain-init-guard",
			"reason_code": "accepted",
			"reason": "The evidence supports the candidate."
		}]
	}`)
	require.ErrorContains(t, err, "verdict")
}

func TestStructuredOutputValidatorRejectsWrongDynamicID(t *testing.T) {
	schema, err := StructuredOutputSchemaWithOptions(ContractReviewKnowledge, StructuredOutputOptions{
		CandidateIDs: []string{"expected"},
	})
	require.NoError(t, err)
	validator, err := CompileStructuredOutputValidator(schema)
	require.NoError(t, err)

	err = validator.Validate(`{
		"decisions": [{
			"candidate_id": "unexpected",
			"verdict": "accept",
			"reason_code": "accepted",
			"reason": "The evidence supports the candidate."
		}]
	}`)

	require.ErrorContains(t, err, "candidate_id")
}

func TestStructuredOutputValidatorRejectsInvalidInputs(t *testing.T) {
	_, err := CompileStructuredOutputValidator(`{"type":`)
	require.ErrorContains(t, err, "decode structured output schema")

	validator, err := CompileStructuredOutputValidator(`{"type":"object"}`)
	require.NoError(t, err)
	require.ErrorContains(t, validator.Validate(`{"value":1} {"value":2}`), "multiple JSON values")
	require.ErrorContains(t, validator.Validate(`{"value":1} trailing`), "invalid character")
	require.ErrorContains(t, (*StructuredOutputValidator)(nil).Validate(`{}`), "not configured")
}

func TestCompileStructuredOutputValidatorRejectsInvalidSchemaKeyword(t *testing.T) {
	_, err := CompileStructuredOutputValidator(`{"type":7}`)
	require.ErrorContains(t, err, "compile structured output schema")
}
