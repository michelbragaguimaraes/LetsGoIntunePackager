package packager

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// writeZipTree adds every entry under sourcePath to zw, using forward slashes
// so the archive reads the same on any platform.
//
// totalFiles, when positive, is the denominator for the progress fraction
// handed to callback; pass 0 to report no progress.
func writeZipTree(zw *zip.Writer, sourcePath string, totalFiles int, callback func(file string, progress float64)) error {
	// An absolute base keeps the relative paths reliable regardless of how the
	// caller spelled sourcePath.
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	var processed int
	return filepath.Walk(absSource, func(path string, info os.FileInfo, err error) error {
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
			_, err = zw.Create(zipPath + "/")
			return err
		}

		if callback != nil && totalFiles > 0 {
			callback(relPath, float64(processed)/float64(totalFiles))
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("failed to create file header: %w", err)
		}
		header.Name = zipPath
		header.Method = zip.Deflate

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create ZIP entry: %w", err)
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		if _, err := io.Copy(writer, file); err != nil {
			return fmt.Errorf("failed to write file to ZIP: %w", err)
		}

		processed++
		return nil
	})
}

// ZipFolder compresses a folder into an in-memory ZIP archive
// Returns the ZIP data as bytes
func ZipFolder(sourcePath string) ([]byte, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("source path error: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path is not a directory: %s", sourcePath)
	}

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	if err := writeZipTree(zipWriter, sourcePath, 0, nil); err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}
	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close ZIP writer: %w", err)
	}

	return buf.Bytes(), nil
}

// ZipFolderWithProgress compresses a folder with progress callback
// callback receives current file path and progress percentage (0.0 to 1.0)
func ZipFolderWithProgress(sourcePath string, callback func(file string, progress float64)) ([]byte, error) {
	totalFiles, err := CountFiles(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to count files: %w", err)
	}
	if totalFiles == 0 {
		return nil, fmt.Errorf("no files found in source directory")
	}

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	if err := writeZipTree(zipWriter, sourcePath, totalFiles, callback); err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}
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
//
// It buffers the whole package; Package writes the same bytes straight to disk
// through writeIntunewinPackage.
func CreateIntunewinPackage(encryptedContent, detectionXML []byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := writeIntunewinPackage(buf, bytes.NewReader(encryptedContent), detectionXML); err != nil {
		return nil, err
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
