package text

import "strings"

func NormalizeStructureSummary(structure string) string {
	structure = strings.ReplaceAll(structure, "\u00a0", " ")
	structure = strings.ReplaceAll(structure, "&nbsp;", " ")
	structure = strings.ReplaceAll(structure, "\r\n", "\n")
	structure = strings.ReplaceAll(structure, "\r", "\n")

	lines := strings.Split(structure, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
