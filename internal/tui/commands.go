package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/michelbragaguimaraes/LetsGoIntunePackager/internal/packager"
)

// Message types for async operations

// packageStartMsg signals that packaging has started
type packageStartMsg struct{}

// packageProgressMsg carries progress updates
type packageProgressMsg struct {
	step    string
	percent float64
}

// packageCompleteMsg carries the successful result
type packageCompleteMsg struct {
	result *packager.PackageResult
}

// packageErrorMsg carries error information
type packageErrorMsg struct {
	err error
}

// Global program reference for sending messages from goroutines
var program *tea.Program

// SetProgram sets the global program reference
// This must be called before starting any packaging operations
func SetProgram(p *tea.Program) {
	program = p
}

// startPackaging initiates the packaging process asynchronously
func startPackaging(sourcePath, setupFile, outputPath string) tea.Cmd {
	return func() tea.Msg {
		// Start the packaging in a goroutine
		go func() {
			result, err := packager.Package(sourcePath, setupFile, outputPath,
				func(step string, pct float64) {
					// Send progress updates back to the TUI
					if program != nil {
						program.Send(packageProgressMsg{
							step:    step,
							percent: pct,
						})
					}
				})

			// Send final result
			if program != nil {
				if err != nil {
					program.Send(packageErrorMsg{err: err})
				} else {
					program.Send(packageCompleteMsg{result: result})
				}
			}
		}()

		return packageStartMsg{}
	}
}

// clearInputCmd returns a command that does nothing (placeholder)
func clearInputCmd() tea.Cmd {
	return nil
}

// autoDetectSetupFileCmd tries to detect a setup file in the source directory
func autoDetectSetupFileCmd(sourceDir string) tea.Cmd {
	return func() tea.Msg {
		setupFile := autoDetectSetupFile(sourceDir)
		if setupFile != "" {
			return setupFileDetectedMsg{filename: setupFile}
		}
		return nil
	}
}

// setupFileDetectedMsg signals that a setup file was auto-detected
type setupFileDetectedMsg struct {
	filename string
}

// validatePathCmd validates a path asynchronously
func validatePathCmd(path string, isDir bool) tea.Cmd {
	return func() tea.Msg {
		valid := validatePath(path, isDir)
		return pathValidatedMsg{
			path:  path,
			isDir: isDir,
			valid: valid,
		}
	}
}

// pathValidatedMsg carries path validation result
type pathValidatedMsg struct {
	path  string
	isDir bool
	valid bool
}

// listSetupFilesCmd lists potential setup files in a directory
func listSetupFilesCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		files, err := listSetupFiles(dir)
		return setupFilesListedMsg{
			files: files,
			err:   err,
		}
	}
}

// setupFilesListedMsg carries the list of setup files found
type setupFilesListedMsg struct {
	files []string
	err   error
}
