package jsonrepair

import (
	"encoding/json"
	"fmt"
	"strings"
)

type jsonRepairContext struct {
	kind  byte
	state jsonRepairState
}

type jsonArrayObjectRepairContext struct {
	kind      byte
	state     jsonRepairState
	synthetic bool
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
		repairJSONComments,
		repairDuplicatedObjectStarts,
		repairSingleQuotedStrings,
		repairPythonLiterals,
		repairRawControlCharactersInStrings,
		repairBareObjectKeys,
		repairMissingObjectStartsInArrays,
		repairMissingCommas,
		repairUnescapedQuotesInStrings,
		repairInvalidStringEscapes,
		repairTrailingCommas,
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

func validateJSON(jsonStr string) error {
	var js interface{}
	return json.Unmarshal([]byte(jsonStr), &js)
}

func repairDuplicatedObjectStarts(jsonStr string) (string, error) {
	var b strings.Builder
	b.Grow(len(jsonStr))

	inString := false
	escapeNext := false
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

		if ch == '"' {
			b.WriteByte(ch)
			inString = !inString
			continue
		}

		if !inString && ch == '{' && i+2 < len(jsonStr) && jsonStr[i+1] == '{' && jsonStr[i+2] == '"' {
			b.WriteByte(ch)
			repaired = true
			i++
			continue
		}
		if !inString && ch == '{' && i+3 < len(jsonStr) && jsonStr[i+1] == '"' && jsonStr[i+2] == '{' && jsonStr[i+3] == '"' {
			b.WriteByte(ch)
			repaired = true
			i += 2
			continue
		}

		b.WriteByte(ch)
	}
	if inString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	if !repaired {
		return jsonStr, nil
	}
	return b.String(), nil
}

func repairBareObjectKeys(jsonStr string) (string, error) {
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
		if ch == '"' {
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
			b.WriteByte(ch)
			continue
		}
		if inString {
			b.WriteByte(ch)
			continue
		}

		if canRepairBareKeyAt(stack, ch) {
			if key, afterKey, ok := parseBareObjectKey(jsonStr, i); ok {
				b.WriteByte('"')
				b.WriteString(key)
				b.WriteByte('"')
				b.WriteString(jsonStr[afterKey:iAfterColon(jsonStr, afterKey)])
				i = iAfterColon(jsonStr, afterKey) - 1
				setTopRepairState(stack, jsonRepairStateExpectValue)
				repaired = true
				continue
			}
		}

		b.WriteByte(ch)
		updateRepairContext(&stack, ch)
	}
	if inString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	if !repaired {
		return jsonStr, nil
	}
	return b.String(), nil
}

func repairMissingObjectStartsInArrays(jsonStr string) (string, error) {
	var b strings.Builder
	b.Grow(len(jsonStr))

	stack := make([]jsonArrayObjectRepairContext, 0, 8)
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
		if ch == '"' {
			if !inString {
				if canRepairArrayObjectStartAt(stack, jsonStr, i) {
					b.WriteByte('{')
					stack = append(stack, jsonArrayObjectRepairContext{kind: '{', state: jsonRepairStateExpectKeyOrEnd, synthetic: true})
					repaired = true
				}
				b.WriteByte(ch)
				inString = true
				stringRole = stringRoleForArrayObjectRepair(stack)
				continue
			}
			if quoteTerminatesJSONString(jsonStr, i, stringRole, arrayObjectRepairBaseStack(stack)) {
				b.WriteByte(ch)
				inString = false
				if stringRole == jsonRepairStringRoleKey {
					setTopArrayObjectRepairState(stack, jsonRepairStateExpectColon)
				} else {
					markArrayObjectRepairValueComplete(stack)
				}
				continue
			}
			b.WriteByte(ch)
			continue
		}
		if inString {
			b.WriteByte(ch)
			continue
		}

		if shouldCloseSyntheticArrayObjectBefore(stack, jsonStr, i) {
			b.WriteByte('}')
			stack = stack[:len(stack)-1]
			markArrayObjectRepairValueComplete(stack)
			repaired = true
		}

		b.WriteByte(ch)
		updateArrayObjectRepairContext(&stack, ch)
	}
	if inString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	if !repaired {
		return jsonStr, nil
	}
	return b.String(), nil
}

