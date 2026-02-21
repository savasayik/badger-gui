package ui

import (
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

const cursorStartMarker = "\x01"
const cursorEndMarker = "\x02"

func (m Model) renderJSONEditor(lay layout) string {
	value := m.editor.Value()
	lines := strings.Split(value, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	textWidth := m.editor.Width()
	if textWidth < 1 {
		textWidth = lay.rightContentWidth
	}
	lnWidth := lay.rightContentWidth - textWidth
	showNumbers := m.editor.ShowLineNumbers && lnWidth > 0

	cursorLine := m.editor.Line()
	if cursorLine < 0 {
		cursorLine = 0
	}
	if cursorLine >= len(lines) {
		cursorLine = len(lines) - 1
	}
	lineInfo := m.editor.LineInfo()
	cursorRowOffset := lineInfo.RowOffset
	cursorColOffset := lineInfo.ColumnOffset

	var visualLines []visualLine
	cursorVisRow := 0
	cursorColIndex := 0

	for i, line := range lines {
		runes := []rune(line)
		segments := wrapRunes(runes, textWidth)
		if len(segments) == 0 {
			segments = [][]rune{{}}
		}
		if i == cursorLine && cursorRowOffset >= len(segments) {
			cursorRowOffset = len(segments) - 1
			if cursorRowOffset < 0 {
				cursorRowOffset = 0
			}
		}
		for si, seg := range segments {
			lineNo := 0
			if showNumbers && si == 0 {
				lineNo = i + 1
			}
			visualLines = append(visualLines, visualLine{text: string(seg), lineNo: lineNo})

			if i == cursorLine && si == cursorRowOffset {
				cursorVisRow = len(visualLines) - 1
				cursorColIndex = cursorColOffset
			}
		}
	}

	visibleHeight := m.editorHeight
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	scroll := 0
	if cursorVisRow >= visibleHeight {
		scroll = cursorVisRow - visibleHeight + 1
	}
	if scroll < 0 {
		scroll = 0
	}
	if len(visualLines) > 0 && scroll > len(visualLines)-visibleHeight {
		scroll = max(0, len(visualLines)-visibleHeight)
	}

	start := scroll
	end := min(start+visibleHeight, len(visualLines))
	if end < start {
		end = start
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		v := visualLines[i]
		var colored string
		if i == cursorVisRow {
			colored = colorizeJSONWithCursor(v.text, cursorColIndex)
		} else {
			colored = colorizeJSON(v.text)
		}

		colored = padAnsi(colored, textWidth)

		if showNumbers {
			prefix := formatLineNumberFixed(v.lineNo, lnWidth)
			b.WriteString(editorLineNumberStyle.Render(prefix))
		}

		b.WriteString(colored)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}

	return b.String()
}

func wrapRunes(runes []rune, width int) [][]rune {
	if width <= 0 {
		return [][]rune{runes}
	}
	if len(runes) == 0 {
		return [][]rune{{}}
	}

	lines := [][]rune{{}}
	var (
		word   []rune
		row    int
		spaces int
	)

	for _, r := range runes {
		if unicode.IsSpace(r) {
			spaces++
		} else {
			word = append(word, r)
		}

		if spaces > 0 {
			if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces > width {
				row++
				lines = append(lines, []rune{})
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], repeatSpaces(spaces)...)
				spaces = 0
				word = nil
			} else {
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], repeatSpaces(spaces)...)
				spaces = 0
				word = nil
			}
		} else {
			lastCharLen := runewidth.RuneWidth(word[len(word)-1])
			if uniseg.StringWidth(string(word))+lastCharLen > width {
				if len(lines[row]) > 0 {
					row++
					lines = append(lines, []rune{})
				}
				lines[row] = append(lines[row], word...)
				word = nil
			}
		}
	}

	if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces >= width {
		lines = append(lines, []rune{})
		lines[row+1] = append(lines[row+1], word...)
		spaces++
		lines[row+1] = append(lines[row+1], repeatSpaces(spaces)...)
	} else {
		lines[row] = append(lines[row], word...)
		spaces++
		lines[row] = append(lines[row], repeatSpaces(spaces)...)
	}

	return lines
}

func repeatSpaces(n int) []rune {
	return []rune(strings.Repeat(string(' '), n))
}

func colorizeJSONWithCursor(raw string, cursorIdx int) string {
	runes := []rune(raw)
	if cursorIdx < 0 {
		cursorIdx = 0
	}
	if cursorIdx > len(runes) {
		cursorIdx = len(runes)
	}
	if len(runes) == 0 || cursorIdx == len(runes) {
		runes = append(runes, ' ')
	}
	if cursorIdx >= len(runes) {
		cursorIdx = len(runes) - 1
	}

	marked := string(runes[:cursorIdx]) + cursorStartMarker + string(runes[cursorIdx]) + cursorEndMarker + string(runes[cursorIdx+1:])
	colored := colorizeJSON(marked)
	return applyCursorMarker(colored)
}

func applyCursorMarker(s string) string {
	start := strings.Index(s, cursorStartMarker)
	if start == -1 {
		return s
	}
	rest := s[start+len(cursorStartMarker):]
	end := strings.Index(rest, cursorEndMarker)
	if end == -1 {
		return strings.NewReplacer(cursorStartMarker, "", cursorEndMarker, "").Replace(s)
	}
	end += start + len(cursorStartMarker)

	mid := s[start+len(cursorStartMarker) : end]
	const underlineOn = "\x1b[4m"
	const underlineOff = "\x1b[24m"
	return s[:start] + underlineOn + mid + underlineOff + s[end+len(cursorEndMarker):]
}
