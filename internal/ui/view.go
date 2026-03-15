package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) View() string {
	lay := computeLayout(m.width, m.height)

	header := headerBarStyle.Render(padToWidth(joinLeftRight(m.appHeaderLeft(), m.appHeaderRight(), lay.innerWidth), lay.innerWidth))

	leftHeader := panelHeaderStyle.Render(padToWidth(m.listHeaderText(), lay.listWidth))
	left := paneStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			leftHeader,
			m.list.View(),
		),
	)

	var rightTitle string
	if m.editing {
		rightTitle = fmt.Sprintf("Edit: %s  (Ctrl+S save · Esc cancel)", m.editKey)
	} else {
		rightTitle = fmt.Sprintf("Value: %s", m.selected)
		if m.focusRight {
			rightTitle += "  [scroll]"
		}
	}
	rightTitle = truncateString(rightTitle, lay.rightContentWidth)
	rightHeader := panelHeaderStyle.Render(padToWidth(rightTitle, lay.rightContentWidth))

	var rightBody string
	if m.editing {
		if m.valFormat == fmtJSON {
			rightBody = m.renderJSONEditor(lay)
		} else {
			rightBody = m.editor.View()
		}
	} else {
		rightBody = m.viewport.View()
	}
	right := paneStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			rightHeader,
			rightBody,
		),
	)

	spacer := strings.Repeat(" ", panelGap)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
	footerText := m.status
	if m.patternDelete {
		footerText = "Delete pattern (glob): " + m.patternInput.View() + "  (Enter confirm · Esc cancel)"
	}
	footer := footerBarStyle.Render(padToWidth(truncateString(footerText, lay.innerWidth), lay.innerWidth))

	app := lipgloss.NewStyle().Padding(appPadY, appPadX).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, body, footer),
	)
	if m.showAbout {
		return m.aboutView(lay)
	}
	if m.showGroupCounts {
		return m.groupCountsView(lay)
	}
	return app
}

func (m Model) listHeaderText() string {
	total := len(m.list.Items())
	visible := len(m.list.VisibleItems())
	suffix := ""
	if m.hasMoreKeys {
		suffix = "+"
	}
	if m.loadingKeys {
		suffix += "…"
	}
	if m.list.IsFiltered() || m.list.SettingFilter() {
		return fmt.Sprintf("Keys %d/%d%s", visible, total, suffix)
	}
	return fmt.Sprintf("Keys %d%s", total, suffix)
}

func (m Model) appHeaderLeft() string {
	left := appTitleStyle.Render("badger-gui")
	meta := appMetaStyle.Render(fmt.Sprintf("DB: %s", m.dbPath))
	return left + "  " + meta
}

func (m Model) appHeaderRight() string {
	total := len(m.list.Items())
	visible := len(m.list.VisibleItems())
	suffix := ""
	if m.hasMoreKeys {
		suffix = "+"
	}
	if m.loadingKeys {
		suffix += "…"
	}
	count := fmt.Sprintf("Keys: %d%s", total, suffix)
	if m.list.IsFiltered() || m.list.SettingFilter() {
		count = fmt.Sprintf("Keys: %d/%d%s", visible, total, suffix)
	}
	format := fmt.Sprintf("Format: %s", m.formatName())
	filter := ""
	if m.list.FilterState() != list.Unfiltered {
		fv := m.list.FilterValue()
		if fv == "" && m.list.SettingFilter() {
			fv = "..."
		}
		filter = fmt.Sprintf("Filter: %s", truncateString(fv, 20))
	}
	parts := []string{count, format}
	if filter != "" {
		parts = append(parts, filter)
	}
	if m.list.FilterState() == list.FilterApplied {
		if m.filterCountLoading {
			parts = append(parts, "Matches: …")
		} else if m.filterCountErr != "" {
			parts = append(parts, "Matches: !")
		} else if m.filterCountValid {
			parts = append(parts, fmt.Sprintf("Matches: %d", m.filterCount))
		}
	}
	return appMetaStyle.Render(strings.Join(parts, "  "))
}

