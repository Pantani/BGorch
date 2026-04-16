package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Pantani/gorchestrator/internal/app"
)

// Run starts the interactive Bubble Tea UI.
func Run(application *app.App) error {
	program := tea.NewProgram(New(application), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}
	return nil
}
