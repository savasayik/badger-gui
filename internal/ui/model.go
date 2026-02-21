package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

const defaultPageSize = 500

func NewModel(store Store, dbPath string) Model {
	items := make([]list.Item, 0, defaultPageSize)

	l := list.New(items, thinCursorDelegate{}, 0, 0)
	l.Title = "Badger Keys"
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(true)

	ta := textarea.New()
	ta.Placeholder = "Edit mode..."
	ta.ShowLineNumbers = true
	ta.SetWidth(60)
	ta.SetHeight(10)

	pi := textinput.New()
	pi.Placeholder = "rec:*"
	pi.CharLimit = 256
	pi.Prompt = "Pattern: "

	return Model{
		store:        store,
		list:         l,
		status:       "↑/↓: list · Enter: load & focus value · Esc/Shift+←: back · t/h/b/j: format · /: filter · e: edit · d/Delete: delete · p: delete pattern · g: groups · F1: about · q: exit",
		valFormat:    fmtJSON,
		editor:       ta,
		dbPath:       dbPath,
		patternInput: pi,
		pageSize:     defaultPageSize,
		hasMoreKeys:  true,
		loadingKeys:  true,
	}
}

func (m Model) Init() tea.Cmd {
	return loadKeysCmd(m.store, "", m.pageSize)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		if msg.String() == "f1" {
			m.showAbout = !m.showAbout
			return m, nil
		}
		if m.showAbout {
			if msg.String() == "esc" {
				m.showAbout = false
			}
			return m, nil
		}
		// I handle delete confirmation flow.
		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y", "enter":
				key := m.pendingDelete
				m.confirmDelete = false
				m.pendingDelete = ""
				m.status = "Deleting..."
				return m, deleteKeyCmd(m.store, key)
			case "n", "N", "esc":
				m.confirmDelete = false
				m.pendingDelete = ""
				m.status = "Delete canceled."
				return m, nil
			}
			return m, nil
		}

		// I handle pattern delete confirmation.
		if m.confirmPatternDelete {
			switch msg.String() {
			case "y", "Y", "enter":
				pattern := m.pendingPattern
				m.confirmPatternDelete = false
				m.pendingPattern = ""
				m.status = "Deleting by pattern..."
				return m, deletePatternCmd(m.store, pattern)
			case "n", "N", "esc":
				m.confirmPatternDelete = false
				m.pendingPattern = ""
				m.status = "Pattern delete canceled."
				return m, nil
			}
			return m, nil
		}

		// I handle edit mode.
		if m.editing {
			switch msg.String() {
			case "esc":
				m.editing = false
				m.editKey = "" // I clear the edit key when canceling.
				m.focusRight = true
				m.status = "Edit canceled."
				m.updateEditorLayout(computeLayout(m.width, m.height))
				return m, nil
			case "ctrl+s":
				// I save changes.
				bytes, err := m.bytesFromEditor()
				if err != nil {
					m.status = errStyle.Render(fmt.Sprintf("Error: save failed: %v", err))
					return m, nil
				}
				m.status = "Saving..."
				return m, saveValueCmd(m.store, m.editKey, bytes)
			}
			// I pass through other editor keys.
			var ecmd tea.Cmd
			m.editor, ecmd = m.editor.Update(msg)
			return m, ecmd
		}

		// I handle pattern input.
		if m.patternDelete {
			switch msg.String() {
			case "esc":
				m.patternDelete = false
				m.patternInput.Blur()
				m.status = "Pattern delete canceled."
				return m, nil
			case "enter":
				pattern := strings.TrimSpace(m.patternInput.Value())
				if pattern == "" {
					m.patternDelete = false
					m.patternInput.Blur()
					m.status = "Pattern delete canceled."
					return m, nil
				}
				if _, err := matchPattern(pattern, ""); err != nil {
					m.status = errStyle.Render(fmt.Sprintf("Error: invalid pattern: %v", err))
					return m, nil
				}
				m.patternDelete = false
				m.patternInput.Blur()
				m.confirmPatternDelete = true
				m.pendingPattern = pattern
				m.status = fmt.Sprintf("Delete pattern '%s'? (y/n)", pattern)
				return m, nil
			}
			var pcmd tea.Cmd
			m.patternInput, pcmd = m.patternInput.Update(msg)
			return m, pcmd
		}

		// I disable global shortcuts while filtering.
		if m.list.SettingFilter() {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			maybeFilter, filterCmd := m.maybeStartFilterWork()
			return maybeFilter, tea.Batch(cmd, filterCmd)
		}

		// I handle right-panel focus (value view).
		if m.focusRight && !m.editing {
			switch msg.String() {
			case "esc", "shift+left":
				m.focusRight = false
				m.status = "List focused."
				return m, nil
			case "p":
				m.patternDelete = true
				m.patternInput.SetValue("")
				m.patternInput.Focus()
				m.status = "Pattern delete mode. (Enter confirm · Esc cancel)"
				return m, nil
			case "t":
				m.valFormat = fmtText
				return m.reloadSelected()
			case "h":
				m.valFormat = fmtHex
				return m.reloadSelected()
			case "b":
				m.valFormat = fmtBase64
				return m.reloadSelected()
			case "j":
				m.valFormat = fmtJSON
				return m.reloadSelected()
			case "e":
				if m.selected != "" {
					m.editKey = m.selected
					m.status = "Loading..."
					return m, loadValueCmd(m.store, m.selected)
				}
			case "g", "G", "ctrl+g":
				return m.toggleGroupCounts()
			}

			var vcmd tea.Cmd
			m.viewport, vcmd = m.viewport.Update(msg)
			return m, vcmd
		}

		// I handle normal mode.
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit

		case "t":
			m.valFormat = fmtText
			return m.reloadSelected()

		case "h":
			m.valFormat = fmtHex
			return m.reloadSelected()

		case "b":
			m.valFormat = fmtBase64
			return m.reloadSelected()

		case "j":
			m.valFormat = fmtJSON
			return m.reloadSelected()

		case "enter":
			i, ok := m.list.SelectedItem().(kvItem)
			if ok {
				m.selected = i.key
				m.editKey = "" // I clear editKey for normal loads.
				m.focusRight = true
				return m, loadValueCmd(m.store, i.key)
			}

		case "d", "delete":
			i, ok := m.list.SelectedItem().(kvItem)
			if ok {
				m.confirmDelete = true
				m.pendingDelete = i.key
				m.status = fmt.Sprintf("Delete '%s'? (y/n)", i.key)
			}
			return m, nil

		case "p":
			m.patternDelete = true
			m.patternInput.SetValue("")
			m.patternInput.Focus()
			m.status = "Pattern delete mode. (Enter confirm · Esc cancel)"
			return m, nil

		case "e":
			// I enter edit mode.
			i, ok := m.list.SelectedItem().(kvItem)
			if ok {
				m.selected = i.key
				m.editKey = i.key // I mark that edit mode should start after load.
				m.focusRight = true
				m.status = "Loading..."
				// I load the value via loadValueCmd.
				return m, loadValueCmd(m.store, i.key)
			}
		case "g", "G", "ctrl+g":
			return m.toggleGroupCounts()
		}

	case tea.WindowSizeMsg:
		// I split the window into two columns.
		if !m.ready {
			m.ready = true
		}
		m.width = msg.Width
		m.height = msg.Height
		lay := computeLayout(msg.Width, msg.Height)
		m.list.SetSize(lay.listWidth, lay.listHeight)
		m.viewport = viewport.Model{
			Width:  lay.rightContentWidth,
			Height: lay.rightContentHeight,
		}
		m.updateEditorLayout(lay)
		_, moreCmd := m.maybeLoadMore()
		maybeFilter, filterCmd := m.maybeStartFilterWork()
		return maybeFilter, tea.Batch(moreCmd, filterCmd)

	case loadKeysMsg:
		m.loadingKeys = false
		if msg.err != nil {
			m.status = errStyle.Render(fmt.Sprintf("Error: failed to load keys: %v", msg.err))
			m.hasMoreKeys = false
			return m, nil
		}
		if len(msg.keys) == 0 {
			m.hasMoreKeys = msg.hasMore
			return m, nil
		}
		items := m.list.Items()
		for _, k := range msg.keys {
			items = append(items, kvItem{key: k})
		}
		cmd := m.list.SetItems(items)
		m.lastKey = msg.lastKey
		m.hasMoreKeys = msg.hasMore
		_, moreCmd := m.maybeLoadMore()
		maybeFilter, filterCmd := m.maybeStartFilterWork()
		return maybeFilter, tea.Batch(cmd, moreCmd, filterCmd)

	case filterCountMsg:
		if msg.term != strings.TrimSpace(m.list.FilterValue()) {
			return m, nil
		}
		m.filterCountLoading = false
		if msg.err != nil {
			m.filterCountErr = msg.err.Error()
			m.filterCountValid = false
			return m, nil
		}
		m.filterCountErr = ""
		m.filterCount = msg.count
		m.filterCountValid = true
		return m, nil

	case groupCountsMsg:
		m.groupCountsLoading = false
		if msg.err != nil {
			m.groupCountsErr = msg.err.Error()
			return m, nil
		}
		m.groupCountsErr = ""
		m.groupCounts = msg.counts
		return m, nil

	case loadValueMsg:
		if msg.key != m.selected && m.editKey != msg.key {
			return m, nil
		}
		if msg.err != nil {
			// I clear editKey on error.
			if m.editKey == msg.key {
				m.editKey = ""
			}
			m.viewport.SetContent(fmt.Sprintf("Error: %v", msg.err))
			return m, nil
		}

		m.lastLoadValue = msg.value

		var cmd tea.Cmd // I keep a focus command here.

		// I start edit mode only when load was triggered by 'e' (editKey set).
		if m.editKey == msg.key && !m.editing {
			// I start edit mode.
			m.startEditWithContent(msg.key, msg.value)
			m.updateEditorLayout(computeLayout(m.width, m.height))

			// I return focus to the editor to activate editing.
			cmd = m.editor.Focus()

			return m, cmd // I return the focus command.
		}

		// I handle normal loads (Enter or format change).
		m.viewport.SetContent(m.formatValue(msg.key, msg.value))
		m.viewport.GotoTop()
		return m, nil

	case deleteResultMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Error: delete failed: %v", msg.err)
			return m, nil
		}
		// I remove the selected item from the list.
		idx := m.list.Index()
		if idx >= 0 && idx < len(m.list.Items()) {
			m.list.RemoveItem(idx)
		}
		// I clear the right panel and selection.
		m.selected = ""
		m.viewport.SetContent("")
		m.status = okStyle.Render(fmt.Sprintf("'%s' deleted.", msg.key))
		return m, nil

	case deletePatternResultMsg:
		if msg.err != nil {
			m.status = errStyle.Render(fmt.Sprintf("Error: pattern delete failed: %v", msg.err))
			return m, nil
		}
		if len(msg.keys) == 0 {
			m.status = errStyle.Render(fmt.Sprintf("Warning: no matches for pattern: %s", msg.pattern))
			return m, nil
		}

		remove := make(map[string]struct{}, len(msg.keys))
		for _, k := range msg.keys {
			remove[k] = struct{}{}
		}

		items := m.list.Items()
		newItems := make([]list.Item, 0, len(items))
		for _, it := range items {
			ki, ok := it.(kvItem)
			if ok {
				if _, exists := remove[ki.key]; exists {
					continue
				}
			}
			newItems = append(newItems, it)
		}
		cmd := m.list.SetItems(newItems)
		if _, deleted := remove[m.selected]; deleted {
			m.selected = ""
			m.viewport.SetContent("")
		}
		m.status = okStyle.Render(fmt.Sprintf("Deleted %d records (pattern: %s).", len(msg.keys), msg.pattern))
		return m, cmd

	case saveResultMsg:
		if msg.err != nil {
			m.status = errStyle.Render(fmt.Sprintf("Error: save failed: %v", msg.err))
			return m, nil
		}
		m.editing = false
		m.editKey = "" // I clear editKey after a successful save.
		m.focusRight = true
		m.status = okStyle.Render(fmt.Sprintf("'%s' updated.", msg.key))
		m.updateEditorLayout(computeLayout(m.width, m.height))
		// I reload the right panel.
		return m, loadValueCmd(m.store, msg.key)
	}

	// I update the list and viewport.
	var cmd tea.Cmd
	var prevKey string
	if !m.focusRight && !m.editing && !m.list.SettingFilter() {
		if i, ok := m.list.SelectedItem().(kvItem); ok {
			prevKey = i.key
		}
	}
	m.list, cmd = m.list.Update(msg)
	maybe, moreCmd := m.maybeLoadMore()
	maybeFilter, filterCmd := maybe.maybeStartFilterWork()
	if !maybeFilter.focusRight && !maybeFilter.editing && !maybeFilter.list.SettingFilter() {
		if i, ok := maybeFilter.list.SelectedItem().(kvItem); ok {
			if i.key != "" && i.key != prevKey {
				maybeFilter.selected = i.key
				maybeFilter.editKey = ""
				return maybeFilter, tea.Batch(cmd, moreCmd, filterCmd, loadValueCmd(maybeFilter.store, i.key))
			}
		}
	}
	return maybeFilter, tea.Batch(cmd, moreCmd, filterCmd)
}

