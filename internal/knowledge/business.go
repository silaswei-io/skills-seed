package knowledge

import (
	"strings"
	"unicode"
)

// SplitIdentifierWords 将代码标识符拆成适合展示的词。
func SplitIdentifierWords(text string) []string {
	var normalized strings.Builder
	var previous rune
	for _, r := range strings.TrimSpace(text) {
		switch {
		case unicode.IsUpper(r) && previous != 0 && (unicode.IsLower(previous) || unicode.IsDigit(previous)):
			normalized.WriteRune(' ')
			normalized.WriteRune(unicode.ToLower(r))
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			normalized.WriteRune(unicode.ToLower(r))
		default:
			normalized.WriteRune(' ')
		}
		previous = r
	}
	return strings.Fields(normalized.String())
}

// TitleFromWords 将拆分后的词转为展示标题。
func TitleFromWords(words []string) string {
	parts := make([]string, 0, len(words))
	for _, word := range words {
		if word == "" {
			continue
		}
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		parts = append(parts, string(runes))
	}
	return strings.Join(parts, " ")
}
