package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
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

// GetSHA256ForVersion fetches the expected SHA256 hash for a Python version
// from the python-build-standalone GitHub releases
func (v *Verifier) GetSHA256ForVersion(version string, platform Platform, buildDate string) (string, error) {
	slog.Info("fetching SHA256 hash from GitHub releases",
		"version", version,
		"platform", platform.ToPythonBuildStandalone(),
		"buildDate", buildDate)

	// Build filename to search for
	triple := platform.ToPythonBuildStandalone()
	filename := fmt.Sprintf("cpython-%s+%s-%s-install_only.tar.gz",
		version, buildDate, triple)

	// Fetch release data from GitHub API
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use astral-sh repo (was indygreg/python-build-standalone)
	releaseURL := fmt.Sprintf("https://api.github.com/repos/astral-sh/python-build-standalone/releases/tags/%s", buildDate)

	req, err := http.NewRequestWithContext(ctx, "GET", releaseURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent (GitHub API requires it)
	req.Header.Set("User-Agent", "ophid/0.1.0")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch release data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d for release %s", resp.StatusCode, buildDate)
	}

	// Parse release response
	var release struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release data: %w", err)
	}

	// Extract SHA256 hash from release body
	// Format in release notes: "sha256:ca8f0ba14dbcf474fe3b9cd5d8839a48eb08b00f6e90244546e761a2ba956ee0"
	hash, err := v.extractSHA256FromReleaseNotes(release.Body, filename)
	if err != nil {
		return "", fmt.Errorf("failed to extract SHA256 for %s: %w", filename, err)
	}

	slog.Info("successfully fetched SHA256 hash",
		"filename", filename,
		"hash", hash[:16]+"...")

	return hash, nil
}

// extractSHA256FromReleaseNotes parses the release body to find the SHA256 hash
func (v *Verifier) extractSHA256FromReleaseNotes(body, filename string) (string, error) {
	// Look for the filename in the release notes
	lines := strings.Split(body, "\n")

	// Find the line with the filename
	filenameIdx := -1
	for i, line := range lines {
		if strings.Contains(line, filename) {
			filenameIdx = i
			break
		}
	}

	if filenameIdx == -1 {
		return "", fmt.Errorf("filename %s not found in release notes", filename)
	}

	// Look for sha256 hash in nearby lines (typically 1-3 lines after filename)
	// Pattern: "sha256:HASH" or "SHA256: HASH"
	hashPattern := regexp.MustCompile(`sha256:\s*([a-fA-F0-9]{64})`)

	// Check the next 5 lines for the hash
	for i := filenameIdx; i < len(lines) && i < filenameIdx+5; i++ {
		matches := hashPattern.FindStringSubmatch(lines[i])
		if len(matches) == 2 {
			return strings.ToLower(matches[1]), nil
		}
	}

	return "", fmt.Errorf("SHA256 hash not found near filename in release notes")
}