func canRepairArrayObjectStartAt(stack []jsonArrayObjectRepairContext, s string, quoteIndex int) bool {
	if len(stack) == 0 || !looksLikeObjectKeyAt(s, quoteIndex) {
		return false
	}
	top := stack[len(stack)-1]
	return top.kind == '[' && top.state == jsonRepairStateExpectValue
}

func stringRoleForArrayObjectRepair(stack []jsonArrayObjectRepairContext) jsonRepairStringRole {
	if len(stack) == 0 {
		return jsonRepairStringRoleValue
	}
	top := stack[len(stack)-1]
	if top.kind == '{' && top.state == jsonRepairStateExpectKeyOrEnd {
		return jsonRepairStringRoleKey
	}
	return jsonRepairStringRoleValue
}

func arrayObjectRepairBaseStack(stack []jsonArrayObjectRepairContext) []jsonRepairContext {
	base := make([]jsonRepairContext, 0, len(stack))
	for _, ctx := range stack {
		base = append(base, jsonRepairContext{kind: ctx.kind, state: ctx.state})
	}
	return base
}

func shouldCloseSyntheticArrayObjectBefore(stack []jsonArrayObjectRepairContext, s string, index int) bool {
	if len(stack) == 0 {
		return false
	}
	top := stack[len(stack)-1]
	if !top.synthetic || top.kind != '{' || top.state != jsonRepairStateExpectCommaOrEnd {
		return false
	}
	switch s[index] {
	case ']':
		return true
	case ',':
		next := nextNonWhitespaceIndex(s, index+1)
		if next >= len(s) {
			return true
		}
		return !looksLikeObjectKeyAt(s, next)
	default:
		return false
	}
}

