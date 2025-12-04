package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the TUI application
// presets can contain values from CLI flags to pre-populate inputs
func Run(presets *Presets) error {
	// Create initial model
	model := NewModel(presets)

	// Create program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Set global program reference for async updates
	SetProgram(p)

	// Run the program
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Check if there was an error in the final state
	if m, ok := finalModel.(Model); ok {
		if m.err != nil && m.screen == ScreenError {
			// User quit with an error showing - don't propagate
			return nil
		}
	}

	return nil
}

// RunWithResult starts the TUI and returns the packaging result
// This is useful for testing or automation
func RunWithResult(presets *Presets) (*Model, error) {
	model := NewModel(presets)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	SetProgram(p)

	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("TUI error: %w", err)
	}

	if m, ok := finalModel.(Model); ok {
		return &m, nil
	}

	return nil, fmt.Errorf("unexpected model type")
}
