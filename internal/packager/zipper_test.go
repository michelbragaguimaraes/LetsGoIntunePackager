package packager

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Note: os and path/filepath are still used by other tests in this file

func TestZipFolder(t *testing.T) {
	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "ziptest")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFiles := map[string]string{
		"file1.txt":           "content of file 1",
		"file2.txt":           "content of file 2",
		"subdir/file3.txt":    "content of file 3",
		"subdir/sub2/deep.txt": "deep nested content",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Zip the folder
	zipData, err := ZipFolder(tempDir)
	if err != nil {
		t.Fatalf("ZipFolder() error = %v", err)
	}

	// Verify it's a valid ZIP
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("Failed to read ZIP: %v", err)
	}

	// Verify all files are present
	found := make(map[string]bool)
	for _, f := range reader.File {
		if !f.FileInfo().IsDir() {
			found[f.Name] = true
		}
	}

	for path := range testFiles {
		normalizedPath := filepath.ToSlash(path)
		if !found[normalizedPath] {
			t.Errorf("File %s not found in ZIP", normalizedPath)
		}
	}
}

func TestZipFolderWithProgress(t *testing.T) {
	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "ziptest")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	for i := 0; i < 5; i++ {
		filename := filepath.Join(tempDir, "file"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(filename, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Track progress calls
	var progressCalls []float64
	progressCallback := func(step string, percent float64) {
		progressCalls = append(progressCalls, percent)
	}

	// Zip with progress
	zipData, err := ZipFolderWithProgress(tempDir, progressCallback)
	if err != nil {
		t.Fatalf("ZipFolderWithProgress() error = %v", err)
	}

	// Verify progress was reported
	if len(progressCalls) == 0 {
		t.Error("No progress calls received")
	}

	// Verify ZIP is valid
	_, err = zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("Failed to read ZIP: %v", err)
	}
}

func TestZipFolderEmpty(t *testing.T) {
	// Create empty temporary directory
	tempDir, err := os.MkdirTemp("", "ziptest")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Zip the empty folder
	zipData, err := ZipFolder(tempDir)
	if err != nil {
		t.Fatalf("ZipFolder() error = %v", err)
	}

	// Verify it's a valid (empty) ZIP
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("Failed to read ZIP: %v", err)
	}

	if len(reader.File) != 0 {
		t.Errorf("Expected empty ZIP, got %d files", len(reader.File))
	}
}

func TestZipFolderNonExistent(t *testing.T) {
	_, err := ZipFolder("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

func TestCreateIntunewinPackage(t *testing.T) {
	// Create fake encrypted data and detection XML
	encryptedData := []byte("fake encrypted data")
	detectionXML := []byte("<?xml version=\"1.0\"?><ApplicationInfo/>")

	// Create package - returns bytes, not writing to file
	zipData, err := CreateIntunewinPackage(encryptedData, detectionXML)
	if err != nil {
		t.Fatalf("CreateIntunewinPackage() error = %v", err)
	}

	// Verify it's a valid ZIP
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("Output is not a valid ZIP: %v", err)
	}

	// Check for expected files
	expectedFiles := map[string]bool{
		"IntuneWinPackage/Contents/IntunePackage.intunewin": false,
		"IntuneWinPackage/Metadata/Detection.xml":           false,
	}

	for _, f := range reader.File {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
		}
	}

	for file, found := range expectedFiles {
		if !found {
			t.Errorf("Expected file %s not found in package", file)
		}
	}
}
