package parser

import (
	"fmt"
	"strings"
)

type jsonRepairContext struct {
	kind  byte
	state jsonRepairState
}

type jsonRepairState int

const (
	// jsonRepairStateExpectKeyOrEnd 表示对象中正在等待字段名或结束符。
	jsonRepairStateExpectKeyOrEnd jsonRepairState = iota
	// jsonRepairStateExpectColon 表示对象字段名已结束，正在等待冒号。
	jsonRepairStateExpectColon
	// jsonRepairStateExpectValue 表示对象字段或数组元素正在等待值。
	jsonRepairStateExpectValue
	// jsonRepairStateExpectCommaOrEnd 表示一个值已结束，正在等待逗号或容器结束符。
	jsonRepairStateExpectCommaOrEnd
)

type jsonRepairStringRole int

const (
	// jsonRepairStringRoleValue 表示当前字符串是 JSON 值。
	jsonRepairStringRoleValue jsonRepairStringRole = iota
	// jsonRepairStringRoleKey 表示当前字符串是对象字段名。
	jsonRepairStringRoleKey
)

// FixAIJSON 修复 AI 常见的非标准 JSON 输出，并返回可被 encoding/json 解析的 JSON。
func FixAIJSON(jsonStr string) (string, error) {
	current := strings.TrimSpace(jsonStr)
	if current == "" {
		return "", fmt.Errorf("empty JSON")
	}
	if err := validateJSON(current); err == nil {
		return current, nil
	}

	var lastErr error
	repairs := []func(string) (string, error){
		repairDuplicatedObjectStarts,
		repairUnescapedQuotesInStrings,
		repairInvalidStringEscapes,
		repairMissingClosingContainers,
	}
	for round := 0; round < 3; round++ {
		changed := false
		for _, repair := range repairs {
			next, err := repair(current)
			if err != nil {
				lastErr = err
				continue
			}
			next = strings.TrimSpace(next)
			if next == "" || next == current {
				continue
			}
			changed = true
			current = next
			if err := validateJSON(current); err == nil {
				return current, nil
			} else {
				lastErr = err
			}
		}
		if !changed {
			break
		}
	}
	if err := validateJSON(current); err != nil {
		return "", err
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("unable to repair AI JSON")
}

func repairUnescapedQuotesInStrings(jsonStr string) (string, error) {
	var b strings.Builder
	b.Grow(len(jsonStr))

	stack := make([]jsonRepairContext, 0, 8)
	inString := false
	escapeNext := false
	stringRole := jsonRepairStringRoleValue
	repaired := false
	for i := 0; i < len(jsonStr); i++ {
		ch := jsonStr[i]
		if escapeNext {
			b.WriteByte(ch)
			escapeNext = false
			continue
		}
		if ch == '\\' {
			b.WriteByte(ch)
			if inString {
				escapeNext = true
			}
			continue
		}
		if ch != '"' {
			b.WriteByte(ch)
			if !inString {
				updateRepairContext(&stack, ch)
			}
			continue
		}
		if !inString {
			b.WriteByte(ch)
			inString = true
			stringRole = stringRoleForContext(stack)
			continue
		}
		if quoteTerminatesJSONString(jsonStr, i, stringRole, stack) {
			b.WriteByte(ch)
			inString = false
			if stringRole == jsonRepairStringRoleKey {
				setTopRepairState(stack, jsonRepairStateExpectColon)
			} else {
				markRepairValueComplete(stack)
			}
			continue
		}
		b.WriteString(`\"`)
		repaired = true
	}
	if inString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	if !repaired {
		return jsonStr, nil
	}
	return b.String(), nil
}

func updateRepairContext(stack *[]jsonRepairContext, ch byte) {
	switch ch {
	case '{':
		*stack = append(*stack, jsonRepairContext{kind: '{', state: jsonRepairStateExpectKeyOrEnd})
	case '[':
		*stack = append(*stack, jsonRepairContext{kind: '[', state: jsonRepairStateExpectValue})
	case '}':
		if len(*stack) > 0 && (*stack)[len(*stack)-1].kind == '{' {
			*stack = (*stack)[:len(*stack)-1]
			markRepairValueComplete(*stack)
		}
	case ']':
		if len(*stack) > 0 && (*stack)[len(*stack)-1].kind == '[' {
			*stack = (*stack)[:len(*stack)-1]
			markRepairValueComplete(*stack)
		}
	case ':':
		setTopRepairState(*stack, jsonRepairStateExpectValue)
	case ',':
		if len(*stack) == 0 {
			return
		}
		top := &(*stack)[len(*stack)-1]
		if top.kind == '{' {
			top.state = jsonRepairStateExpectKeyOrEnd
		} else if top.kind == '[' {
			top.state = jsonRepairStateExpectValue
		}
	default:
		if len(*stack) == 0 || isJSONWhitespace(ch) {
			return
		}
		top := &(*stack)[len(*stack)-1]
		if top.state == jsonRepairStateExpectValue {
			top.state = jsonRepairStateExpectCommaOrEnd
		}
	}
}

func stringRoleForContext(stack []jsonRepairContext) jsonRepairStringRole {
	if len(stack) == 0 {
		return jsonRepairStringRoleValue
	}
	top := stack[len(stack)-1]
	if top.kind == '{' && top.state == jsonRepairStateExpectKeyOrEnd {
		return jsonRepairStringRoleKey
	}
	return jsonRepairStringRoleValue
}

func setTopRepairState(stack []jsonRepairContext, state jsonRepairState) {
	if len(stack) == 0 {
		return
	}
	stack[len(stack)-1].state = state
}

func markRepairValueComplete(stack []jsonRepairContext) {
	if len(stack) == 0 {
		return
	}
	stack[len(stack)-1].state = jsonRepairStateExpectCommaOrEnd
}

func quoteTerminatesJSONString(s string, quoteIndex int, stringRole jsonRepairStringRole, stack []jsonRepairContext) bool {
	next := nextNonWhitespaceIndex(s, quoteIndex+1)
	if stringRole == jsonRepairStringRoleKey {
		return next < len(s) && s[next] == ':'
	}
	if next >= len(s) {
		return true
	}
	parentKind := byte(0)
	if len(stack) > 0 {
		parentKind = stack[len(stack)-1].kind
	}
	switch s[next] {
	case ',':
		return commaAfterStringValueLooksStructural(s, next, parentKind)
	case '}':
		return parentKind == '{' || parentKind == 0
	case ']':
		return parentKind == '[' || parentKind == 0
	default:
		return false
	}
}

func commaAfterStringValueLooksStructural(s string, commaIndex int, parentKind byte) bool {
	next := nextNonWhitespaceIndex(s, commaIndex+1)
	if next >= len(s) {
		return false
	}
	if parentKind == '{' {
		return s[next] == '"' && looksLikeObjectKeyAt(s, next)
	}
	if parentKind == '[' {
		return looksLikeJSONValueStart(s[next])
	}
	return false
}

func looksLikeObjectKeyAt(s string, quoteIndex int) bool {
	inEscape := false
	for i := quoteIndex + 1; i < len(s); i++ {
		ch := s[i]
		if inEscape {
			inEscape = false
			continue
		}
		if ch == '\\' {
			inEscape = true
			continue
		}
		if ch != '"' {
			continue
		}
		next := nextNonWhitespaceIndex(s, i+1)
		return next < len(s) && s[next] == ':'
	}
	return false
}

func looksLikeJSONValueStart(ch byte) bool {
	switch ch {
	case '{', '[', '"', 't', 'f', 'n', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	default:
		return false
	}
}

func nextNonWhitespaceIndex(s string, start int) int {
	for i := start; i < len(s); i++ {
		if !isJSONWhitespace(s[i]) {
			return i
		}
	}
	return len(s)
}

func isJSONWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\n' || ch == '\r' || ch == '\t'
}
