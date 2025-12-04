package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/michelbragaguimaraes/LetsGoIntunePackager/internal/packager"
)

// View renders the current screen
func (m Model) View() string {
	switch m.screen {
	case ScreenWelcome:
		return m.viewWelcome()
	case ScreenInput:
		return m.viewInput()
	case ScreenFilePicker:
		return m.viewFilePicker()
	case ScreenProcessing:
		return m.viewProcessing()
	case ScreenSuccess:
		return m.viewSuccess()
	case ScreenError:
		return m.viewError()
	default:
		return "Unknown screen"
	}
}

// viewWelcome renders the welcome screen
func (m Model) viewWelcome() string {
	var b strings.Builder

	// Logo
	b.WriteString(LogoStyle.Render(Logo))
	b.WriteString("\n\n")

	// Description
	desc := lipgloss.NewStyle().
		Foreground(dimTextColor).
		Render("Create .intunewin packages for Microsoft Intune Win32 app deployment")
	b.WriteString(desc)
	b.WriteString("\n\n")

	// Instructions
	instructions := BoxStyle.Render(
		TitleStyle.Render("Getting Started") + "\n\n" +
			"This tool will package your application installer into the\n" +
			".intunewin format required by Microsoft Intune.\n\n" +
			"You will need:\n" +
			"  • Source folder containing your setup file\n" +
			"  • Setup file name (e.g., setup.msi or install.exe)\n" +
			"  • Output folder for the .intunewin file",
	)
	b.WriteString(instructions)
	b.WriteString("\n\n")

	// Help
	b.WriteString(renderHelp(WelcomeKeyMap()))

	return AppStyle.Render(b.String())
}

// viewInput renders the input screen
func (m Model) viewInput() string {
	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render("📦 Create Intune Package"))
	b.WriteString("\n\n")

	// Source folder input
	b.WriteString(m.inputLabelStyle(0).Render("Source Folder"))
	b.WriteString("\n")
	b.WriteString(m.inputStyle(0).Render(m.inputs[0].View()))
	if m.focusIndex == 0 {
		b.WriteString("  ")
		b.WriteString(DimStyle.Render("(Ctrl+O to browse)"))
	}
	b.WriteString("\n\n")

	// Setup file input
	b.WriteString(m.inputLabelStyle(1).Render("Setup File"))
	b.WriteString("\n")
	b.WriteString(m.inputStyle(1).Render(m.inputs[1].View()))
	b.WriteString("\n\n")

	// Output folder input
	b.WriteString(m.inputLabelStyle(2).Render("Output Folder"))
	b.WriteString("\n")
	b.WriteString(m.inputStyle(2).Render(m.inputs[2].View()))
	if m.focusIndex == 2 {
		b.WriteString("  ")
		b.WriteString(DimStyle.Render("(Ctrl+O to browse)"))
	}
	b.WriteString("\n\n")

	// Submit button
	buttonText := "  Create Package  "
	if m.focusIndex == int(FieldSubmitButton) {
		b.WriteString(ButtonFocusedStyle.Render(buttonText))
	} else {
		b.WriteString(ButtonStyle.Render(buttonText))
	}
	b.WriteString("\n\n")

	// Help
	b.WriteString(renderHelp(InputKeyMap()))

	return AppStyle.Render(b.String())
}

// viewFilePicker renders the file picker screen
func (m Model) viewFilePicker() string {
	var b strings.Builder

	// Title
	title := getFilePickerTitle(m.pickerTarget)
	b.WriteString(TitleStyle.Render("📁 " + title))
	b.WriteString("\n")

	// Help text
	help := getFilePickerHelp(m.pickerTarget)
	b.WriteString(DimStyle.Render(help))
	b.WriteString("\n\n")

	// Current directory
	b.WriteString(DimStyle.Render("Current: "))
	b.WriteString(lipgloss.NewStyle().Foreground(primaryColor).Render(m.filepicker.CurrentDirectory))
	b.WriteString("\n\n")

	// File picker
	b.WriteString(FilePickerStyle.Render(m.filepicker.View()))
	b.WriteString("\n\n")

	// Help
	b.WriteString(renderHelp(FilePickerKeyMap()))

	return AppStyle.Render(b.String())
}

