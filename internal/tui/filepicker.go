package tui

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/filepicker"
)

// newFilePicker creates a new file picker configured for directory selection
func newFilePicker(dirOnly bool) filepicker.Model {
	fp := filepicker.New()

	// Get current working directory as starting point
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "/"
	}

	fp.CurrentDirectory = cwd
	fp.ShowHidden = false
	fp.ShowPermissions = false
	fp.DirAllowed = true
	fp.FileAllowed = !dirOnly

	if !dirOnly {
		// Allow common installer file types
		fp.AllowedTypes = []string{".msi", ".exe", ".ps1", ".cmd", ".bat"}
	}

	// Set height for better visibility
	fp.Height = 15

	return fp
}

// configureFilePickerForSource configures file picker for source folder selection
func configureFilePickerForSource(fp *filepicker.Model, currentValue string) {
	fp.DirAllowed = true
	fp.FileAllowed = true // Allow files to be visible for navigation
	fp.AllowedTypes = []string{}

	// Set starting directory
	if currentValue != "" {
		if info, err := os.Stat(currentValue); err == nil && info.IsDir() {
			fp.CurrentDirectory = currentValue
		} else {
			fp.CurrentDirectory = filepath.Dir(currentValue)
		}
	} else {
		// Default to current working directory or home
		if cwd, err := os.Getwd(); err == nil {
			fp.CurrentDirectory = cwd
		} else if home, err := os.UserHomeDir(); err == nil {
			fp.CurrentDirectory = home
		} else {
			fp.CurrentDirectory = "/"
		}
	}
}

// configureFilePickerForOutput configures file picker for output folder selection
func configureFilePickerForOutput(fp *filepicker.Model, currentValue string) {
	fp.DirAllowed = true
	fp.FileAllowed = true // Allow files to be visible for navigation
	fp.AllowedTypes = []string{}

	// Set starting directory
	if currentValue != "" {
		if info, err := os.Stat(currentValue); err == nil && info.IsDir() {
			fp.CurrentDirectory = currentValue
		} else {
			fp.CurrentDirectory = filepath.Dir(currentValue)
		}
	} else {
		// Default to current working directory or home
		if cwd, err := os.Getwd(); err == nil {
			fp.CurrentDirectory = cwd
		} else if home, err := os.UserHomeDir(); err == nil {
			fp.CurrentDirectory = home
		} else {
			fp.CurrentDirectory = "/"
		}
	}
}

// configureFilePickerForSetupFile configures file picker for setup file selection
func configureFilePickerForSetupFile(fp *filepicker.Model, sourceFolder string) {
	fp.DirAllowed = false
	fp.FileAllowed = true
	fp.AllowedTypes = []string{".msi", ".exe", ".ps1", ".cmd", ".bat"}

	// Start in source folder if available
	if sourceFolder != "" {
		if info, err := os.Stat(sourceFolder); err == nil && info.IsDir() {
			fp.CurrentDirectory = sourceFolder
		}
	}
}

// getFilePickerTitle returns the title for the file picker based on target
func getFilePickerTitle(target FilePickerTarget) string {
	switch target {
	case PickerTargetSourceFolder:
		return "Select Source Folder"
	case PickerTargetSetupFile:
		return "Select Setup File"
	case PickerTargetOutputFolder:
		return "Select Output Folder"
	default:
		return "Select"
	}
}

// getFilePickerHelp returns help text for the file picker based on target
func getFilePickerHelp(target FilePickerTarget) string {
	switch target {
	case PickerTargetSourceFolder:
		return "Navigate to and select the folder containing your setup file"
	case PickerTargetSetupFile:
		return "Navigate to and select the setup file (.msi, .exe, .ps1, .cmd, .bat)"
	case PickerTargetOutputFolder:
		return "Navigate to and select the folder where the .intunewin file will be created"
	default:
		return "Navigate and select"
	}
}

// validatePath checks if a path exists and matches expected type
func validatePath(path string, expectDir bool) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if expectDir {
		return info.IsDir()
	}
	return !info.IsDir()
}

// listSetupFiles lists potential setup files in a directory
func listSetupFiles(dir string) ([]string, error) {
	var files []string
	extensions := map[string]bool{
		".msi": true,
		".exe": true,
		".ps1": true,
		".cmd": true,
		".bat": true,
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if extensions[ext] || extensions["."+ext] {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// autoDetectSetupFile attempts to find the main setup file in a directory
// It looks for common installer patterns
func autoDetectSetupFile(dir string) string {
	files, err := listSetupFiles(dir)
	if err != nil || len(files) == 0 {
		return ""
	}

	// Priority patterns for setup file detection
	patterns := []string{
		"setup.msi",
		"setup.exe",
		"install.msi",
		"install.exe",
		"installer.msi",
		"installer.exe",
	}

	// Check for priority patterns first
	for _, pattern := range patterns {
		for _, file := range files {
			if filepath.Base(file) == pattern {
				return file
			}
		}
	}

	// Prefer MSI over EXE
	for _, file := range files {
		if filepath.Ext(file) == ".msi" {
			return file
		}
	}

	// Return first file found
	if len(files) > 0 {
		return files[0]
	}

	return ""
}
