package ui

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) formatName() string {
	switch m.valFormat {
	case fmtText:
		return "text"
	case fmtHex:
		return "hex"
	case fmtBase64:
		return "base64"
	case fmtJSON:
		return "json"
	default:
		return "text"
	}
}

func (m Model) formatValue(_ string, v []byte) string {
	switch m.valFormat {
	case fmtText:
		if utf8.Valid(v) {
			return string(v)
		}
		return fmt.Sprintf("Warning: invalid UTF-8. Base64:\n%s", base64.StdEncoding.EncodeToString(v))
	case fmtHex:
		return hex.Dump(v)
	case fmtBase64:
		return base64.StdEncoding.EncodeToString(v)
	case fmtJSON:
		if !utf8.Valid(v) {
			return fmt.Sprintf("Warning: invalid UTF-8; cannot be JSON. Base64:\n%s", base64.StdEncoding.EncodeToString(v))
		}
		var any interface{}
		if err := json.Unmarshal(v, &any); err != nil {
			errLine := jsonErrorStyle.Render(fmt.Sprintf("Warning: invalid JSON: %v", err))
			return errLine + "\n\n" + colorizeJSON(string(v))
		}
		pretty, _ := json.MarshalIndent(any, "", "  ")
		return colorizeJSON(string(pretty))
	default:
		return string(v)
	}
}

func (m Model) reloadSelected() (tea.Model, tea.Cmd) {
	if m.selected == "" {
		if i, ok := m.list.SelectedItem().(kvItem); ok {
			m.selected = i.key
		} else {
			return m, nil
		}
	}
	// I clear editKey on format change.
	m.editKey = ""
	return m, loadValueCmd(m.store, m.selected)
}

// I keep edit helpers here.

func (m *Model) startEditWithContent(key string, raw []byte) {
	m.editKey = key
	m.editing = true
	m.lastLoadValue = raw
	m.editorHelp = "(Ctrl+S save · Esc cancel)"

	// I set formatted content in the editor.
	switch m.valFormat {
	case fmtText:
		if utf8.Valid(raw) {
			m.editor.SetValue(string(raw))
		} else {
			m.editor.SetValue("") // I leave it empty for binary data.
			m.status = errStyle.Render("Warning: invalid UTF-8; editing in text mode may be unsafe.")
		}
	case fmtBase64:
		m.editor.SetValue(base64.StdEncoding.EncodeToString(raw))
	case fmtHex:
		// I expect plain hex (not a dump).
		m.editor.SetValue(strings.ToLower(hex.EncodeToString(raw)))
	case fmtJSON:
		if !utf8.Valid(raw) {
			m.editor.SetValue("")
			m.status = errStyle.Render("Warning: invalid UTF-8; cannot be JSON.")
			break
		}
		var any interface{}
		if err := json.Unmarshal(raw, &any); err != nil {
			// I still show raw text so it can be fixed.
			m.editor.SetValue(string(raw))
			m.status = errStyle.Render(fmt.Sprintf("Warning: invalid JSON: %v (you can fix it)", err))
			break
		}
		pretty, _ := json.MarshalIndent(any, "", "  ")
		m.editor.SetValue(string(pretty))
	}
	m.editor.CursorEnd()
	m.status = "Editing. (Ctrl+S save · Esc cancel)"
}

