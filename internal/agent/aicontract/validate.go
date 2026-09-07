package aicontract

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// structuredOutputSchemaURL 是进程内编译动态输出 Schema 时使用的稳定资源标识。
const structuredOutputSchemaURL = "urn:skills-seed:structured-output"

// StructuredOutputValidator 校验 Agent 返回值是否满足实际传给 CLI 的动态 Schema。
type StructuredOutputValidator struct {
	schema *jsonschema.Schema
}

// CompileStructuredOutputValidator 编译一次动态 Schema，供同一调用的所有重试复用。
func CompileStructuredOutputValidator(schemaText string) (*StructuredOutputValidator, error) {
	schemaDocument, err := decodeJSONValue(schemaText)
	if err != nil {
		return nil, fmt.Errorf("decode structured output schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(structuredOutputSchemaURL, schemaDocument); err != nil {
		return nil, fmt.Errorf("register structured output schema: %w", err)
	}
	compiled, err := compiler.Compile(structuredOutputSchemaURL)
	if err != nil {
		return nil, fmt.Errorf("compile structured output schema: %w", err)
	}
	return &StructuredOutputValidator{schema: compiled}, nil
}

// Validate 拒绝语法无效、包含多个 JSON 值或不符合动态 Schema 的输出。
func (v *StructuredOutputValidator) Validate(output string) error {
	if v == nil || v.schema == nil {
		return fmt.Errorf("structured output validator is not configured")
	}
	value, err := decodeJSONValue(output)
	if err != nil {
		return fmt.Errorf("decode structured output: %w", err)
	}
	if err := v.schema.Validate(value); err != nil {
		return fmt.Errorf("validate structured output: %w", err)
	}
	return nil
}

func decodeJSONValue(text string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(text)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values")
		}
		return nil, err
	}
	return value, nil
}
