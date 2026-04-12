package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
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

	ni := textinput.New()
	ni.Placeholder = "my:new:key"
	ni.CharLimit = 512
	ni.Prompt = "Key: "

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return Model{
		store:        store,
		list:         l,
		status:       "↑/↓: list · Enter: load · /: filter · e: edit · n: new · d: delete · p: pattern · c: copy · x/X: export · Ctrl+Z: undo · g: groups · q: exit",
		valFormat:    fmtJSON,
		editor:       ta,
		dbPath:       dbPath,
		patternInput: pi,
		newKeyInput:  ni,
		groupSpinner: sp,
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

		// Handle group counts modal — both during loading and after data arrives.
		if m.showGroupCounts {
			if m.groupCountsLoading {
				// Only allow Esc to close while loading; all other keys are swallowed.
				if msg.String() == "esc" || msg.String() == "g" {
					m.showGroupCounts = false
					m.groupCountsLoading = false
				}
				return m, nil
			}
			switch msg.String() {
			case "esc", "g":
				m.showGroupCounts = false
				return m, nil
			case "up", "k":
				if m.groupCursor > 0 {
					m.groupCursor--
				}
				return m, nil
			case "down", "j":
				if m.groupCursor < len(m.groupCounts)-1 {
					m.groupCursor++
				}
				return m, nil
			case "enter":
				if m.groupCursor >= 0 && m.groupCursor < len(m.groupCounts) {
					// Close the modal and apply the selected group as a filter.
					selected := m.groupCounts[m.groupCursor]
					m.showGroupCounts = false
					m.groupCursor = 0
					m.list.SetFilterText(selected.group)
					maybeFilter, filterCmd := m.maybeStartFilterWork()
					return maybeFilter, filterCmd
				}
				return m, nil
			}
			return m, nil
		}

		// Handle delete confirmation flow.
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

		// Handle pattern delete confirmation.
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

		// Handle new key name input.
		if m.creatingKey {
			switch msg.String() {
			case "esc":
				m.creatingKey = false
				m.newKeyInput.Blur()
				m.status = "New key canceled."
				return m, nil
			case "enter":
				name := strings.TrimSpace(m.newKeyInput.Value())
				if name == "" {
					m.creatingKey = false
					m.newKeyInput.Blur()
					m.status = "New key canceled (empty name)."
					return m, nil
				}
				m.creatingKey = false
				m.newKeyInput.Blur()
				m.creatingValue = true
				m.newKeyName = name
				m.editing = true
				m.editKey = name
				m.focusRight = true
				m.valFormat = fmtText
				m.editor.SetValue("")
				m.editor.CursorEnd()
				m.lastLoadValue = nil
				m.status = fmt.Sprintf("Enter value for '%s'. (Ctrl+S save · Esc cancel)", name)
				m.updateEditorLayout(computeLayout(m.width, m.height))
				return m, m.editor.Focus()
			}
			var ncmd tea.Cmd
			m.newKeyInput, ncmd = m.newKeyInput.Update(msg)
			return m, ncmd
		}

		// Handle edit mode.
		if m.editing {
			switch msg.String() {
			case "esc":
				m.editing = false
				m.editKey = ""
				m.creatingValue = false
				m.newKeyName = ""
				m.focusRight = true
				m.status = "Edit canceled."
				m.updateEditorLayout(computeLayout(m.width, m.height))
				return m, nil
			case "ctrl+s":
				// Save changes.
				bytes, err := m.bytesFromEditor()
				if err != nil {
					m.status = errStyle.Render(fmt.Sprintf("Error: save failed: %v", err))
					return m, nil
				}
				m.status = "Saving..."
				if m.creatingValue {
					m.pushUndo(undoEntry{op: undoCreate, key: m.editKey})
					return m, createKeyCmd(m.store, m.editKey, bytes)
				}
				m.pushUndo(undoEntry{op: undoEdit, key: m.editKey, oldValue: m.lastLoadValue})
				return m, saveValueCmd(m.store, m.editKey, bytes)
			}
			// Pass through other editor keys.
			var ecmd tea.Cmd
			m.editor, ecmd = m.editor.Update(msg)
			return m, ecmd
		}

		// Handle pattern input.
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

		// Disable global shortcuts while filtering.
		if m.list.SettingFilter() {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			maybeFilter, filterCmd := m.maybeStartFilterWork()
			return maybeFilter, tea.Batch(cmd, filterCmd)
		}

		// Handle full database export.
		if msg.String() == "ctrl+e" && !m.editing && !m.creatingKey {
			m.status = "Exporting entire database..."
			return m, exportAllCmd(m.store)
		}

		// Handle undo in any non-modal, non-editing state.
		if msg.String() == "ctrl+z" && !m.editing && !m.creatingKey {
			entry, ok := m.popUndo()
			if !ok {
				m.status = "Nothing to undo."
				return m, nil
			}
			m.status = "Undoing..."
			return m, undoCmd(m.store, entry)
		}

		// Handle right-panel focus (value view).
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
			case "c":
				if m.selected != "" && m.lastLoadValue != nil {
					content := m.plainFormatValue(m.lastLoadValue)
					return m, copyToClipboardCmd(content, "value")
				}
				return m, nil
			case "x":
				if m.selected != "" {
					m.status = "Exporting..."
					return m, exportSingleCmd(m.store, m.selected)
				}
				return m, nil
			case "g", "G", "ctrl+g":
				return m.toggleGroupCounts()
			}

			var vcmd tea.Cmd
			m.viewport, vcmd = m.viewport.Update(msg)
			return m, vcmd
		}

		// Handle normal mode.
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
				m.editKey = "" // Clear editKey for normal loads.
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
			// Enter edit mode.
			i, ok := m.list.SelectedItem().(kvItem)
			if ok {
				m.selected = i.key
				m.editKey = i.key // Mark that edit mode should start after load.
				m.focusRight = true
				m.status = "Loading..."
				// Load the value via loadValueCmd.
				return m, loadValueCmd(m.store, i.key)
			}
		case "n":
			m.creatingKey = true
			m.newKeyInput.SetValue("")
			m.newKeyInput.Focus()
			m.status = "New key. Enter key name: (Enter confirm · Esc cancel)"
			return m, nil
		case "x":
			i, ok := m.list.SelectedItem().(kvItem)
			if ok {
				m.status = "Exporting..."
				return m, exportSingleCmd(m.store, i.key)
			}
			return m, nil
		case "X":
			visible := m.list.VisibleItems()
			if len(visible) == 0 {
				m.status = "No keys to export."
				return m, nil
			}
			keys := make([]string, 0, len(visible))
			for _, it := range visible {
				if ki, ok := it.(kvItem); ok {
					keys = append(keys, ki.key)
				}
			}
			m.status = fmt.Sprintf("Exporting %d keys...", len(keys))
			return m, exportVisibleCmd(m.store, keys)
		case "c":
			i, ok := m.list.SelectedItem().(kvItem)
			if ok {
				return m, copyToClipboardCmd(i.key, "key name")
			}
			return m, nil
		case "g", "G", "ctrl+g":
			return m.toggleGroupCounts()
		}

	case tea.WindowSizeMsg:
		// Split the window into two columns.
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
		m.lastKey = msg.lastKey
		m.hasMoreKeys = msg.hasMore

		// Buffer new keys while the user is typing a filter
		// so that SetItems does not disrupt the active filter input.
		if m.list.SettingFilter() {
			m.pendingKeys = append(m.pendingKeys, msg.keys...)
			maybeFilter, filterCmd := m.maybeStartFilterWork()
			return maybeFilter, filterCmd
		}

		items := m.list.Items()
		for _, k := range msg.keys {
			items = append(items, kvItem{key: k})
		}
		cmd := m.list.SetItems(items)
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

	case spinner.TickMsg:
		// Update the spinner only while the group modal is loading.
		if m.showGroupCounts && m.groupCountsLoading {
			var cmd tea.Cmd
			m.groupSpinner, cmd = m.groupSpinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case loadValueMsg:
		if msg.key != m.selected && m.editKey != msg.key {
			return m, nil
		}
		if msg.err != nil {
			// Clear editKey on error.
			if m.editKey == msg.key {
				m.editKey = ""
			}
			m.viewport.SetContent(fmt.Sprintf("Error: %v", msg.err))
			return m, nil
		}

		m.lastLoadValue = msg.value

		var cmd tea.Cmd // Focus command.

		// Start edit mode only when load was triggered by 'e' (editKey set).
		if m.editKey == msg.key && !m.editing {
			// Start edit mode.
			m.startEditWithContent(msg.key, msg.value)
			m.updateEditorLayout(computeLayout(m.width, m.height))

			// Return focus to the editor to activate editing.
			cmd = m.editor.Focus()

			return m, cmd
		}

		// Handle normal loads (Enter or format change).
		m.viewport.SetContent(m.formatValue(msg.key, msg.value))
		m.viewport.GotoTop()
		return m, nil

	case deleteResultMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Error: delete failed: %v", msg.err)
			return m, nil
		}
		if msg.oldValue != nil {
			m.pushUndo(undoEntry{op: undoDelete, key: msg.key, oldValue: msg.oldValue})
		}
		// Remove the selected item from the list.
		idx := m.list.Index()
		if idx >= 0 && idx < len(m.list.Items()) {
			m.list.RemoveItem(idx)
		}
		// Clear the right panel and selection.
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

		for _, k := range msg.keys {
			if v, ok := msg.oldValues[k]; ok {
				m.pushUndo(undoEntry{op: undoDelete, key: k, oldValue: v})
			}
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

	case undoResultMsg:
		if msg.err != nil {
			m.status = errStyle.Render(fmt.Sprintf("Error: undo failed: %v", msg.err))
			return m, nil
		}
		switch msg.op {
		case undoEdit:
			m.status = okStyle.Render(fmt.Sprintf("Undo: '%s' restored to previous value.", msg.key))
			if m.selected == msg.key {
				return m, loadValueCmd(m.store, msg.key)
			}
		case undoDelete:
			m.status = okStyle.Render(fmt.Sprintf("Undo: '%s' restored.", msg.key))
			items := m.list.Items()
			items = append(items, kvItem{key: msg.key})
			cmd := m.list.SetItems(items)
			return m, cmd
		case undoCreate:
			m.status = okStyle.Render(fmt.Sprintf("Undo: created key '%s' removed.", msg.key))
			for i, it := range m.list.Items() {
				if ki, ok := it.(kvItem); ok && ki.key == msg.key {
					m.list.RemoveItem(i)
					break
				}
			}
			if m.selected == msg.key {
				m.selected = ""
				m.viewport.SetContent("")
			}
		}
		return m, nil

	case exportResultMsg:
		if msg.err != nil {
			m.status = errStyle.Render(fmt.Sprintf("Error: export failed: %v", msg.err))
			return m, nil
		}
		m.status = okStyle.Render(fmt.Sprintf("Exported %d key(s) to %s", msg.count, msg.filePath))
		return m, nil

	case clipboardResultMsg:
		if msg.err != nil {
			m.status = errStyle.Render(fmt.Sprintf("Error: clipboard: %v", msg.err))
			return m, nil
		}
		m.status = okStyle.Render(fmt.Sprintf("Copied %s to clipboard.", msg.what))
		return m, nil

	case saveResultMsg:
		if msg.err != nil {
			m.status = errStyle.Render(fmt.Sprintf("Error: save failed: %v", msg.err))
			return m, nil
		}
		m.editing = false
		m.editKey = ""
		m.focusRight = true
		m.updateEditorLayout(computeLayout(m.width, m.height))

		if msg.isNew {
			m.creatingValue = false
			m.newKeyName = ""
			m.selected = msg.key
			// Add the new key to the list.
			items := m.list.Items()
			items = append(items, kvItem{key: msg.key})
			setCmd := m.list.SetItems(items)
			// Select the new item.
			for i, it := range m.list.Items() {
				if ki, ok := it.(kvItem); ok && ki.key == msg.key {
					m.list.Select(i)
					break
				}
			}
			m.status = okStyle.Render(fmt.Sprintf("'%s' created.", msg.key))
			return m, tea.Batch(setCmd, loadValueCmd(m.store, msg.key))
		}

		m.status = okStyle.Render(fmt.Sprintf("'%s' updated.", msg.key))
		return m, loadValueCmd(m.store, msg.key)
	}

	// Update the list and viewport.
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

	// Flush buffered keys when the user is no longer typing a filter.
	if !m.list.SettingFilter() && len(m.pendingKeys) > 0 {
		items := m.list.Items()
		for _, k := range m.pendingKeys {
			items = append(items, kvItem{key: k})
		}
		cmds = append(cmds, m.list.SetItems(items))
		m.pendingKeys = nil
	}

	if state == list.Unfiltered {
		m.filterCountLoading = false
		m.filterCountErr = ""
		m.filterTerm = ""
		m.filterCount = 0
		m.filterCountValid = false
		m.loadingAllForFilter = false

		// Resume normal pagination after the filter is cleared.
		m2, moreCmd := m.maybeLoadMore()
		if moreCmd != nil {
			cmds = append(cmds, moreCmd)
		}
		if len(cmds) > 0 {
			return m2, tea.Batch(cmds...)
		}
		return m2, nil
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
			// Keep the current count.
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

func (m *Model) pushUndo(entry undoEntry) {
	m.undoStack = append(m.undoStack, entry)
	if len(m.undoStack) > 20 {
		m.undoStack = m.undoStack[len(m.undoStack)-20:]
	}
}

func (m *Model) popUndo() (undoEntry, bool) {
	if len(m.undoStack) == 0 {
		return undoEntry{}, false
	}
	entry := m.undoStack[len(m.undoStack)-1]
	m.undoStack = m.undoStack[:len(m.undoStack)-1]
	return entry, true
}

func (m Model) toggleGroupCounts() (Model, tea.Cmd) {
	m.showGroupCounts = !m.showGroupCounts
	if !m.showGroupCounts {
		return m, nil
	}
	if m.groupCountsLoading {
		return m, nil
	}
	m.groupCursor = 0
	m.groupScrollOffset = 0
	m.groupCountsLoading = true
	m.groupCountsErr = ""
	return m, tea.Batch(m.groupSpinner.Tick, loadTreeGroupCountsCmd(m.store, 3))
}
