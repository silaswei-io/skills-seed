package jsonrepair

import (
	"fmt"
	"strings"
	"unicode"
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

func repairNumericRanges(jsonStr string) (string, error) {
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
		if inString || !isDigitByte(ch) {
			b.WriteByte(ch)
			continue
		}

		valueStart := i
		valueEnd := consumeDigits(jsonStr, i)
		if !numericRangeHasValuePrefix(jsonStr, valueStart) || valueEnd >= len(jsonStr) || jsonStr[valueEnd] != '-' {
			b.WriteString(jsonStr[valueStart:valueEnd])
			i = valueEnd - 1
			continue
		}
		nextStart := valueEnd + 1
		nextEnd := consumeDigits(jsonStr, nextStart)
		if nextEnd == nextStart || !numericRangeHasValueSuffix(jsonStr, nextEnd) {
			b.WriteString(jsonStr[valueStart:valueEnd])
			i = valueEnd - 1
			continue
		}

		b.WriteString(jsonStr[valueStart:valueEnd])
		i = nextEnd - 1
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

func consumeDigits(s string, index int) int {
	for index < len(s) && isDigitByte(s[index]) {
		index++
	}
	return index
}

func numericRangeHasValuePrefix(s string, index int) bool {
	for i := index - 1; i >= 0; i-- {
		if unicode.IsSpace(rune(s[i])) {
			continue
		}
		return s[i] == ':'
	}
	return false
}

func numericRangeHasValueSuffix(s string, index int) bool {
	for i := index; i < len(s); i++ {
		if unicode.IsSpace(rune(s[i])) {
			continue
		}
		switch s[i] {
		case ',', '}', ']':
			return true
		default:
			return false
		}
	}
	return true
}

func isDigitByte(ch byte) bool {
	return ch >= '0' && ch <= '9'
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

func repairExtraClosingContainers(jsonStr string) (string, error) {
	var b strings.Builder
	b.Grow(len(jsonStr))

	stack := make([]byte, 0, 8)
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
		if inString {
			b.WriteByte(ch)
			continue
		}

		switch ch {
		case '{', '[':
			stack = append(stack, ch)
			b.WriteByte(ch)
		case '}', ']':
			if len(stack) == 0 || !closingMatchesOpening(stack[len(stack)-1], ch) {
				repaired = true
				continue
			}
			stack = stack[:len(stack)-1]
			b.WriteByte(ch)
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

func closingMatchesOpening(opening, closing byte) bool {
	return (opening == '{' && closing == '}') || (opening == '[' && closing == ']')
}
