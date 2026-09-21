package packager

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
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

	sourceSize, err := GetFolderSize(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get source folder size: %w", err)
	}

	fileCount, err := CountFiles(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to count files: %w", err)
	}
	if fileCount == 0 {
		return nil, fmt.Errorf("no files found in source directory")
	}

	// Step 2: Extract MSI info if applicable (10%)
	report("Checking for MSI metadata", 0.10)

	var msiInfo *MsiInfo
	setupFilePath := filepath.Join(sourcePath, setupFile)
	if IsMsiFile(setupFile) {
		msiInfo, err = ExtractMsiInfo(setupFilePath)
		if err != nil {
			// MSI metadata is optional, so a failure here is reported rather
			// than fatal. It goes through the progress callback instead of
			// stdout: the TUI owns the terminal, and printing straight to it
			// corrupts the alternate screen.
			report(fmt.Sprintf("Warning: could not extract MSI metadata: %v", err), 0.10)
		}
	}

	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Step 3: Compress and encrypt in one pass (15-70%)
	//
	// The payload is compressed straight into the encryptor and out to a
	// scratch file, so the source never has to be held in memory. The ZIP is
	// therefore never materialised: its SHA256 and length are taken as the
	// bytes go past.
	report("Compressing and encrypting", 0.15)

	encKey, macKey, iv, err := GenerateKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	payload, err := writeEncryptedPayload(outputPath, sourcePath, fileCount, encKey, macKey, iv,
		func(file string, pct float64) {
			report(fmt.Sprintf("Compressing: %s", file), 0.15+pct*0.55)
		})
	if err != nil {
		return nil, err
	}
	defer os.Remove(payload.path)

	report("Encryption complete", 0.70)

	// Step 4: Generate metadata XML (70-80%)
	report("Generating metadata", 0.75)

	appName := GetApplicationName(setupFile)
	detectionXML, err := GenerateDetectionXML(&MetadataParams{
		Name:                   appName,
		SetupFile:              setupFile,
		UnencryptedContentSize: payload.plainSize,
		EncryptionInfo: &EncryptionInfo{
			EncryptionKey:        encKey,
			MacKey:               macKey,
			InitializationVector: iv,
			Mac:                  payload.mac,
			FileDigest:           payload.fileDigest,
		},
		MsiInfo: msiInfo,
	})
	if err != nil {
		return nil, fmt.Errorf("metadata generation failed: %w", err)
	}

	// Step 5: Write the package (80-100%)
	report("Creating package", 0.85)

	outputFilePath := filepath.Join(outputPath, fmt.Sprintf("%s.intunewin", appName))
	finalSize, err := buildPackageFile(outputFilePath, payload.path, detectionXML)
	if err != nil {
		return nil, err
	}

	report("Complete", 1.0)

	return &PackageResult{
		OutputPath:    outputFilePath,
		SourceSize:    sourceSize,
		ZipSize:       payload.plainSize,
		EncryptedSize: payload.encryptedSize,
		FinalSize:     finalSize,
		FileCount:     fileCount,
	}, nil
}

// encryptedPayload describes the scratch file holding the encrypted content,
// laid out as [HMAC][IV][ciphertext] exactly as it appears inside the package.
type encryptedPayload struct {
	path          string
	plainSize     int64 // length of the ZIP before encryption
	encryptedSize int64
	fileDigest    []byte // SHA256 of the ZIP before encryption
	mac           []byte
}

// writeEncryptedPayload compresses sourcePath and encrypts it in a single pass
// into a scratch file beside the output.
//
// The MAC leads the blob but is only known once the last byte has been
// encrypted, so room is reserved for it and it is filled in at the end.
func writeEncryptedPayload(outputPath, sourcePath string, fileCount int, encKey, macKey, iv []byte,
	progress func(file string, pct float64)) (*encryptedPayload, error) {

	tmp, err := os.CreateTemp(outputPath, ".intunewin-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create scratch file: %w", err)
	}
	defer tmp.Close()

	abandon := func(err error) (*encryptedPayload, error) {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}

	if _, err := tmp.Write(make([]byte, sha256.Size)); err != nil {
		return abandon(fmt.Errorf("failed to reserve MAC space: %w", err))
	}

	enc, err := newContentEncryptor(tmp, encKey, macKey, iv)
	if err != nil {
		return abandon(fmt.Errorf("encryption failed: %w", err))
	}

	// The digest and length describe the ZIP, so both are taken on the way in.
	digest := sha256.New()
	counter := &countingWriter{}

	zw := zip.NewWriter(io.MultiWriter(enc, digest, counter))
	if err := writeZipTree(zw, sourcePath, fileCount, progress); err != nil {
		return abandon(fmt.Errorf("compression failed: %w", err))
	}
	if err := zw.Close(); err != nil {
		return abandon(fmt.Errorf("failed to close ZIP writer: %w", err))
	}

	mac, err := enc.Close()
	if err != nil {
		return abandon(fmt.Errorf("encryption failed: %w", err))
	}
	if _, err := tmp.WriteAt(mac, 0); err != nil {
		return abandon(fmt.Errorf("failed to write MAC: %w", err))
	}

	info, err := tmp.Stat()
	if err != nil {
		return abandon(fmt.Errorf("failed to stat scratch file: %w", err))
	}

	return &encryptedPayload{
		path:          tmp.Name(),
		plainSize:     counter.n,
		encryptedSize: info.Size(),
		fileDigest:    digest.Sum(nil),
		mac:           mac,
	}, nil
}

// buildPackageFile writes the outer archive to outputFilePath, copying the
// encrypted payload in from the scratch file, and reports the finished size.
func buildPackageFile(outputFilePath, payloadPath string, detectionXML []byte) (int64, error) {
	payload, err := os.Open(payloadPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open scratch file: %w", err)
	}
	defer payload.Close()

	out, err := os.Create(outputFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create output file: %w", err)
	}

	if err := writeIntunewinPackage(out, payload, detectionXML); err != nil {
		out.Close()
		os.Remove(outputFilePath)
		return 0, fmt.Errorf("package creation failed: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(outputFilePath)
		return 0, fmt.Errorf("failed to write output file: %w", err)
	}

	info, err := os.Stat(outputFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat output file: %w", err)
	}
	return info.Size(), nil
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
