package ui

import "github.com/charmbracelet/lipgloss"

var (
	// I keep UI styles here.
	borderColor = lipgloss.Color("240")
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
	paneStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(borderColor).Padding(0, 1)
	aboutBoxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(1, 2)
	aboutTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true)

	appTitleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true)
	appMetaStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	headerBarStyle   = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))
	panelHeaderStyle = lipgloss.NewStyle().Background(lipgloss.Color("238")).Foreground(lipgloss.Color("252")).Bold(true)
	footerBarStyle   = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))

	jsonKeyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	jsonStringStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	jsonNumberStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	jsonBoolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("111")).Bold(true)
	jsonNullStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("111")).Bold(true)
	jsonPunctStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	jsonErrorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)

	editorLineNumberStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
)

const (
	appPadX          = 2
	appPadY          = 1
	headerHeight     = 1
	footerHeight     = 1
	panelGap         = 1
	panelHeaderLines = 1
)