func updateArrayObjectRepairContext(stack *[]jsonArrayObjectRepairContext, ch byte) {
	switch ch {
	case '{':
		*stack = append(*stack, jsonArrayObjectRepairContext{kind: '{', state: jsonRepairStateExpectKeyOrEnd})
	case '[':
		*stack = append(*stack, jsonArrayObjectRepairContext{kind: '[', state: jsonRepairStateExpectValue})
	case '}':
		if len(*stack) > 0 && (*stack)[len(*stack)-1].kind == '{' {
			*stack = (*stack)[:len(*stack)-1]
			markArrayObjectRepairValueComplete(*stack)
		}
	case ']':
		if len(*stack) > 0 && (*stack)[len(*stack)-1].kind == '[' {
			*stack = (*stack)[:len(*stack)-1]
			markArrayObjectRepairValueComplete(*stack)
		}
	case ':':
		setTopArrayObjectRepairState(*stack, jsonRepairStateExpectValue)
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

func setTopArrayObjectRepairState(stack []jsonArrayObjectRepairContext, state jsonRepairState) {
	if len(stack) == 0 {
		return
	}
	stack[len(stack)-1].state = state
}

func markArrayObjectRepairValueComplete(stack []jsonArrayObjectRepairContext) {
	if len(stack) == 0 {
		return
	}
	stack[len(stack)-1].state = jsonRepairStateExpectCommaOrEnd
}

func canRepairBareKeyAt(stack []jsonRepairContext, ch byte) bool {
	if len(stack) == 0 || !isBareObjectKeyStart(ch) {
		return false
	}
	top := stack[len(stack)-1]
	return top.kind == '{' && top.state == jsonRepairStateExpectKeyOrEnd
}

func parseBareObjectKey(s string, start int) (string, int, bool) {
	if start >= len(s) || !isBareObjectKeyStart(s[start]) {
		return "", start, false
	}
	end := start + 1
	for end < len(s) && isBareObjectKeyPart(s[end]) {
		end++
	}
	afterKey := end
	if afterKey < len(s) && s[afterKey] == '"' {
		afterKey++
	}
	colon := nextNonWhitespaceIndex(s, afterKey)
	if colon >= len(s) || s[colon] != ':' {
		return "", start, false
	}
	return s[start:end], afterKey, true
}

func iAfterColon(s string, start int) int {
	colon := nextNonWhitespaceIndex(s, start)
	if colon >= len(s) {
		return start
	}
	return colon + 1
}

func isBareObjectKeyStart(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_' || ch == '$'
}

func isBareObjectKeyPart(ch byte) bool {
	return isBareObjectKeyStart(ch) || (ch >= '0' && ch <= '9') || ch == '-'
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

func repairRawControlCharactersInStrings(jsonStr string) (string, error) {
	var b strings.Builder
	b.Grow(len(jsonStr))

	inString := false
	escapeNext := false
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
		if ch == '"' {
			b.WriteByte(ch)
			inString = !inString
			continue
		}
		if inString && ch < 0x20 {
			writeEscapedControlCharacter(&b, ch)
			repaired = true
			continue
		}
		b.WriteByte(ch)
	}
	if inString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	if !repaired {
		return jsonStr, nil
	}
	return b.String(), nil
}

func repairTrailingCommas(jsonStr string) (string, error) {
	var b strings.Builder
	b.Grow(len(jsonStr))

	inString := false
	escapeNext := false
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
		if ch == '"' {
			b.WriteByte(ch)
			inString = !inString
			continue
		}
		if !inString && ch == ',' {
			next := nextNonWhitespaceIndex(jsonStr, i+1)
			if next < len(jsonStr) && (jsonStr[next] == '}' || jsonStr[next] == ']') {
				repaired = true
				continue
			}
		}
		b.WriteByte(ch)
	}
	if inString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	if !repaired {
		return jsonStr, nil
	}
	return b.String(), nil
}

func writeEscapedControlCharacter(b *strings.Builder, ch byte) {
	switch ch {
	case '\n':
		b.WriteString(`\n`)
	case '\r':
		b.WriteString(`\r`)
	case '\t':
		b.WriteString(`\t`)
	default:
		b.WriteString(fmt.Sprintf(`\u%04x`, ch))
	}
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
		return missingCommaAfterStringValueLooksStructural(s, next, parentKind)
	}
}

func commaAfterStringValueLooksStructural(s string, commaIndex int, parentKind byte) bool {
	next := nextNonWhitespaceIndex(s, commaIndex+1)
	if next >= len(s) {
		return false
	}
	if parentKind == '{' {
		if s[next] == '"' {
			return looksLikeObjectKeyAt(s, next)
		}
		_, _, ok := parseBareObjectKey(s, next)
		return ok
	}
	if parentKind == '[' {
		return looksLikeArrayValueAt(s, next)
	}
	return false
}

func missingCommaAfterStringValueLooksStructural(s string, next int, parentKind byte) bool {
	if parentKind == '{' {
		return looksLikeObjectKeyAt(s, next)
	}
	if parentKind == '[' {
		return looksLikeArrayValueAt(s, next)
	}
	return false
}

// looksLikeArrayValueAt checks whether the position starts a complete JSON value
// inside an array. It is stricter than looksLikeJSONValueStart because the
// surrounding context already guarantees a string just ended, and we need to
// verify the comma is a structural separator rather than a character inside
// that string.
func looksLikeArrayValueAt(s string, index int) bool {
	if index >= len(s) {
		return false
	}
	ch := s[index]
	switch ch {
	case '{', '[':
		// Container start is always structural.
		return true
	case '"':
		// A quoted string value – verify it terminates before the next comma
		// or closing bracket.
		return looksLikeQuotedValueAt(s, index)
	case 't':
		return matchLiteral(s, index, "true")
	case 'f':
		return matchLiteral(s, index, "false")
	case 'n':
		return matchLiteral(s, index, "null")
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return looksLikeNumberAt(s, index)
	default:
		return false
	}
}

