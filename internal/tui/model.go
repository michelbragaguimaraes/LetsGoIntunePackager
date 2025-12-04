package tui

import (
	"os"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/michelbragaguimaraes/LetsGoIntunePackager/internal/packager"
)

// Screen represents the current screen state
type Screen int

const (
	ScreenWelcome Screen = iota
	ScreenInput
	ScreenFilePicker
	ScreenProcessing
	ScreenSuccess
	ScreenError
)

// FilePickerTarget indicates which input field the file picker is for
type FilePickerTarget int

const (
	PickerTargetSourceFolder FilePickerTarget = iota
	PickerTargetSetupFile
	PickerTargetOutputFolder
)

// InputField represents the focused input field
type InputField int

const (
	FieldSourceFolder InputField = iota
	FieldSetupFile
	FieldOutputFolder
	FieldSubmitButton
)

const numInputFields = 4

// Model is the main application state
type Model struct {
	// Screen management
	screen         Screen
	previousScreen Screen

	// Window dimensions
	width  int
	height int

	// Input fields
	inputs      []textinput.Model
	focusIndex  int

	// File picker
	filepicker       filepicker.Model
	filePickerActive bool
	pickerTarget     FilePickerTarget

	// Processing state
	spinner       spinner.Model
	progress      float64
	progressStep  string
	processingLog []string

	// Results
	result *packager.PackageResult
	err    error

	// Key bindings
	keys KeyMap

	// Presets from CLI flags
	presets *Presets
}

// Presets holds values passed from CLI flags
type Presets struct {
	ContentPath string
	SetupFile   string
	OutputPath  string
}

// NewModel creates a new Model with initial state
func NewModel(presets *Presets) Model {
	// Initialize text inputs
	inputs := make([]textinput.Model, 3)

	// Source folder input
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "/path/to/source/folder"
	inputs[0].CharLimit = 500
	inputs[0].Width = 50

	// Setup file input
	inputs[1] = textinput.New()
	inputs[1].Placeholder = "setup.msi or install.exe"
	inputs[1].CharLimit = 256
	inputs[1].Width = 50

	// Output folder input
	inputs[2] = textinput.New()
	inputs[2].Placeholder = "/path/to/output/folder"
	inputs[2].CharLimit = 500
	inputs[2].Width = 50

	// Apply presets if provided
	if presets != nil {
		if presets.ContentPath != "" {
			inputs[0].SetValue(presets.ContentPath)
		}
		if presets.SetupFile != "" {
			inputs[1].SetValue(presets.SetupFile)
		}
		if presets.OutputPath != "" {
			inputs[2].SetValue(presets.OutputPath)
		}
	}

	// Focus first empty input or first input
	focusIdx := 0
	for i, input := range inputs {
		if input.Value() == "" {
			focusIdx = i
			break
		}
	}
	inputs[focusIdx].Focus()

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	// Initialize file picker
	fp := filepicker.New()
	fp.AllowedTypes = []string{} // Allow all for directories
	fp.ShowHidden = false
	fp.DirAllowed = true
	fp.FileAllowed = true // Allow files to be visible for navigation
	fp.Height = 15

	// Set starting directory to current working directory
	if cwd, err := os.Getwd(); err == nil {
		fp.CurrentDirectory = cwd
	} else if home, err := os.UserHomeDir(); err == nil {
		fp.CurrentDirectory = home
	} else {
		fp.CurrentDirectory = "/"
	}

	// Set filepicker styles
	fp.Styles.Cursor = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)
	fp.Styles.Selected = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)
	fp.Styles.Directory = lipgloss.NewStyle().Foreground(secondaryColor)
	fp.Styles.File = lipgloss.NewStyle()
	fp.Styles.Symlink = lipgloss.NewStyle().Foreground(lipgloss.Color("135"))
	fp.Styles.DisabledCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	fp.Styles.DisabledFile = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	fp.Styles.DisabledSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	return Model{
		screen:        ScreenWelcome,
		inputs:        inputs,
		focusIndex:    focusIdx,
		spinner:       s,
		filepicker:    fp,
		keys:          DefaultKeyMap,
		presets:       presets,
		processingLog: make([]string, 0),
	}
}

// GetSourceFolder returns the source folder value
func (m Model) GetSourceFolder() string {
	return m.inputs[0].Value()
}

// GetSetupFile returns the setup file value
func (m Model) GetSetupFile() string {
	return m.inputs[1].Value()
}

// GetOutputFolder returns the output folder value
func (m Model) GetOutputFolder() string {
	return m.inputs[2].Value()
}

// SetProgress updates the progress state
func (m *Model) SetProgress(step string, percent float64) {
	m.progressStep = step
	m.progress = percent
	if step != "" && step != "Complete" {
		m.processingLog = append(m.processingLog, step)
		// Keep only last 5 log entries
		if len(m.processingLog) > 5 {
			m.processingLog = m.processingLog[len(m.processingLog)-5:]
		}
	}
}

// ValidateInputs checks if all required inputs are filled
func (m Model) ValidateInputs() (bool, string) {
	if m.inputs[0].Value() == "" {
		return false, "Source folder is required"
	}
	if m.inputs[1].Value() == "" {
		return false, "Setup file is required"
	}
	if m.inputs[2].Value() == "" {
		return false, "Output folder is required"
	}
	return true, ""
}

// nextInput moves focus to the next input field
func (m *Model) nextInput() {
	if m.focusIndex < len(m.inputs) {
		m.inputs[m.focusIndex].Blur()
	}
	m.focusIndex = (m.focusIndex + 1) % numInputFields
	if m.focusIndex < len(m.inputs) {
		m.inputs[m.focusIndex].Focus()
	}
}

// prevInput moves focus to the previous input field
func (m *Model) prevInput() {
	if m.focusIndex < len(m.inputs) {
		m.inputs[m.focusIndex].Blur()
	}
	m.focusIndex--
	if m.focusIndex < 0 {
		m.focusIndex = numInputFields - 1
	}
	if m.focusIndex < len(m.inputs) {
		m.inputs[m.focusIndex].Focus()
	}
}

// setFocus sets focus to a specific input
func (m *Model) setFocus(idx int) {
	if m.focusIndex < len(m.inputs) {
		m.inputs[m.focusIndex].Blur()
	}
	m.focusIndex = idx
	if idx < len(m.inputs) {
		m.inputs[idx].Focus()
	}
}

// inputLabelStyle returns the style for an input label based on focus state
func (m Model) inputLabelStyle(idx int) lipgloss.Style {
	if m.focusIndex == idx {
		return InputLabelFocusedStyle
	}
	return InputLabelStyle
}

// inputStyle returns the style for an input based on focus state
func (m Model) inputStyle(idx int) lipgloss.Style {
	if m.focusIndex == idx {
		return InputFocusedStyle
	}
	return InputStyle
}

// buttonStyle returns the style for a button based on focus state
func (m Model) buttonStyle(idx int) lipgloss.Style {
	if m.focusIndex == idx {
		return ButtonFocusedStyle
	}
	return ButtonStyle
}

// resetForNewPackage resets the model state for creating a new package
func (m *Model) resetForNewPackage() {
	m.screen = ScreenInput
	m.result = nil
	m.err = nil
	m.progress = 0
	m.progressStep = ""
	m.processingLog = make([]string, 0)

	// Clear inputs
	for i := range m.inputs {
		m.inputs[i].SetValue("")
	}
	m.setFocus(0)
}