// viewProcessing renders the processing screen
func (m Model) viewProcessing() string {
	var b strings.Builder

	// Title with spinner
	b.WriteString(TitleStyle.Render("📦 Creating Package"))
	b.WriteString("\n\n")

	// Progress info
	b.WriteString(m.spinner.View())
	b.WriteString(" ")
	b.WriteString(m.progressStep)
	b.WriteString("\n\n")

	// Progress bar
	b.WriteString(renderProgressBar(m.progress, 40))
	b.WriteString("\n")
	b.WriteString(ProgressTextStyle.Render(fmt.Sprintf("%.0f%%", m.progress*100)))
	b.WriteString("\n\n")

	// Processing log (last few steps)
	if len(m.processingLog) > 0 {
		b.WriteString(DimStyle.Render("Recent activity:"))
		b.WriteString("\n")
		for _, entry := range m.processingLog {
			b.WriteString(DimStyle.Render("  • " + entry))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	// Help
	b.WriteString(renderHelp(ProcessingKeyMap()))

	return AppStyle.Render(b.String())
}

// viewSuccess renders the success screen
func (m Model) viewSuccess() string {
	var b strings.Builder

	// Title
	b.WriteString(SuccessStyle.Render("✓ Package Created Successfully!"))
	b.WriteString("\n\n")

	// Result details
	if m.result != nil {
		resultBox := ResultBoxStyle.Render(
			StatLabelStyle.Render("Output File:") + " " + StatValueStyle.Render(m.result.OutputPath) + "\n" +
				StatLabelStyle.Render("Files Packaged:") + " " + StatValueStyle.Render(fmt.Sprintf("%d", m.result.FileCount)) + "\n" +
				StatLabelStyle.Render("Source Size:") + " " + StatValueStyle.Render(packager.FormatSize(m.result.SourceSize)) + "\n" +
				StatLabelStyle.Render("Final Size:") + " " + StatValueStyle.Render(packager.FormatSize(m.result.FinalSize)),
		)
		b.WriteString(resultBox)
		b.WriteString("\n\n")

		// Compression ratio
		if m.result.SourceSize > 0 {
			ratio := float64(m.result.FinalSize) / float64(m.result.SourceSize) * 100
			b.WriteString(DimStyle.Render(fmt.Sprintf("Compression ratio: %.1f%%", ratio)))
			b.WriteString("\n\n")
		}
	}

	// Next steps
	nextSteps := BoxStyle.Render(
		SubtitleStyle.Render("Next Steps") + "\n\n" +
			"1. Upload the .intunewin file to Microsoft Intune\n" +
			"2. Configure detection rules and requirements\n" +
			"3. Assign the app to users or devices",
	)
	b.WriteString(nextSteps)
	b.WriteString("\n\n")

	// Help
	b.WriteString(renderHelp(SuccessKeyMap()))

	return AppStyle.Render(b.String())
}

// viewError renders the error screen
func (m Model) viewError() string {
	var b strings.Builder

	// Title
	b.WriteString(ErrorStyle.Render("✗ Error"))
	b.WriteString("\n\n")

	// Error message
	if m.err != nil {
		errorBox := ErrorBoxStyle.Render(m.err.Error())
		b.WriteString(errorBox)
		b.WriteString("\n\n")
	}

	// Suggestions
	suggestions := BoxStyle.Render(
		SubtitleStyle.Render("Troubleshooting") + "\n\n" +
			"• Check that the source folder exists and is accessible\n" +
			"• Verify the setup file name is correct\n" +
			"• Ensure you have write permissions to the output folder\n" +
			"• Make sure no other process is using the files",
	)
	b.WriteString(suggestions)
	b.WriteString("\n\n")

	// Help
	b.WriteString(renderHelp(ErrorKeyMap()))

	return AppStyle.Render(b.String())
}

// renderProgressBar renders a simple progress bar
func renderProgressBar(progress float64, width int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	filled := int(progress * float64(width))
	empty := width - filled

	filledStr := strings.Repeat("█", filled)
	emptyStr := strings.Repeat("░", empty)

	return ProgressBarStyle.Render(filledStr) + MutedStyle.Render(emptyStr)
}

// renderHelp renders the help bar with key bindings
func renderHelp(keys []key.Binding) string {
	var parts []string
	for _, k := range keys {
		help := k.Help()
		keyStr := HelpKeyStyle.Render(help.Key)
		descStr := HelpDescStyle.Render(help.Desc)
		parts = append(parts, keyStr+" "+descStr)
	}
	return HelpStyle.Render(strings.Join(parts, "  •  "))
}
