package jsonrepair

import (
	"fmt"
	"strings"
)

func repairJSONComments(jsonStr string) (string, error) {
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
		if inString || ch != '/' || i+1 >= len(jsonStr) {
			b.WriteByte(ch)
			continue
		}

		switch jsonStr[i+1] {
		case '/':
			repaired = true
			i += 2
			for i < len(jsonStr) && jsonStr[i] != '\n' && jsonStr[i] != '\r' {
				i++
			}
			if i < len(jsonStr) {
				b.WriteByte(jsonStr[i])
			}
		case '*':
			repaired = true
			i += 2
			for i+1 < len(jsonStr) && !(jsonStr[i] == '*' && jsonStr[i+1] == '/') {
				i++
			}
			if i+1 < len(jsonStr) {
				i++
			}
		default:
			b.WriteByte(ch)
		}
	}
	if inString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	if !repaired {
		return jsonStr, nil
	}
	return b.String(), nil
}

func repairSingleQuotedStrings(jsonStr string) (string, error) {
	var b strings.Builder
	b.Grow(len(jsonStr))

	inDoubleString := false
	inSingleString := false
	escapeNext := false
	repaired := false

	for i := 0; i < len(jsonStr); i++ {
		ch := jsonStr[i]
		if escapeNext {
			writeSingleQuotedStringEscapedByte(&b, ch, inSingleString)
			escapeNext = false
			continue
		}
		if ch == '\\' {
			if inSingleString {
				escapeNext = true
				continue
			}
			b.WriteByte(ch)
			if inDoubleString {
				escapeNext = true
			}
			continue
		}
		if inSingleString {
			switch ch {
			case '\'':
				b.WriteByte('"')
				inSingleString = false
			case '"':
				b.WriteString(`\"`)
			case '\n', '\r', '\t':
				writeEscapedControlCharacter(&b, ch)
			default:
				if ch < 0x20 {
					writeEscapedControlCharacter(&b, ch)
				} else {
					b.WriteByte(ch)
				}
			}
			continue
		}
		if ch == '"' {
			b.WriteByte(ch)
			inDoubleString = !inDoubleString
			continue
		}
		if !inDoubleString && ch == '\'' {
			b.WriteByte('"')
			inSingleString = true
			repaired = true
			continue
		}
		b.WriteByte(ch)
	}
	if inDoubleString || inSingleString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	if !repaired {
		return jsonStr, nil
	}
	return b.String(), nil
}

func writeSingleQuotedStringEscapedByte(b *strings.Builder, ch byte, inSingleString bool) {
	if !inSingleString {
		b.WriteByte(ch)
		return
	}
	switch ch {
	case '\'':
		b.WriteByte('\'')
	case '"':
		b.WriteString(`\"`)
	case '\\':
		b.WriteString(`\\`)
	case 'n', 'r', 't', 'b', 'f', 'u':
		b.WriteByte('\\')
		b.WriteByte(ch)
	default:
		b.WriteByte(ch)
	}
}

func repairPythonLiterals(jsonStr string) (string, error) {
	replacements := map[string]string{
		"True":  "true",
		"False": "false",
		"None":  "null",
	}

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
		if inString || !isBareObjectKeyStart(ch) {
			b.WriteByte(ch)
			continue
		}

		matched := false
		for from, to := range replacements {
			if matchIdentifierToken(jsonStr, i, from) {
				b.WriteString(to)
				i += len(from) - 1
				repaired = true
				matched = true
				break
			}
		}
		if !matched {
			b.WriteByte(ch)
		}
	}
	if inString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	if !repaired {
		return jsonStr, nil
	}
	return b.String(), nil
}

func matchIdentifierToken(s string, index int, token string) bool {
	if index+len(token) > len(s) || s[index:index+len(token)] != token {
		return false
	}
	if index > 0 && isBareObjectKeyPart(s[index-1]) {
		return false
	}
	next := index + len(token)
	return next >= len(s) || !isBareObjectKeyPart(s[next])
}

func repairMissingCommas(jsonStr string) (string, error) {
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
				if shouldInsertCommaBeforeQuotedToken(stack, jsonStr, i) {
					b.WriteByte(',')
					setTopRepairState(stack, stateAfterInsertedComma(stack))
					repaired = true
				}
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

		if shouldInsertCommaBeforeNonStringValue(stack, jsonStr, i) {
			b.WriteByte(',')
			setTopRepairState(stack, jsonRepairStateExpectValue)
			repaired = true
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

func shouldInsertCommaBeforeQuotedToken(stack []jsonRepairContext, s string, index int) bool {
	if len(stack) == 0 {
		return false
	}
	top := stack[len(stack)-1]
	if top.state != jsonRepairStateExpectCommaOrEnd {
		return false
	}
	switch top.kind {
	case '{':
		return looksLikeObjectKeyAt(s, index)
	case '[':
		return looksLikeQuotedValueAt(s, index)
	default:
		return false
	}
}

func shouldInsertCommaBeforeNonStringValue(stack []jsonRepairContext, s string, index int) bool {
	if len(stack) == 0 || isJSONWhitespace(s[index]) {
		return false
	}
	top := stack[len(stack)-1]
	return top.kind == '[' &&
		top.state == jsonRepairStateExpectCommaOrEnd &&
		s[index] != ']' &&
		s[index] != ',' &&
		looksLikeArrayValueAt(s, index)
}

func stateAfterInsertedComma(stack []jsonRepairContext) jsonRepairState {
	if len(stack) == 0 || stack[len(stack)-1].kind == '[' {
		return jsonRepairStateExpectValue
	}
	return jsonRepairStateExpectKeyOrEnd
}
