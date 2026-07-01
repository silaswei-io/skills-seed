package jsonrepair

import (
	"os"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	if err := i18n.Init("zh-CN"); err != nil {
		_ = err
	}
	os.Exit(m.Run())
}

func TestExtractJSON_PlainJSON(t *testing.T) {
	input := `{"key": "value"}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key": "value"}`, result)
}

func TestExtractJSON_JSONInCodeBlock(t *testing.T) {
	input := "```json\n{\"key\": \"value\"}\n```"
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key": "value"}`, result)
}

func TestExtractJSON_EmptyOutput(t *testing.T) {
	result, err := ExtractJSON("")
	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestExtractJSON_WhitespaceOnly(t *testing.T) {
	result, err := ExtractJSON("   \n\t  ")
	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestExtractJSON_NoBraces(t *testing.T) {
	result, err := ExtractJSON("no json here")
	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestExtractJSON_RepairsTrailingCommaWithMissingClosingBrace(t *testing.T) {
	input := `{"key": "value",`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key": "value"}`, result)
}

func TestExtractJSON_InvalidJSON(t *testing.T) {
	input := `{not valid}`
	result, err := ExtractJSON(input)
	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestExtractJSON_NestedJSON(t *testing.T) {
	input := `{"outer": {"inner": 1}}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"outer": {"inner": 1}}`, result)
}

func TestExtractJSON_JSONWithTextBefore(t *testing.T) {
	input := `some text {"key": 1}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key": 1}`, result)
}

func TestExtractJSON_PrefersStructuredResultObjectAfterCodeLikePrefix(t *testing.T) {
	input := `分析完成：func Demo() { return nil } {"patterns":[{"id":"p1","name":"Pattern"}]}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"id":"p1","name":"Pattern"}]}`, result)
}

func TestExtractJSON_JSONWithTextAfter(t *testing.T) {
	input := `{"key": 1} some text`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key": 1}`, result)
}

func TestExtractJSON_EscapedStrings(t *testing.T) {
	input := `{"msg": "hello \"world\""}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"msg": "hello \"world\""}`, result)
}

func TestExtractJSON_RepairsInvalidBackslashEscapesInStrings(t *testing.T) {
	input := `{"good_example": "const path = \"src\ pages\"\nconst re = /\s+/"}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"good_example": "const path = \"src\\ pages\"\nconst re = /\\s+/"}`, result)
}

func TestExtractJSON_RepairsUnescapedQuotesInsideCodeStrings(t *testing.T) {
	input := `{"patterns":[{"good_example":"resp.Extra.Desc = fmt.Sprintf("%s", "admin")","bad_example":""}]}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"good_example":"resp.Extra.Desc = fmt.Sprintf(\"%s\", \"admin\")","bad_example":""}]}`, result)
}

func TestExtractJSON_RepairsPartiallyEscapedQuotesInsideCodeStrings(t *testing.T) {
	input := `{"patterns":[{"good_example":"resp.Extra.Desc = fmt.Sprintf(\"%s【%s】", resp.Extra.Object, req.Username)","bad_example":""}]}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"good_example":"resp.Extra.Desc = fmt.Sprintf(\"%s【%s】\", resp.Extra.Object, req.Username)","bad_example":""}]}`, result)
}

func TestFixAIJSON_FixesCommonAIJSONDefectsTogether(t *testing.T) {
	input := `{{"patterns":[{"good_example":"resp.Extra.Desc = fmt.Sprintf(\"%s【%s】", resp.Extra.Object, req.Username)","bad_example":"const path = \"src\ pages\""}]`
	result, err := FixAIJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"good_example":"resp.Extra.Desc = fmt.Sprintf(\"%s【%s】\", resp.Extra.Object, req.Username)","bad_example":"const path = \"src\\ pages\""}]}`, result)
}

func TestFixAIJSON_RepairsBareObjectKey(t *testing.T) {
	input := `{"evidence_locations":[{"path":"internal/logic/access_grant/config.go", line": 253, "symbol":"Config"}]}`
	result, err := FixAIJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"evidence_locations":[{"path":"internal/logic/access_grant/config.go","line":253,"symbol":"Config"}]}`, result)
}

func TestFixAIJSON_RepairsNumericLineRange(t *testing.T) {
	input := `{"patterns":[{"evidence_locations":[{"path":"desc/api/auth/auth.api","line":29-43,"symbol":"service cipher_machine"}]}]}`
	result, err := FixAIJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"evidence_locations":[{"path":"desc/api/auth/auth.api","line":29,"symbol":"service cipher_machine"}]}]}`, result)
}

