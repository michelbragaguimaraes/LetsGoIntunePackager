package packager

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ZipFolder compresses a folder into an in-memory ZIP archive
// Returns the ZIP data as bytes
func ZipFolder(sourcePath string) ([]byte, error) {
	// Ensure source path exists and is a directory
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("source path error: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path is not a directory: %s", sourcePath)
	}

	// Create buffer for ZIP
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Get absolute path for reliable relative path calculation
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Walk the directory tree
	err = filepath.Walk(absSource, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == absSource {
			return nil
		}

		// Calculate relative path for ZIP entry
		relPath, err := filepath.Rel(absSource, path)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path: %w", err)
		}

		// Use forward slashes for ZIP paths (cross-platform compatibility)
		zipPath := strings.ReplaceAll(relPath, string(os.PathSeparator), "/")

		if info.IsDir() {
			// Add directory entry (must end with /)
			_, err = zipWriter.Create(zipPath + "/")
			return err
		}

		// Create file entry with proper header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("failed to create file header: %w", err)
		}
		header.Name = zipPath
		header.Method = zip.Deflate // Use compression

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create ZIP entry: %w", err)
		}

		// Open and copy file content
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		if err != nil {
			return fmt.Errorf("failed to write file to ZIP: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Close ZIP writer to finalize
	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close ZIP writer: %w", err)
	}

	return buf.Bytes(), nil
}

// ZipFolderWithProgress compresses a folder with progress callback
// callback receives current file path and progress percentage (0.0 to 1.0)
func ZipFolderWithProgress(sourcePath string, callback func(file string, progress float64)) ([]byte, error) {
	// First pass: count total files for progress calculation
	var totalFiles int
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	err = filepath.Walk(absSource, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalFiles++
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to count files: %w", err)
	}

	if totalFiles == 0 {
		return nil, fmt.Errorf("no files found in source directory")
	}

	// Create buffer for ZIP
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	var processedFiles int

	// Walk and compress
	err = filepath.Walk(absSource, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == absSource {
			return nil
		}

		relPath, err := filepath.Rel(absSource, path)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path: %w", err)
		}

		zipPath := strings.ReplaceAll(relPath, string(os.PathSeparator), "/")

		if info.IsDir() {
			_, err = zipWriter.Create(zipPath + "/")
			return err
		}

		// Report progress
		if callback != nil {
			progress := float64(processedFiles) / float64(totalFiles)
			callback(relPath, progress)
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("failed to create file header: %w", err)
		}
		header.Name = zipPath
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create ZIP entry: %w", err)
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		if err != nil {
			return fmt.Errorf("failed to write file to ZIP: %w", err)
		}

		processedFiles++
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Final progress callback
	if callback != nil {
		callback("complete", 1.0)
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close ZIP writer: %w", err)
	}

	return buf.Bytes(), nil
}

// CreateIntunewinPackage creates the final .intunewin package structure
// Structure: outer.zip/IntuneWinPackage/Contents/IntunePackage.intunewin + Metadata/Detection.xml
// IMPORTANT: The outer ZIP must use Store method (no compression) to match Microsoft's official format
func CreateIntunewinPackage(encryptedContent, detectionXML []byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	now := time.Now()

	// Create directory structure (using IntuneWinPackage to match official Microsoft format)
	// IntuneWinPackage/Contents/IntunePackage.intunewin
	// Must use Store method (no compression) - this is critical for Intune acceptance
	contentHeader := &zip.FileHeader{
		Name:   "IntuneWinPackage/Contents/IntunePackage.intunewin",
		Method: zip.Store, // No compression - required by Microsoft Intune
	}
	contentHeader.Modified = now
	contentWriter, err := zipWriter.CreateHeader(contentHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to create encrypted content entry: %w", err)
	}
	if _, err := contentWriter.Write(encryptedContent); err != nil {
		return nil, fmt.Errorf("failed to write encrypted content: %w", err)
	}

	// IntuneWinPackage/Metadata/Detection.xml
	// Must use Store method (no compression) - this is critical for Intune acceptance
	metadataHeader := &zip.FileHeader{
		Name:   "IntuneWinPackage/Metadata/Detection.xml",
		Method: zip.Store, // No compression - required by Microsoft Intune
	}
	metadataHeader.Modified = now
	metadataWriter, err := zipWriter.CreateHeader(metadataHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata entry: %w", err)
	}
	if _, err := metadataWriter.Write(detectionXML); err != nil {
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close package: %w", err)
	}

	return buf.Bytes(), nil
}

// GetFolderSize calculates the total size of all files in a folder
func GetFolderSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// CountFiles returns the number of files in a directory (recursive)
func CountFiles(path string) (int, error) {
	var count int
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}
