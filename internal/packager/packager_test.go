package packager

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPackage(t *testing.T) {
	// Create source directory with test files
	sourceDir, err := os.MkdirTemp("", "source")
	if err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	defer os.RemoveAll(sourceDir)

	// Create output directory
	outputDir, err := os.MkdirTemp("", "output")
	if err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Create a test setup file
	setupContent := []byte("fake installer content for testing")
	setupFile := "setup.exe"
	if err := os.WriteFile(filepath.Join(sourceDir, setupFile), setupContent, 0644); err != nil {
		t.Fatalf("Failed to write setup file: %v", err)
	}

	// Create additional test files
	if err := os.WriteFile(filepath.Join(sourceDir, "readme.txt"), []byte("readme content"), 0644); err != nil {
		t.Fatalf("Failed to write readme file: %v", err)
	}

	// Track progress
	var progressSteps []string
	progressCallback := func(step string, percent float64) {
		progressSteps = append(progressSteps, step)
	}

	// Run packager
	result, err := Package(sourceDir, setupFile, outputDir, progressCallback)
	if err != nil {
		t.Fatalf("Package() error = %v", err)
	}

	// Verify result
	if result == nil {
		t.Fatal("Result is nil")
	}
	if result.OutputPath == "" {
		t.Error("OutputPath is empty")
	}
	if result.FileCount == 0 {
		t.Error("FileCount is 0")
	}
	if result.SourceSize == 0 {
		t.Error("SourceSize is 0")
	}
	if result.FinalSize == 0 {
		t.Error("FinalSize is 0")
	}

	// Verify output file exists
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Error("Output file does not exist")
	}

	// Verify output is valid ZIP with correct structure
	zipData, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("Output is not a valid ZIP: %v", err)
	}

	// Check for expected structure
	hasContents := false
	hasDetection := false
	for _, f := range reader.File {
		if f.Name == "IntuneWinPackage/Contents/IntunePackage.intunewin" {
			hasContents = true
		}
		if f.Name == "IntuneWinPackage/Metadata/Detection.xml" {
			hasDetection = true
		}
	}

	if !hasContents {
		t.Error("Missing IntuneWinPackage/Contents/IntunePackage.intunewin")
	}
	if !hasDetection {
		t.Error("Missing IntuneWinPackage/Metadata/Detection.xml")
	}

	// Verify progress was reported
	if len(progressSteps) == 0 {
		t.Error("No progress updates received")
	}
}

func TestPackageInvalidSource(t *testing.T) {
	outputDir, err := os.MkdirTemp("", "output")
	if err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	_, err = Package("/nonexistent/path", "setup.exe", outputDir, nil)
	if err == nil {
		t.Error("Expected error for non-existent source")
	}
}

func TestPackageMissingSetupFile(t *testing.T) {
	sourceDir, err := os.MkdirTemp("", "source")
	if err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	defer os.RemoveAll(sourceDir)

	outputDir, err := os.MkdirTemp("", "output")
	if err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Create source dir but no setup file
	if err := os.WriteFile(filepath.Join(sourceDir, "other.txt"), []byte("other"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err = Package(sourceDir, "setup.exe", outputDir, nil)
	if err == nil {
		t.Error("Expected error for missing setup file")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0 bytes"},
		{100, "100 bytes"},
		{1023, "1023 bytes"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1572864, "1.50 MB"},
		{1073741824, "1.00 GB"},
		{1610612736, "1.50 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatSize(tt.size)
			if result != tt.expected {
				t.Errorf("FormatSize(%d) = %s, want %s", tt.size, result, tt.expected)
			}
		})
	}
}

func TestPackageWithSubdirectories(t *testing.T) {
	// Create source directory with subdirectories
	sourceDir, err := os.MkdirTemp("", "source")
	if err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	defer os.RemoveAll(sourceDir)

	// Create output directory
	outputDir, err := os.MkdirTemp("", "output")
	if err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Create setup file
	setupFile := "setup.exe"
	if err := os.WriteFile(filepath.Join(sourceDir, setupFile), []byte("installer"), 0644); err != nil {
		t.Fatalf("Failed to write setup file: %v", err)
	}

	// Create subdirectory with files
	subdir := filepath.Join(sourceDir, "data", "config")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "settings.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Run packager
	result, err := Package(sourceDir, setupFile, outputDir, nil)
	if err != nil {
		t.Fatalf("Package() error = %v", err)
	}

	// Verify file count includes all files
	if result.FileCount < 2 {
		t.Errorf("FileCount = %d, expected at least 2", result.FileCount)
	}
}

func TestPackageNilProgressCallback(t *testing.T) {
	// Create source directory
	sourceDir, err := os.MkdirTemp("", "source")
	if err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	defer os.RemoveAll(sourceDir)

	// Create output directory
	outputDir, err := os.MkdirTemp("", "output")
	if err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Create setup file
	setupFile := "setup.exe"
	if err := os.WriteFile(filepath.Join(sourceDir, setupFile), []byte("installer"), 0644); err != nil {
		t.Fatalf("Failed to write setup file: %v", err)
	}

	// Run packager with nil callback - should not panic
	result, err := Package(sourceDir, setupFile, outputDir, nil)
	if err != nil {
		t.Fatalf("Package() error = %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}
}