func TestExtractJSON_RepairsNumericLineRangeAfterTextPrefix(t *testing.T) {
	input := `基于源码分析，输出如下。{"patterns":[{"evidence_locations":[{"path":"desc/api/auth/auth.api","line":29-43,"symbol":"service cipher_machine"}]}]}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"evidence_locations":[{"path":"desc/api/auth/auth.api","line":29,"symbol":"service cipher_machine"}]}]}`, result)
}

func TestFixAIJSON_RepairsRawNewlineInsideString(t *testing.T) {
	input := "{\"patterns\":[{\"good_example\":\"line1\nline2\"}]}"
	result, err := FixAIJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"good_example":"line1\nline2"}]}`, result)
}

func TestFixAIJSON_RepairsMissingObjectStartInArray(t *testing.T) {
	input := `{"profile_delta":{"layers":[{"name":"Handler层","files":["handler.go"]},{"name":"Logic层","files":["logic.go"]},"name":"数据访问层","description":"通过Model层访问数据库配置表","responsibilities":["条件查询"],"files":["model.go"]]}}`
	result, err := FixAIJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"profile_delta":{"layers":[{"name":"Handler层","files":["handler.go"]},{"name":"Logic层","files":["logic.go"]},{"name":"数据访问层","description":"通过Model层访问数据库配置表","responsibilities":["条件查询"],"files":["model.go"]}]}}`, result)
}

func TestFixAIJSON_RepairsTrailingCommas(t *testing.T) {
	input := `{"patterns":[{"id":"service","tags":["go","cli",],},],"summary":"ok",}`
	result, err := FixAIJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"id":"service","tags":["go","cli"]}],"summary":"ok"}`, result)
}

func TestFixAIJSON_RepairsLineAndBlockComments(t *testing.T) {
	input := `{
  // learned patterns
  "patterns": [
    {"id": "service"} /* primary pattern */
  ],
  "summary": "keep // inside string and /* inside string */"
}`
	result, err := FixAIJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"id":"service"}],"summary":"keep // inside string and /* inside string */"}`, result)
}

func TestFixAIJSON_RepairsSingleQuotedStrings(t *testing.T) {
	input := `{'patterns':[{'id':'service','good_example':'return fmt.Errorf("load: %w", err)'}]}`
	result, err := FixAIJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"id":"service","good_example":"return fmt.Errorf(\"load: %w\", err)"}]}`, result)
}

func TestFixAIJSON_RepairsPythonLiterals(t *testing.T) {
	input := `{"profile_refresh_recommended":{"needed": True, "reason": None}, "patterns": [{"id": "p", "active": False}]}`
	result, err := FixAIJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"profile_refresh_recommended":{"needed":true,"reason":null},"patterns":[{"id":"p","active":false}]}`, result)
}

func TestFixAIJSON_RepairsMissingCommaBetweenObjectFields(t *testing.T) {
	input := `{"patterns":[{"id":"service" "name":"Service" "confidence":0.91 "active":true "reason":null}]}`
	result, err := FixAIJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"id":"service","name":"Service","confidence":0.91,"active":true,"reason":null}]}`, result)
}

func TestFixAIJSON_RepairsMissingCommaBetweenArrayValues(t *testing.T) {
	input := `{"frameworks":["cobra" "bubbletea" {"name":"custom"} true null 3]}`
	result, err := FixAIJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"frameworks":["cobra","bubbletea",{"name":"custom"},true,null,3]}`, result)
}

func TestExtractJSON_RemovesExtraClosingContainerInsideArray(t *testing.T) {
	input := `{"patterns":[{"id":"a","evidence_locations":[{"path":"p","line":1}]}},{"id":"b"}],"profile_refresh_recommended":{"needed":false}}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"id":"a","evidence_locations":[{"path":"p","line":1}]},{"id":"b"}],"profile_refresh_recommended":{"needed":false}}`, result)
}

