package runtime

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Extractor handles extracting tar.gz archives
type Extractor struct{}

// NewExtractor creates a new extractor
func NewExtractor() *Extractor {
	return &Extractor{}
}

// Extract extracts a tar.gz file to the destination directory
func (e *Extractor) Extract(tarballPath, destDir string) error {
	// Open the tarball
	file, err := os.Open(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to open tarball: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	// Create tar reader
	tr := tar.NewReader(gzr)

	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination dir: %w", err)
	}

	// Extract files
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Construct target path
		target := filepath.Join(destDir, header.Name)

		// Security: Prevent path traversal
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			// Create file
			if err := e.extractFile(tr, target, header); err != nil {
				return fmt.Errorf("failed to extract file %s: %w", header.Name, err)
			}

		case tar.TypeSymlink:
			// Create symlink
			if err := e.extractSymlink(target, header); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", header.Name, err)
			}

		default:
			// Skip other types (char devices, block devices, etc.)
			fmt.Printf("  skipping: %s (type %c)\n", header.Name, header.Typeflag)
		}
	}

	return nil
}

// extractFile extracts a single file from the tar archive
func (e *Extractor) extractFile(tr *tar.Reader, target string, header *tar.Header) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}

	// Create the file
	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
	if err != nil {
		return err
	}
	defer file.Close()

	// Copy contents
	if _, err := io.Copy(file, tr); err != nil {
		return err
	}

	return nil
}

// extractSymlink creates a symbolic link
func (e *Extractor) extractSymlink(target string, header *tar.Header) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}

	// Remove existing file/symlink if it exists
	os.Remove(target)

	// Create symlink
	if err := os.Symlink(header.Linkname, target); err != nil {
		return err
	}

	return nil
}
