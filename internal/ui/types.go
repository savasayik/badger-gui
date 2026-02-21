package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Store interface {
	ListKeysPage(startAfter string, limit int) ([]string, string, bool, error)
	CountKeysMatching(term string) (int, error)
	GroupKeyCounts() (map[string]int, error)
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
	Delete(key string) error
}

type kvItem struct{ key string }

func (i kvItem) Title() string       { return i.key }
func (i kvItem) Description() string { return "" }
func (i kvItem) FilterValue() string { return i.key }

// I use a thin cursor and no bold in the delegate.
type thinCursorDelegate struct{}

func (d thinCursorDelegate) Height() int                               { return 1 }
func (d thinCursorDelegate) Spacing() int                              { return 0 }
func (d thinCursorDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d thinCursorDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	it, _ := listItem.(kvItem)

	// I show a thin line for the selected row; otherwise I use spaces.
	cursor := "  "
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250")) // I keep the normal style (no bold).
	if index == m.Index() {
		cursor = "│ "
		titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true) // I use the selected color.
	}

	fmt.Fprintf(w, "%s%s", cursor, titleStyle.Render(it.Title()))
}

type valueFormat int

const (
	fmtText valueFormat = iota
	fmtHex
	fmtBase64
	fmtJSON
)

type Model struct {
	store      Store
	list       list.Model
	viewport   viewport.Model
	status     string
	valFormat  valueFormat
	ready      bool
	selected   string
	dbPath     string
	focusRight bool
	width      int
	height     int

	editorHeight        int
	pageSize            int
	lastKey             string
	hasMoreKeys         bool
	loadingKeys         bool
	filterTerm          string
	filterCount         int
	filterCountValid    bool
	filterCountLoading  bool
	filterCountErr      string
	loadingAllForFilter bool
	groupCounts         []groupCount
	groupCountsLoading  bool
	groupCountsErr      string
	showGroupCounts     bool
	showAbout           bool

	// I track delete confirmation state.
	confirmDelete bool
	pendingDelete string

	// I track pattern delete state.
	patternDelete        bool
	patternInput         textinput.Model
	confirmPatternDelete bool
	pendingPattern       string

	// I track edit mode state.
	editing       bool
	editor        textarea.Model
	editKey       string // I track the key being edited.
	editorHelp    string
	lastLoadValue []byte
}

type loadValueMsg struct {
	key   string
	value []byte
	err   error
}

type deleteResultMsg struct {
	key string
	err error
}

type saveResultMsg struct {
	key string
	err error
}

type deletePatternResultMsg struct {
	pattern string
	keys    []string
	err     error
}

type loadKeysMsg struct {
	keys       []string
	lastKey    string
	hasMore    bool
	startAfter string
	err        error
}

type filterCountMsg struct {
	term  string
	count int
	err   error
}

type groupCount struct {
	group string
	count int
}

type groupCountsMsg struct {
	counts []groupCount
	err    error
}

type visualLine struct {
	text   string
	lineNo int
}