func (m Model) groupCountsView(lay layout) string {
	modalWidth := min(lay.innerWidth-4, 72)
	if modalWidth < 30 {
		modalWidth = lay.innerWidth
	}

	var bodyLines []string
	if m.groupCountsLoading {
		bodyLines = append(bodyLines, fmt.Sprintf("%s Scanning keys…", m.groupSpinner.View()))
	} else if m.groupCountsErr != "" {
		bodyLines = append(bodyLines, fmt.Sprintf("Error: %s", m.groupCountsErr))
	} else if len(m.groupCounts) == 0 {
		bodyLines = append(bodyLines, "No keys found.")
	} else {
		maxVisible := lay.innerHeight/2 - 6
		if maxVisible < 5 {
			maxVisible = 5
		}

		// Adjust scroll offset so the cursor stays visible.
		if m.groupCursor < m.groupScrollOffset {
			m.groupScrollOffset = m.groupCursor
		}
		if m.groupCursor >= m.groupScrollOffset+maxVisible {
			m.groupScrollOffset = m.groupCursor - maxVisible + 1
		}

		end := m.groupScrollOffset + maxVisible
		if end > len(m.groupCounts) {
			end = len(m.groupCounts)
		}

		prefixColWidth := modalWidth - 14
		if prefixColWidth < 20 {
			prefixColWidth = 20
		}
		for i := m.groupScrollOffset; i < end; i++ {
			g := m.groupCounts[i]
			indent := strings.Repeat("  ", g.depth)
			name := indent + g.group
			if lipgloss.Width(name) > prefixColWidth {
				name = truncateString(name, prefixColWidth)
			}
			countStr := fmt.Sprintf("%d", g.count)
			gap := prefixColWidth - lipgloss.Width(name) + 2
			if gap < 1 {
				gap = 1
			}
			line := name + strings.Repeat(" ", gap) + countStr

			if i == m.groupCursor {
				line = lipgloss.NewStyle().
					Foreground(lipgloss.Color("170")).
					Bold(true).
					Render("│ " + line)
			} else {
				line = "  " + line
			}
			bodyLines = append(bodyLines, line)
		}

		if end < len(m.groupCounts) {
			bodyLines = append(bodyLines, fmt.Sprintf("  … %d more", len(m.groupCounts)-end))
		}
	}

	body := strings.Join(bodyLines, "\n")
	header := aboutTitleStyle.Render("Data Groups") + "\n" +
		appMetaStyle.Render("↑/↓: navigate · Enter: filter · Esc: close") + "\n"
	box := aboutBoxStyle.Width(modalWidth).Render(header + "\n" + body)
	content := lipgloss.Place(lay.innerWidth, lay.innerHeight, lipgloss.Center, lipgloss.Center, box)
	return lipgloss.NewStyle().Padding(appPadY, appPadX).Render(content)
}

func (m Model) aboutView(lay layout) string {
	modalWidth := min(lay.innerWidth-4, 72)
	if modalWidth < 30 {
		modalWidth = lay.innerWidth
	}
	lines := []string{
		"Hi,",
		"",
		"I work extensively with BadgerDB in production systems",
		"and often needed a focused, distraction-free way to",
		"inspect keys and values directly from the terminal.",
		"So I built this CLI GUI.",
		"",
		"The goal is simple: stay close to the data.",
		"During debugging, testing, or low-level exploration,",
		"having immediate visibility into the database",
		"structure makes a real difference.",
		"",
		"This tool is designed to be lightweight,",
		"fast, and developer-friendly — without",
		"adding unnecessary abstraction.",
		"",
		"I'm sharing it publicly in the hope that it",
		"can also contribute to the BadgerDB community",
		"and help other engineers who prefer",
		"working close to their storage layer.",
		"",
		"Feedback, ideas, and contributions are welcome.",
		"",
		"Press Esc or F1 to close.",
		"https://savasayik.com",
	}
	body := strings.Join(lines, "\n")
	box := aboutBoxStyle.Width(modalWidth).Render(aboutTitleStyle.Render("About Me") + "\n\n" + body)
	content := lipgloss.Place(lay.innerWidth, lay.innerHeight, lipgloss.Center, lipgloss.Center, box)
	return lipgloss.NewStyle().Padding(appPadY, appPadX).Render(content)
}

func joinLeftRight(left, right string, width int) string {
	if width <= 0 {
		return left + " " + right
	}
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)
	if lw+rw+1 > width {
		maxLeft := width - rw - 1
		if maxLeft < 1 {
			return truncateString(right, width)
		}
		left = truncateString(left, maxLeft)
		lw = lipgloss.Width(left)
	}
	gap := width - lw - rw
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func padToWidth(s string, width int) string {
	if width <= 0 {
		return s
	}
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func padAnsi(s string, width int) string {
	if width <= 0 {
		return s
	}
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func truncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= max {
		return s
	}
	return ansi.Truncate(s, max, "...")
}

func formatLineNumberFixed(n, width int) string {
	if width <= 0 {
		return ""
	}
	if n <= 0 {
		return strings.Repeat(" ", width)
	}
	s := strconv.Itoa(n)
	if len(s) >= width {
		return s[len(s)-width:]
	}
	return strings.Repeat(" ", width-len(s)) + s
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
