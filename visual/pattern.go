package visual

import "strings"

func RenderPatternRow(pattern string, width int) string {
	if pattern == "" { pattern = " " }
	runes := []rune(pattern)
	row := make([]rune, width)
	for i := range width { row[i] = runes[i%len(runes)] }
	return string(row)
}

func RenderPatternRows(pattern string, width, height, offset int) []string {
	if pattern == "" { pattern = " " }
	runes := []rune(pattern)
	rows := make([]string, height)
	for i := range height {
		shifted := make([]rune, width)
		rowOffset := offset + i
		for j := range width {
			idx := (j + rowOffset) % len(runes)
			shifted[j] = runes[idx]
		}
		rows[i] = string(shifted)
	}
	return rows
}

func RepeatToWidth(s string, width int) string {
	if s == "" { return strings.Repeat(" ", width) }
	runes := []rune(s)
	result := make([]rune, width)
	for i := range width { result[i] = runes[i%len(runes)] }
	return string(result)
}
