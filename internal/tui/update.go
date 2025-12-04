package tui

import (
	"path/filepath"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
	)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// File picker needs to receive ALL messages (not just KeyMsg) to read directories
	if m.screen == ScreenFilePicker {
		return m.updateFilePicker(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Global quit handling
		if key.Matches(msg, m.keys.Quit) && m.screen != ScreenProcessing {
			return m, tea.Quit
		}

		// Screen-specific key handling
		switch m.screen {
		case ScreenWelcome:
			return m.updateWelcome(msg)
		case ScreenInput:
			return m.updateInput(msg)
		case ScreenProcessing:
			return m.updateProcessing(msg)
		case ScreenSuccess:
			return m.updateSuccess(msg)
		case ScreenError:
			return m.updateError(msg)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)


	case packageStartMsg:
		m.screen = ScreenProcessing
		m.progress = 0
		m.progressStep = "Starting..."
		cmds = append(cmds, m.spinner.Tick)

	case packageProgressMsg:
		m.SetProgress(msg.step, msg.percent)

	case packageCompleteMsg:
		m.screen = ScreenSuccess
		m.result = msg.result
		m.progress = 1.0

	case packageErrorMsg:
		m.screen = ScreenError
		m.err = msg.err

	case setupFileDetectedMsg:
		if msg.filename != "" && m.inputs[1].Value() == "" {
			m.inputs[1].SetValue(msg.filename)
		}
	}

	return m, tea.Batch(cmds...)
}

// updateWelcome handles input on the welcome screen
func (m Model) updateWelcome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Space):
		m.screen = ScreenInput
		m.setFocus(0)
		return m, nil
	}
	return m, nil
}

// updateInput handles input on the input screen
func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch {
	case key.Matches(msg, m.keys.Escape):
		m.screen = ScreenWelcome
		return m, nil

	case key.Matches(msg, m.keys.Tab):
		m.nextInput()
		return m, nil

	case key.Matches(msg, m.keys.ShiftTab):
		m.prevInput()
		return m, nil

	case key.Matches(msg, m.keys.Browse), msg.String() == "ctrl+o", msg.String() == "ctrl+b", msg.Type == tea.KeyF2:
		// Open file picker for current field
		if m.focusIndex == int(FieldSourceFolder) {
			m.pickerTarget = PickerTargetSourceFolder
			configureFilePickerForSource(&m.filepicker, m.inputs[0].Value())
			m.filePickerActive = true
			m.previousScreen = ScreenInput
			m.screen = ScreenFilePicker
			return m, m.filepicker.Init()
		} else if m.focusIndex == int(FieldOutputFolder) {
			m.pickerTarget = PickerTargetOutputFolder
			configureFilePickerForOutput(&m.filepicker, m.inputs[2].Value())
			m.filePickerActive = true
			m.previousScreen = ScreenInput
			m.screen = ScreenFilePicker
			return m, m.filepicker.Init()
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		if m.focusIndex == int(FieldSubmitButton) {
			// Validate and start packaging
			valid, errMsg := m.ValidateInputs()
			if !valid {
				m.err = &validationError{message: errMsg}
				m.screen = ScreenError
				return m, nil
			}

			// Start packaging
			return m, startPackaging(
				m.GetSourceFolder(),
				m.GetSetupFile(),
				m.GetOutputFolder(),
			)
		}
		// Move to next field
		m.nextInput()
		return m, nil

	default:
		// Update the focused text input
		if m.focusIndex < len(m.inputs) {
			var cmd tea.Cmd
			m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
			cmds = append(cmds, cmd)

			// Auto-detect setup file when source folder changes
			if m.focusIndex == int(FieldSourceFolder) {
				sourceDir := m.inputs[0].Value()
				if sourceDir != "" {
					cmds = append(cmds, autoDetectSetupFileCmd(sourceDir))
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// updateFilePicker handles input on the file picker screen
func (m Model) updateFilePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle key messages for escape and selection
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(keyMsg, m.keys.Escape) {
			m.filePickerActive = false
			m.screen = m.previousScreen
			return m, nil
		}
	}

	// Update file picker with all message types (including internal readDirMsg)
	m.filepicker, cmd = m.filepicker.Update(msg)

	// Check if a path was selected (only for KeyMsg)
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if didSelect, path := m.filepicker.DidSelectFile(keyMsg); didSelect {
			m.filePickerActive = false
			m.screen = m.previousScreen

			// Set the selected path to the appropriate input
			switch m.pickerTarget {
			case PickerTargetSourceFolder:
				m.inputs[0].SetValue(path)
				m.setFocus(1) // Move to setup file field
				// Try to auto-detect setup file
				return m, autoDetectSetupFileCmd(path)
			case PickerTargetSetupFile:
				// Just use the filename, not the full path
				m.inputs[1].SetValue(filepath.Base(path))
				m.setFocus(2) // Move to output folder field
			case PickerTargetOutputFolder:
				m.inputs[2].SetValue(path)
				m.setFocus(3) // Move to submit button
			}
			return m, nil
		}

		// Check if a directory was selected (for folder pickers)
		if didSelect, path := m.filepicker.DidSelectDisabledFile(keyMsg); didSelect {
			// For directory-only pickers, treat this as selection
			if m.pickerTarget == PickerTargetSourceFolder || m.pickerTarget == PickerTargetOutputFolder {
				m.filePickerActive = false
				m.screen = m.previousScreen

				switch m.pickerTarget {
				case PickerTargetSourceFolder:
					m.inputs[0].SetValue(path)
					m.setFocus(1)
					return m, autoDetectSetupFileCmd(path)
				case PickerTargetOutputFolder:
					m.inputs[2].SetValue(path)
					m.setFocus(3)
				}
				return m, nil
			}
		}
	}

	return m, cmd
}

// updateProcessing handles input on the processing screen
func (m Model) updateProcessing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Only allow quit during processing (with confirmation would be nice)
	// For now, processing cannot be interrupted
	return m, nil
}

// updateSuccess handles input on the success screen
func (m Model) updateSuccess(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Enter):
		// Start new package
		m.resetForNewPackage()
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		return m, tea.Quit
	}
	return m, nil
}

// updateError handles input on the error screen
func (m Model) updateError(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Retry):
		// Retry packaging with same inputs
		valid, _ := m.ValidateInputs()
		if valid {
			m.err = nil
			return m, startPackaging(
				m.GetSourceFolder(),
				m.GetSetupFile(),
				m.GetOutputFolder(),
			)
		}
		// If inputs are invalid, go back to input screen
		m.err = nil
		m.screen = ScreenInput
		return m, nil

	case key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.Back):
		m.err = nil
		m.screen = ScreenInput
		return m, nil
	}
	return m, nil
}

// validationError represents an input validation error
type validationError struct {
	message string
}

func (e *validationError) Error() string {
	return e.message
}
