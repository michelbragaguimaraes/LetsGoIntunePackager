package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PackageResult contains the results of a successful packaging operation
type PackageResult struct {
	// OutputPath is the full path to the generated .intunewin file
	OutputPath string
	// SourceSize is the size of the original source folder in bytes
	SourceSize int64
	// ZipSize is the size of the compressed ZIP in bytes
	ZipSize int64
	// EncryptedSize is the size of the encrypted blob in bytes
	EncryptedSize int64
	// FinalSize is the size of the final .intunewin file in bytes
	FinalSize int64
	// FileCount is the number of files in the source folder
	FileCount int
}

// ProgressCallback is called during packaging to report progress
// step: current step name (e.g., "Compressing files", "Encrypting")
// percent: progress percentage (0.0 to 1.0)
type ProgressCallback func(step string, percent float64)

// Package creates an .intunewin package from the source folder
// sourcePath: folder containing the setup file and related files
// setupFile: name of the setup file (e.g., "setup.msi", "install.exe")
// outputPath: folder where the .intunewin file will be created
// progress: optional callback for progress updates (can be nil)
func Package(sourcePath, setupFile, outputPath string, progress ProgressCallback) (*PackageResult, error) {
	// Helper to report progress
	report := func(step string, pct float64) {
		if progress != nil {
			progress(step, pct)
		}
	}

	// Step 1: Validate inputs (5%)
	report("Validating inputs", 0.05)

	if err := validateInputs(sourcePath, setupFile, outputPath); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get source folder stats
	sourceSize, err := GetFolderSize(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get source folder size: %w", err)
	}

	fileCount, err := CountFiles(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to count files: %w", err)
	}

	// Step 2: Extract MSI info if applicable (10%)
	report("Checking for MSI metadata", 0.10)

	var msiInfo *MsiInfo
	setupFilePath := filepath.Join(sourcePath, setupFile)
	if IsMsiFile(setupFile) {
		msiInfo, err = ExtractMsiInfo(setupFilePath)
		if err != nil {
			// Log warning but continue - MSI info is optional
			fmt.Printf("Warning: Could not extract MSI metadata: %v\n", err)
		}
	}

	// Step 3: Compress source folder (10-40%)
	report("Compressing files", 0.15)

	zipData, err := ZipFolderWithProgress(sourcePath, func(file string, pct float64) {
		// Scale ZIP progress from 15% to 40%
		scaledPct := 0.15 + (pct * 0.25)
		report(fmt.Sprintf("Compressing: %s", file), scaledPct)
	})
	if err != nil {
		return nil, fmt.Errorf("compression failed: %w", err)
	}
	zipSize := int64(len(zipData))

	// Step 4: Encrypt content (40-70%)
	report("Encrypting content", 0.45)

	encInfo, encryptedData, err := CreateEncryptionInfo(zipData)
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}
	encryptedSize := int64(len(encryptedData))

	report("Encryption complete", 0.70)

	// Step 5: Generate metadata XML (70-80%)
	report("Generating metadata", 0.75)

	appName := GetApplicationName(setupFile)
	metadataParams := &MetadataParams{
		Name:                   appName,
		SetupFile:              setupFile,
		UnencryptedContentSize: zipSize,
		EncryptionInfo:         encInfo,
		MsiInfo:                msiInfo,
	}

	detectionXML, err := GenerateDetectionXML(metadataParams)
	if err != nil {
		return nil, fmt.Errorf("metadata generation failed: %w", err)
	}

	// Step 6: Create final package (80-95%)
	report("Creating package", 0.85)

	packageData, err := CreateIntunewinPackage(encryptedData, detectionXML)
	if err != nil {
		return nil, fmt.Errorf("package creation failed: %w", err)
	}
	finalSize := int64(len(packageData))

	// Step 7: Write output file (95-100%)
	report("Writing output file", 0.95)

	// Ensure output directory exists
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate output filename
	outputFileName := fmt.Sprintf("%s.intunewin", appName)
	outputFilePath := filepath.Join(outputPath, outputFileName)

	// Write the package
	if err := os.WriteFile(outputFilePath, packageData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write output file: %w", err)
	}

	report("Complete", 1.0)

	return &PackageResult{
		OutputPath:    outputFilePath,
		SourceSize:    sourceSize,
		ZipSize:       zipSize,
		EncryptedSize: encryptedSize,
		FinalSize:     finalSize,
		FileCount:     fileCount,
	}, nil
}

// validateInputs validates the input parameters
func validateInputs(sourcePath, setupFile, outputPath string) error {
	// Check source path exists and is a directory
	sourceInfo, err := os.Stat(sourcePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("source folder does not exist: %s", sourcePath)
	}
	if err != nil {
		return fmt.Errorf("cannot access source folder: %w", err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source path is not a directory: %s", sourcePath)
	}

	// Check setup file exists in source folder
	setupFilePath := filepath.Join(sourcePath, setupFile)
	setupInfo, err := os.Stat(setupFilePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("setup file not found: %s", setupFilePath)
	}
	if err != nil {
		return fmt.Errorf("cannot access setup file: %w", err)
	}
	if setupInfo.IsDir() {
		return fmt.Errorf("setup file is a directory: %s", setupFilePath)
	}

	// Validate setup file extension
	ext := strings.ToLower(filepath.Ext(setupFile))
	validExtensions := map[string]bool{
		".msi": true,
		".exe": true,
		".ps1": true,
		".cmd": true,
		".bat": true,
	}
	if !validExtensions[ext] {
		return fmt.Errorf("unsupported setup file type: %s (supported: .msi, .exe, .ps1, .cmd, .bat)", ext)
	}

	// Validate output path is not empty
	if outputPath == "" {
		return fmt.Errorf("output path cannot be empty")
	}

	return nil
}

// FormatSize formats bytes into human-readable string
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
