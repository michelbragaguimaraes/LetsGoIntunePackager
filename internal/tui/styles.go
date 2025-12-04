package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette
var (
	primaryColor   = lipgloss.Color("#7D56F4") // Purple
	secondaryColor = lipgloss.Color("#874BFD") // Light purple
	successColor   = lipgloss.Color("#04B575") // Green
	errorColor     = lipgloss.Color("#FF4672") // Red/Pink
	warningColor   = lipgloss.Color("#FFCC00") // Yellow
	mutedColor     = lipgloss.Color("#626262") // Gray
	textColor      = lipgloss.Color("#FAFAFA") // White
	dimTextColor   = lipgloss.Color("#A0A0A0") // Light gray
)

// Layout styles
var (
	// AppStyle is the main container style
	AppStyle = lipgloss.NewStyle().
			Padding(1, 2)

	// TitleStyle is for the main title/header
	TitleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1)

	// SubtitleStyle is for secondary headers
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			MarginBottom(1)

	// BoxStyle is for bordered containers
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)
)

// Input styles
var (
	// InputStyle is for text inputs
	InputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(mutedColor).
			Padding(0, 1)

	// InputFocusedStyle is for focused text inputs
	InputFocusedStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1)

	// InputLabelStyle is for input labels
	InputLabelStyle = lipgloss.NewStyle().
			Foreground(dimTextColor).
			MarginBottom(0)

	// InputLabelFocusedStyle is for focused input labels
	InputLabelFocusedStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				MarginBottom(0)
)

// Button styles
var (
	// ButtonStyle is for unfocused buttons
	ButtonStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(mutedColor).
			Padding(0, 2).
			MarginRight(1)

	// ButtonFocusedStyle is for focused buttons
	ButtonFocusedStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Background(primaryColor).
				Padding(0, 2).
				MarginRight(1)

	// ButtonSuccessStyle is for success/confirm buttons
	ButtonSuccessStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Background(successColor).
				Padding(0, 2).
				MarginRight(1)
)

// Status styles
var (
	// SuccessStyle is for success messages
	SuccessStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	// ErrorStyle is for error messages
	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// WarningStyle is for warning messages
	WarningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	// MutedStyle is for muted/secondary text
	MutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// DimStyle is for dim text
	DimStyle = lipgloss.NewStyle().
			Foreground(dimTextColor)
)

// Progress styles
var (
	// SpinnerStyle is for the loading spinner
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	// ProgressBarStyle is for progress bar styling
	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(primaryColor)

	// ProgressTextStyle is for progress text
	ProgressTextStyle = lipgloss.NewStyle().
				Foreground(dimTextColor)

	// CheckmarkStyle is for completed step checkmarks
	CheckmarkStyle = lipgloss.NewStyle().
			Foreground(successColor).
			SetString("✓")

	// PendingStyle is for pending step markers
	PendingStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			SetString("○")

	// ActiveStyle is for active step markers
	ActiveStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			SetString("●")
)

// Help styles
var (
	// HelpStyle is for the help bar at the bottom
	HelpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	// HelpKeyStyle is for key bindings in help
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	// HelpDescStyle is for descriptions in help
	HelpDescStyle = lipgloss.NewStyle().
			Foreground(dimTextColor)
)

// File picker styles
var (
	// FilePickerStyle is for the file picker container
	FilePickerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	// FilePickerSelectedStyle is for selected items in file picker
	FilePickerSelectedStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	// FilePickerDirStyle is for directories in file picker
	FilePickerDirStyle = lipgloss.NewStyle().
				Foreground(secondaryColor)

	// FilePickerFileStyle is for files in file picker
	FilePickerFileStyle = lipgloss.NewStyle().
				Foreground(textColor)
)

// Result styles
var (
	// ResultBoxStyle is for result containers
	ResultBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(successColor).
			Padding(1, 2).
			MarginTop(1)

	// ErrorBoxStyle is for error containers
	ErrorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(errorColor).
			Padding(1, 2).
			MarginTop(1)

	// StatLabelStyle is for stat labels
	StatLabelStyle = lipgloss.NewStyle().
			Foreground(dimTextColor).
			Width(20)

	// StatValueStyle is for stat values
	StatValueStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Bold(true)
)

// Logo is the ASCII art logo for the application
const Logo = `
██╗     ███████╗████████╗███████╗ ██████╗  ██████╗ ██╗
██║     ██╔════╝╚══██╔══╝██╔════╝██╔════╝ ██╔═══██╗██║
██║     █████╗     ██║   ███████╗██║  ███╗██║   ██║██║
██║     ██╔══╝     ██║   ╚════██║██║   ██║██║   ██║╚═╝
███████╗███████╗   ██║   ███████║╚██████╔╝╚██████╔╝██╗
╚══════╝╚══════╝   ╚═╝   ╚══════╝ ╚═════╝  ╚═════╝ ╚═╝
    ████  Intune Packager
    ████  By Mike Guimaraes - michelbragaguimaraes@gmail.com
`

// LogoStyle styles the ASCII logo
var LogoStyle = lipgloss.NewStyle().
	Foreground(primaryColor).
	Bold(true)
