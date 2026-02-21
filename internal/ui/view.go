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
		panel := lipgloss.NewStyle().Padding(appPadY, appPadX).Render(m.groupCountsView(lay.innerWidth))
		return lipgloss.JoinVertical(lipgloss.Left, panel, app)
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

func (m Model) groupCountsView(width int) string {
	title := "Group counts (prefix before ':')"
	lines := []string{title}
	if m.groupCountsLoading {
		lines = append(lines, "Loading…")
	} else if m.groupCountsErr != "" {
		lines = append(lines, fmt.Sprintf("Error: %s", m.groupCountsErr))
	} else if len(m.groupCounts) == 0 {
		lines = append(lines, "No keys found.")
	} else {
		maxLines := m.height / 2
		if maxLines < 5 {
			maxLines = 5
		}
		if maxLines > len(m.groupCounts)+1 {
			maxLines = len(m.groupCounts) + 1
		}
		for i := 0; i < len(m.groupCounts) && i < maxLines-1; i++ {
			g := m.groupCounts[i]
			name := g.group
			if lipgloss.Width(name) > 20 {
				name = truncateString(name, 20)
			}
			lines = append(lines, fmt.Sprintf("%-20s %d", name, g.count))
		}
		if len(m.groupCounts) > maxLines-1 {
			lines = append(lines, fmt.Sprintf("… %d more", len(m.groupCounts)-(maxLines-1)))
		}
	}
	content := strings.Join(lines, "\n")
	panel := paneStyle.Width(width).Render(content)
	return panel
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
