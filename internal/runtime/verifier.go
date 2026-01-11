package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Verifier handles checksum verification of downloaded files
type Verifier struct{}

// NewVerifier creates a new verifier
func NewVerifier() *Verifier {
	return &Verifier{}
}

// VerifySHA256 verifies the SHA256 checksum of a file
func (v *Verifier) VerifySHA256(filePath, expectedHash string) error {
	// Calculate hash
	actualHash, err := v.calculateSHA256(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}

	// Compare hashes
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch:\n  expected: %s\n  got:      %s",
			expectedHash, actualHash)
	}

	return nil
}

// calculateSHA256 calculates the SHA256 hash of a file
func (v *Verifier) calculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// VerifyFileExists checks if a file exists and is readable
func (v *Verifier) VerifyFileExists(filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", filePath)
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", filePath)
	}

	return nil
}

// GetSHA256ForVersion returns the expected SHA256 hash for a Python version
// TODO: This should fetch from a trusted source or embedded data
// For now, we'll skip verification and add it later
func (v *Verifier) GetSHA256ForVersion(version string, platform Platform) (string, error) {
	// NOTE: In production, this should:
	// 1. Fetch SHA256SUMS from python-build-standalone releases
	// 2. Or embed known hashes in the binary
	// 3. Or fetch from a trusted OPHID registry

	// For now, return empty to skip verification
	// TODO: Implement proper hash verification
	return "", fmt.Errorf("SHA256 verification not yet implemented")
}
