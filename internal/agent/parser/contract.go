package parser

import (
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
)

// ContractValidator 组合动态 JSON Schema 与业务 parser，定义结构化输出的唯一验收边界。
type ContractValidator struct {
	contract string
	opts     aicontract.StructuredOutputOptions
	schema   *aicontract.StructuredOutputValidator
}

// CompileContractValidator 编译一次动态 Schema，供同一 provider 调用的所有重试复用。
func CompileContractValidator(contract, schemaText string, opts aicontract.StructuredOutputOptions) (*ContractValidator, error) {
	schema, err := aicontract.CompileStructuredOutputValidator(schemaText)
	if err != nil {
		return nil, err
	}
	return &ContractValidator{contract: contract, opts: opts, schema: schema}, nil
}

// Validate 依次执行 JSON Schema 与业务语义校验。
func (v *ContractValidator) Validate(output string) error {
	if v == nil || v.schema == nil {
		return fmt.Errorf("structured output contract validator is not configured")
	}
	if err := v.schema.Validate(output); err != nil {
		return err
	}
	return ValidateContractResult(v.contract, v.opts, output)
}

// ValidateContractResult 复用业务 parser 校验一次结构化输出，确保 provider
// 只把下游确定能够解析的结果交出重试边界。
func ValidateContractResult(contract string, opts aicontract.StructuredOutputOptions, output string) error {
	switch contract {
	case aicontract.ContractUserDefinePattern:
		_, err := ParseUserDefinePatternResult(output)
		return err
	case aicontract.ContractProjectProfile:
		_, err := ParseAnalyzeProjectResult(output)
		return err
	case aicontract.ContractAuthorityExtraction:
		_, err := ParseExtractAuthorityResult(output)
		return err
	case aicontract.ContractAnalyzeCurrentCodebaseBatch:
		_, err := ParseAnalyzeCurrentCodebaseBatchResult(output)
		return err
	case aicontract.ContractAnalyzeCurrentDeltaBatch:
		_, err := ParseAnalyzeCurrentDeltaBatchResult(output)
		return err
	case aicontract.ContractPlanLearningAgenda:
		_, err := ParsePlanLearningAgendaResult(output)
		return err
	case aicontract.ContractReviewKnowledge:
		result, err := ParseReviewKnowledgeResult(output)
		if err != nil {
			return err
		}
		if opts.CandidateIDs != nil {
			return agent.ValidateKnowledgeReviewCandidates(result.Decisions, opts.CandidateIDs)
		}
		return nil
	case aicontract.ContractNormalizePatterns:
		_, err := ParseNormalizePatternsResult(output)
		return err
	case aicontract.ContractWorkspaceProfile:
		_, err := ParseWorkspaceProfile(output)
		return err
	case aicontract.ContractWorkspaceSpec:
		_, err := ParseWorkspaceSpec(output)
		return err
	case aicontract.ContractOptimizeWorkflow, aicontract.ContractOptimizeRule:
		_, err := ParseOptimizeContentResult(output)
		return err
	default:
		return fmt.Errorf("unknown AI output contract %q", contract)
	}
}
