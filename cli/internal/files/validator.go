package files

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// FileInfo holds information about a file to be sent
type FileInfo struct {
	// Path is the absolute path to the file
	Path string

	// Name is the filename (without directory)
	Name string

	// Size is the file size in bytes
	Size int64

	// Type is the MIME type of the file (e.g., "application/pdf", "text/plain")
	Type string

	// IsReadable indicates if the file can be read
	IsReadable bool
}

// ValidateFiles checks if all files exist and are readable
// Returns a list of FileInfo for valid files and an error if any file is invalid
func ValidateFiles(filePaths []string) ([]FileInfo, error) {
	if len(filePaths) == 0 {
		return nil, fmt.Errorf("no files specified")
	}

	var fileInfos []FileInfo
	var errors []string

	for _, path := range filePaths {
		fileInfo, err := validateSingleFile(path)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		fileInfos = append(fileInfos, fileInfo)
	}

	// If any file validation failed, return all errors
	if len(errors) > 0 {
		return nil, fmt.Errorf("file validation failed:\n  - %s", joinErrors(errors))
	}

	return fileInfos, nil
}

// validateSingleFile checks a single file and returns its info
func validateSingleFile(path string) (FileInfo, error) {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return FileInfo{}, fmt.Errorf("%s: failed to get absolute path: %w", path, err)
	}

	// Check if file exists
	stat, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return FileInfo{}, fmt.Errorf("%s: file does not exist", path)
		}
		return FileInfo{}, fmt.Errorf("%s: failed to stat file: %w", path, err)
	}

	// Check if it's a directory
	if stat.IsDir() {
		return FileInfo{}, fmt.Errorf("%s: is a directory (directories not yet supported)", path)
	}

	// Check if file is empty
	if stat.Size() == 0 {
		return FileInfo{}, fmt.Errorf("%s: file is empty", path)
	}

	// Check if file is readable
	file, err := os.Open(absPath)
	if err != nil {
		return FileInfo{}, fmt.Errorf("%s: cannot open file (check permissions): %w", path, err)
	}
	file.Close()

	// Get just the filename (without directory)
	name := filepath.Base(absPath)

	// Detect MIME type from file extension
	mimeType := mime.TypeByExtension(filepath.Ext(absPath))
	if mimeType == "" {
		// Default to binary if unknown
		mimeType = "application/octet-stream"
	}

	return FileInfo{
		Path:       absPath,
		Name:       name,
		Size:       stat.Size(),
		Type:       mimeType,
		IsReadable: true,
	}, nil
}

// joinErrors joins multiple error messages with newlines
func joinErrors(errors []string) string {
	var result strings.Builder
	for i, err := range errors {
		if i > 0 {
			result.WriteString("\n  - ")
		}
		result.WriteString(err)
	}
	return result.String()
}

// GetTotalSize returns the total size of all files
func GetTotalSize(fileInfos []FileInfo) int64 {
	var total int64
	for _, file := range fileInfos {
		total += file.Size
	}
	return total
}