func (m Model) maybeLoadMore() (Model, tea.Cmd) {
	if !m.hasMoreKeys || m.loadingKeys {
		return m, nil
	}
	if m.list.IsFiltered() || m.list.SettingFilter() || m.list.FilterState() != list.Unfiltered {
		return m, nil
	}
	items := m.list.Items()
	if len(items) == 0 {
		return m, nil
	}
	threshold := 5
	if m.list.Index() >= len(items)-1-threshold {
		m.loadingKeys = true
		return m, loadKeysCmd(m.store, m.lastKey, m.pageSize)
	}
	return m, nil
}

func (m Model) maybeStartFilterWork() (Model, tea.Cmd) {
	var cmds []tea.Cmd
	state := m.list.FilterState()
	if state == list.Unfiltered {
		m.filterCountLoading = false
		m.filterCountErr = ""
		m.filterTerm = ""
		m.filterCount = 0
		m.filterCountValid = false
		m.loadingAllForFilter = false
		return m, nil
	}
	term := strings.TrimSpace(m.list.FilterValue())
	if term == "" {
		m.filterCountLoading = false
		m.filterCountErr = ""
		m.filterTerm = ""
		m.filterCount = 0
		m.filterCountValid = false
		m.loadingAllForFilter = false
		return m, nil
	}
	if state == list.FilterApplied {
		if term == m.filterTerm && m.filterCountValid && !m.filterCountLoading {
			// I keep the current count.
		} else if !m.filterCountLoading || term != m.filterTerm {
			m.filterTerm = term
			m.filterCountLoading = true
			m.filterCountErr = ""
			m.filterCountValid = false
			cmds = append(cmds, countFilterCmd(m.store, term))
		}
	}
	m.loadingAllForFilter = true
	if m.hasMoreKeys && !m.loadingKeys {
		m.loadingKeys = true
		cmds = append(cmds, loadKeysCmd(m.store, m.lastKey, m.pageSize))
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m Model) toggleGroupCounts() (Model, tea.Cmd) {
	m.showGroupCounts = !m.showGroupCounts
	if !m.showGroupCounts {
		return m, nil
	}
	if m.groupCountsLoading {
		return m, nil
	}
	m.groupCountsLoading = true
	m.groupCountsErr = ""
	return m, loadGroupCountsCmd(m.store)
}