func (m Model) bytesFromEditor() ([]byte, error) {
	content := m.editor.Value()
	switch m.valFormat {
	case fmtText:
		return []byte(content), nil
	case fmtBase64:
		b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(content))
		if err != nil {
			return nil, fmt.Errorf("invalid base64: %w", err)
		}
		return b, nil
	case fmtHex:
		// I strip whitespace, newlines, and 0x; I expect plain hex.
		clean := strings.ToLower(content)
		clean = strings.ReplaceAll(clean, "0x", "")
		re := regexp.MustCompile(`[^0-9a-f]`)
		clean = re.ReplaceAllString(clean, "")
		if len(clean)%2 != 0 {
			return nil, errors.New("hex length must be even")
		}
		b, err := hex.DecodeString(clean)
		if err != nil {
			return nil, fmt.Errorf("invalid hex: %w", err)
		}
		return b, nil
	case fmtJSON:
		// I validate JSON.
		if !utf8.ValidString(content) {
			return nil, errors.New("JSON must be UTF-8")
		}
		var any interface{}
		if err := json.Unmarshal([]byte(content), &any); err != nil {
			return nil, fmt.Errorf("JSON parse error: %w", err)
		}
		// Pretty-print for now; I can switch to raw if needed.
		pretty, _ := json.MarshalIndent(any, "", "  ")
		return pretty, nil
	default:
		return []byte(content), nil
	}
}

func jsonErrorInfo(err error, content string) string {
	var se *json.SyntaxError
	var te *json.UnmarshalTypeError
	switch {
	case errors.As(err, &se):
		line, col := offsetToLineCol(content, se.Offset)
		return fmt.Sprintf("JSON error at %d:%d: %s", line, col, se.Error())
	case errors.As(err, &te):
		line, col := offsetToLineCol(content, te.Offset)
		return fmt.Sprintf("JSON type error at %d:%d: %s", line, col, te.Error())
	default:
		return err.Error()
	}
}

func offsetToLineCol(s string, offset int64) (int, int) {
	if offset < 1 {
		return 1, 1
	}
	if offset > int64(len(s)) {
		offset = int64(len(s))
	}
	line := 1
	col := 1
	for i := 0; i < len(s) && int64(i)+1 < offset; i++ {
		if s[i] == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return line, col
}

func colorizeJSON(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		ch := s[i]
		switch {
		case ch == '"':
			start := i
			i++
			for i < len(s) {
				if s[i] == '\\' {
					if i+1 < len(s) {
						i += 2
						continue
					}
					i++
					break
				}
				if s[i] == '"' {
					i++
					break
				}
				i++
			}
			token := s[start:i]
			if isJSONKey(s, i) {
				b.WriteString(jsonKeyStyle.Render(token))
			} else {
				b.WriteString(jsonStringStyle.Render(token))
			}
			continue
		case ch == '{' || ch == '}' || ch == '[' || ch == ']' || ch == ':' || ch == ',':
			b.WriteString(jsonPunctStyle.Render(string(ch)))
			i++
			continue
		case ch == '-' || isDigit(ch):
			if ch == '-' && (i+1 >= len(s) || !isDigit(s[i+1])) {
				b.WriteByte(ch)
				i++
				continue
			}
			start := i
			i++
			for i < len(s) && isNumberChar(s[i]) {
				i++
			}
			b.WriteString(jsonNumberStyle.Render(s[start:i]))
			continue
		case hasKeyword(s, i, "true"):
			b.WriteString(jsonBoolStyle.Render("true"))
			i += 4
			continue
		case hasKeyword(s, i, "false"):
			b.WriteString(jsonBoolStyle.Render("false"))
			i += 5
			continue
		case hasKeyword(s, i, "null"):
			b.WriteString(jsonNullStyle.Render("null"))
			i += 4
			continue
		default:
			b.WriteByte(ch)
			i++
		}
	}
	return b.String()
}

func isJSONKey(s string, idx int) bool {
	for idx < len(s) {
		if isWhitespace(s[idx]) {
			idx++
			continue
		}
		return s[idx] == ':'
	}
	return false
}

func hasKeyword(s string, idx int, kw string) bool {
	if !strings.HasPrefix(s[idx:], kw) {
		return false
	}
	end := idx + len(kw)
	if end < len(s) && isIdentChar(s[end]) {
		return false
	}
	return true
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isNumberChar(b byte) bool {
	return isDigit(b) || b == '.' || b == 'e' || b == 'E' || b == '+' || b == '-'
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\t' || b == '\r'
}
