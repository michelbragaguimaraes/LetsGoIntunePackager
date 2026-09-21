package packager

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"io"
	"math/rand"
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

// readPackage opens a .intunewin, decrypts the payload with the keys from its
// own Detection.xml, and returns the inner ZIP plus the parsed metadata.
func readPackage(t *testing.T, path string) ([]byte, *ApplicationInfo) {
	t.Helper()

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}
	defer zr.Close()

	var blob, metadata []byte
	for _, f := range zr.File {
		if f.Method != zip.Store {
			t.Errorf("%s uses method %d, want Store", f.Name, f.Method)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("Open(%s) error = %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("ReadAll(%s) error = %v", f.Name, err)
		}
		switch f.Name {
		case "IntuneWinPackage/Contents/IntunePackage.intunewin":
			blob = data
		case "IntuneWinPackage/Metadata/Detection.xml":
			metadata = data
		default:
			t.Errorf("unexpected entry %q", f.Name)
		}
	}
	if blob == nil || metadata == nil {
		t.Fatal("package is missing the content or metadata entry")
	}

	var info ApplicationInfo
	if err := xml.Unmarshal(metadata, &info); err != nil {
		t.Fatalf("Unmarshal(Detection.xml) error = %v", err)
	}

	decode := func(s string) []byte {
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			t.Fatalf("bad base64 in Detection.xml: %v", err)
		}
		return b
	}
	plain, err := DecryptContent(blob, decode(info.EncryptionInfo.EncryptionKey), decode(info.EncryptionInfo.MacKey))
	if err != nil {
		t.Fatalf("DecryptContent() error = %v", err)
	}

	if got := sha256.Sum256(plain); !bytes.Equal(got[:], decode(info.EncryptionInfo.FileDigest)) {
		t.Error("FileDigest does not match SHA256 of the decrypted payload")
	}
	if info.UnencryptedContentSize != int64(len(plain)) {
		t.Errorf("UnencryptedContentSize = %d, want %d", info.UnencryptedContentSize, len(plain))
	}
	return plain, &info
}

func TestPackageRoundTrip(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := t.TempDir()

	// A payload large enough to cross the 1 MiB ciphertext chunk, plus a
	// nested directory, so the streaming path is exercised properly.
	big := make([]byte, cbcChunkSize+1234)
	rng := rand.New(rand.NewSource(7))
	rng.Read(big)

	want := map[string][]byte{
		"setup.exe":         []byte("installer payload"),
		"readme.txt":        []byte("notes"),
		"sub/data.bin":      big,
		"sub/deep/tiny.txt": []byte("x"),
	}
	for name, content := range want {
		full := filepath.Join(sourceDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, content, 0644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := Package(sourceDir, "setup.exe", outputDir, nil)
	if err != nil {
		t.Fatalf("Package() error = %v", err)
	}

	plain, info := readPackage(t, result.OutputPath)
	if info.SetupFile != "setup.exe" {
		t.Errorf("SetupFile = %q, want %q", info.SetupFile, "setup.exe")
	}
	if result.ZipSize != int64(len(plain)) {
		t.Errorf("ZipSize = %d, want %d", result.ZipSize, len(plain))
	}
	if result.FileCount != len(want) {
		t.Errorf("FileCount = %d, want %d", result.FileCount, len(want))
	}

	inner, err := zip.NewReader(bytes.NewReader(plain), int64(len(plain)))
	if err != nil {
		t.Fatalf("inner zip unreadable: %v", err)
	}
	got := map[string][]byte{}
	for _, f := range inner.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		got[f.Name] = data
	}

	if len(got) != len(want) {
		t.Errorf("payload holds %d files, want %d", len(got), len(want))
	}
	for name, content := range want {
		if !bytes.Equal(got[name], content) {
			t.Errorf("%s: payload content differs (%d bytes vs %d)", name, len(got[name]), len(content))
		}
	}
}

func TestPackageLeavesNoScratchFiles(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "setup.exe"), []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Package(sourceDir, "setup.exe", outputDir, nil); err != nil {
		t.Fatalf("Package() error = %v", err)
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "setup.intunewin" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("output directory holds %v, want only setup.intunewin", names)
	}
}

func TestWriteEncryptedPayloadRemovesScratchOnFailure(t *testing.T) {
	outputDir := t.TempDir()
	encKey, macKey, iv, err := GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}

	// A source that cannot be walked fails partway through, after the scratch
	// file has been created.
	if _, err := writeEncryptedPayload(outputDir, filepath.Join(outputDir, "missing"), 1, encKey, macKey, iv, nil); err == nil {
		t.Fatal("expected an error for a nonexistent source")
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("scratch file left behind: %v", entries)
	}
}