// looksLikeQuotedValueAt verifies that a double-quoted string starting at index
// is followed by a comma, closing bracket, or end-of-input – consistent with
// the comma being structural rather than inside a preceding string.
func looksLikeQuotedValueAt(s string, index int) bool {
	inEscape := false
	for i := index + 1; i < len(s); i++ {
		ch := s[i]
		if inEscape {
			inEscape = false
			continue
		}
		if ch == '\\' {
			inEscape = true
			continue
		}
		if ch == '"' {
			next := nextNonWhitespaceIndex(s, i+1)
			if next >= len(s) {
				return true
			}
			return s[next] == ',' || s[next] == ']'
		}
	}
	return false
}

// matchLiteral checks whether s starting at index begins with the given literal
// followed by a structural character (comma, bracket, whitespace, or EOF).
func matchLiteral(s string, index int, literal string) bool {
	if index+len(literal) > len(s) {
		return false
	}
	if s[index:index+len(literal)] != literal {
		return false
	}
	next := index + len(literal)
	if next >= len(s) {
		return true
	}
	ch := s[next]
	return isJSONWhitespace(ch) || ch == ',' || ch == ']' || ch == '}'
}

// looksLikeNumberAt checks whether a numeric literal starts at index and is
// followed by a structural separator.
func looksLikeNumberAt(s string, index int) bool {
	j := index
	if j < len(s) && s[j] == '-' {
		j++
	}
	digitSeen := false
	for j < len(s) && (s[j] >= '0' && s[j] <= '9') {
		digitSeen = true
		j++
	}
	if !digitSeen {
		return false
	}
	// decimal part
	if j < len(s) && s[j] == '.' {
		j++
		for j < len(s) && (s[j] >= '0' && s[j] <= '9') {
			j++
		}
	}
	// exponent part
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		j++
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		for j < len(s) && (s[j] >= '0' && s[j] <= '9') {
			j++
		}
	}
	if j >= len(s) {
		return true
	}
	ch := s[j]
	return isJSONWhitespace(ch) || ch == ',' || ch == ']' || ch == '}'
}

func repairMissingClosingContainers(jsonStr string) (string, error) {
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" || jsonStr[0] != '{' {
		return "", fmt.Errorf("JSON object start not found")
	}

	stack := make([]byte, 0, 8)
	inString := false
	escapeNext := false
	for i := 0; i < len(jsonStr); i++ {
		ch := jsonStr[i]
		if escapeNext {
			escapeNext = false
			continue
		}
		if ch == '\\' {
			if inString {
				escapeNext = true
			}
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) == 0 || stack[len(stack)-1] != ch {
				return "", fmt.Errorf("JSON containers are mismatched")
			}
			stack = stack[:len(stack)-1]
		}
	}
	if inString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	if len(stack) == 0 {
		return jsonStr, nil
	}

	var b strings.Builder
	b.Grow(len(jsonStr) + len(stack))
	b.WriteString(jsonStr)
	for i := len(stack) - 1; i >= 0; i-- {
		b.WriteByte(stack[i])
	}
	return b.String(), nil
}

func repairInvalidStringEscapes(jsonStr string) (string, error) {
	var b strings.Builder
	b.Grow(len(jsonStr))

	inString := false
	for i := 0; i < len(jsonStr); i++ {
		ch := jsonStr[i]
		if !inString {
			b.WriteByte(ch)
			if ch == '"' {
				inString = true
			}
			continue
		}

		switch ch {
		case '"':
			b.WriteByte(ch)
			inString = false
		case '\\':
			if i+1 >= len(jsonStr) {
				b.WriteString(`\\`)
				continue
			}
			next := jsonStr[i+1]
			if isValidJSONEscape(next) {
				b.WriteByte(ch)
				b.WriteByte(next)
				i++
				if next == 'u' {
					for j := 0; j < 4 && i+1 < len(jsonStr); j++ {
						i++
						b.WriteByte(jsonStr[i])
					}
				}
				continue
			}
			b.WriteString(`\\`)
		default:
			b.WriteByte(ch)
		}
	}
	if inString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	return b.String(), nil
}

func isValidJSONEscape(ch byte) bool {
	switch ch {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't', 'u':
		return true
	default:
		return false
	}
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