func TestExtractJSON_RepairsCommonNonstandardJSONInCodeBlock(t *testing.T) {
	input := "```json\n{\n  // comment\n  'patterns': [{'id': 'service',}],\n  'profile_refresh_recommended': {'needed': False, 'reason': None,},\n}\n```"
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"id":"service"}],"profile_refresh_recommended":{"needed":false,"reason":null}}`, result)
}

func TestExtractJSON_RepairsBareObjectKeyInCodeBlock(t *testing.T) {
	input := "```json\n{\"patterns\":[{\"evidence_locations\":[{\"path\":\"internal/logic/access_grant/config.go\", line\": 253, \"symbol\":\"Config\"}]}]}\n```"
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"evidence_locations":[{"path":"internal/logic/access_grant/config.go","line":253,"symbol":"Config"}]}]}`, result)
}

func TestExtractJSON_RepairsMissingClosingContainersAtEnd(t *testing.T) {
	input := `{"patterns":[{"id":"service","name":"Service"}],"category_summaries":{"structure":{"summary":"layers","patterns":["Service"]}}`
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"patterns":[{"id":"service","name":"Service"}],"category_summaries":{"structure":{"summary":"layers","patterns":["Service"]}}}`, result)
}

func TestExtractJSON_DoesNotRepairUnterminatedString(t *testing.T) {
	input := `{"patterns":[{"id":"service","name":"Service}]`
	result, err := ExtractJSON(input)
	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestExtractJSON_CodeBlockWithNestedJSON(t *testing.T) {
	input := "```json\n{\"outer\": {\"inner\": [1, 2, 3]}}\n```"
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"outer": {"inner": [1, 2, 3]}}`, result)
}

func TestExtractJSON_ArrayInCodeBlock(t *testing.T) {
	input := "```json\n[1, 2, 3]\n```"
	_, err := ExtractJSON(input)
	assert.Error(t, err)
}

func TestExtractJSON_MultipleCodeBlocks(t *testing.T) {
	input := "```\nsome code\n```\n```json\n{\"key\": 1}\n```"
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key": 1}`, result)
}

func TestExtractJSON_SkipsNonJSONCodeBlockBeforeJSON(t *testing.T) {
	input := "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n```json\n{\"key\": 1}\n```"
	result, err := ExtractJSON(input)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key": 1}`, result)
}

func TestFindMatchingBrace_Simple(t *testing.T) {
	input := `{"a":1}`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, 6, end, "should find matching brace at position 6")
}

func TestFindMatchingBrace_Nested(t *testing.T) {
	input := `{"a":{"b":2}}`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, 12, end, "should find matching brace at position 12")
}

func TestFindMatchingBrace_NoMatch(t *testing.T) {
	input := `{"a":1`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, -1, end, "should return -1 for unmatched brace")
}

func TestFindMatchingBrace_WithStrings(t *testing.T) {
	input := `{"a":"}{"}`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, 9, end, "braces inside strings should be ignored")
}

func TestFindMatchingBrace_EscapedQuoteInString(t *testing.T) {
	input := `{"a":"he said \"hello\""}`
	end := findMatchingBrace(input, 0)
	assert.NotEqual(t, -1, end, "should find matching brace with escaped quotes")
	extracted := input[0 : end+1]
	assert.JSONEq(t, `{"a":"he said \"hello\""}`, extracted)
}

func TestFindMatchingBrace_DeeplyNested(t *testing.T) {
	input := `{"a":{"b":{"c":1}}}`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, 18, end, "should find matching brace for deeply nested object")
}

func TestFindMatchingBrace_StartFromMiddle(t *testing.T) {
	input := `prefix {"inner": 1} suffix`
	start := 7
	end := findMatchingBrace(input, start)
	assert.Equal(t, 18, end, "should find matching brace when starting from middle")
}

func TestFindMatchingBrace_EmptyObject(t *testing.T) {
	input := `{}`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, 1, end, "should find matching brace for empty object")
}

func TestFindMatchingBrace_NoOpeningBrace(t *testing.T) {
	input := `no braces here`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, -1, end, "should return -1 when no opening brace at start position")
}

func TestFindMatchingBrace_WithArray(t *testing.T) {
	input := `{"arr": [1, 2, 3]}`
	end := findMatchingBrace(input, 0)
	assert.Equal(t, 17, end, "should correctly skip array brackets")
}

func TestFindMatchingBrace_EscapedBackslash(t *testing.T) {
	input := `{"path": "C:\\Users\\"}`
	end := findMatchingBrace(input, 0)
	assert.NotEqual(t, -1, end, "should handle escaped backslashes correctly")
}
