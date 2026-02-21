package app

import (
	"fmt"

	"badge-reader/internal/store"
	"badge-reader/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func Run(dbPath string) error {
	st, err := store.OpenBadger(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open badger db: %w", err)
	}
	defer st.Close()

	m := ui.NewModel(st, dbPath)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		return err
	}
	return nil
}
