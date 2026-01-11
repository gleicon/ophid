package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gleicon/ophid/internal/security"
)

// GitInstaller handles installation from Git repositories
type GitInstaller struct {
	homeDir     string
	cacheDir    string
	scanner     *security.Scanner
}

// NewGitInstaller creates a new Git installer
func NewGitInstaller(homeDir string, scanner *security.Scanner) *GitInstaller {
	cacheDir := filepath.Join(homeDir, "cache", "git")
	os.MkdirAll(cacheDir, 0755)

	return &GitInstaller{
		homeDir:  homeDir,
		cacheDir: cacheDir,
		scanner:  scanner,
	}
}

// CloneRepository clones a Git repository
func (gi *GitInstaller) CloneRepository(ctx context.Context, source InstallSource) (string, error) {
	// Generate a unique directory name for the clone
	repoName := gi.extractRepoName(source.URL)
	clonePath := filepath.Join(gi.cacheDir, repoName)

	// Remove existing clone if present
	if _, err := os.Stat(clonePath); err == nil {
		fmt.Printf("Removing existing clone at %s\n", clonePath)
		if err := os.RemoveAll(clonePath); err != nil {
			return "", fmt.Errorf("failed to remove existing clone: %w", err)
		}
	}

	// Build git clone command
	args := []string{"clone"}

	// Add depth for faster cloning (shallow clone)
	args = append(args, "--depth", "1")

	// Add branch/tag if specified
	if source.Branch != "" {
		args = append(args, "--branch", source.Branch)
	} else if source.Tag != "" {
		args = append(args, "--branch", source.Tag)
	}

	args = append(args, source.URL, clonePath)

	// Execute git clone
	fmt.Printf("Cloning %s...\n", source.URL)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	// Checkout specific commit if specified
	if source.Commit != "" {
		fmt.Printf("Checking out commit %s...\n", source.Commit)
		checkoutCmd := exec.CommandContext(ctx, "git", "-C", clonePath, "checkout", source.Commit)
		checkoutCmd.Stdout = os.Stdout
		checkoutCmd.Stderr = os.Stderr

		if err := checkoutCmd.Run(); err != nil {
			return "", fmt.Errorf("git checkout failed: %w", err)
		}
	}

	// Return subdirectory path if specified
	installPath := clonePath
	if source.Subdirectory != "" {
		installPath = filepath.Join(clonePath, source.Subdirectory)
		if _, err := os.Stat(installPath); err != nil {
			return "", fmt.Errorf("subdirectory %s not found in repository", source.Subdirectory)
		}
	}

	return installPath, nil
}

// ScanRepository scans a cloned repository for security issues
func (gi *GitInstaller) ScanRepository(ctx context.Context, repoPath string) (*SecurityInfo, error) {
	secInfo := &SecurityInfo{
		LicenseCompliant: true,
	}

	// SECRET SCANNING
	fmt.Println("\n🔐 Scanning for secrets...")
	secretsReport, err := gi.scanner.ScanSecrets(ctx, repoPath)
	if err != nil {
		fmt.Printf("⚠ Warning: secret scan failed: %v\n", err)
	} else {
		secInfo.SecretsReport = secretsReport
		secInfo.SecretsScanDate = time.Now()

		if secretsReport.HasSecrets() {
			fmt.Printf("⚠ ALERT: Found %d secret(s)", secretsReport.TotalSecrets)
			if secretsReport.CriticalSecrets > 0 {
				fmt.Printf(" (%d critical)", secretsReport.CriticalSecrets)
			}
			fmt.Println()

			// Display first few findings
			for i, finding := range secretsReport.Findings {
				if i >= 3 {
					fmt.Printf("  ... and %d more\n", len(secretsReport.Findings)-3)
					break
				}
				fmt.Printf("  - %s in %s:%d\n", finding.Type,
					filepath.Base(finding.File), finding.Line)
			}
			fmt.Println()
		} else {
			fmt.Println("✓ No secrets found")
		}
	}

	// DEPENDENCY VULNERABILITY SCANNING
	// Look for dependency files
	depFiles := []string{
		filepath.Join(repoPath, "requirements.txt"),
		filepath.Join(repoPath, "setup.py"),
		filepath.Join(repoPath, "pyproject.toml"),
		filepath.Join(repoPath, "go.mod"),
		filepath.Join(repoPath, "package.json"),
	}

	var packages []security.Package
	var foundFile string

	for _, depFile := range depFiles {
		if _, err := os.Stat(depFile); err == nil {
			foundFile = depFile
			parsedPackages, err := gi.parseDependencyFile(depFile)
			if err != nil {
				fmt.Printf("Warning: failed to parse %s: %v\n", depFile, err)
				continue
			}
			packages = append(packages, parsedPackages...)
			break
		}
	}

	if len(packages) == 0 {
		fmt.Println("No dependency files found - skipping vulnerability scan")
		return secInfo, nil
	}

	fmt.Printf("Scanning %d dependencies from %s...\n", len(packages), filepath.Base(foundFile))

	// Scan for vulnerabilities
	results, err := gi.scanner.ScanPackages(ctx, packages)
	if err != nil {
		return secInfo, fmt.Errorf("vulnerability scan failed: %w", err)
	}

	// Count vulnerabilities
	for _, result := range results {
		secInfo.VulnCount += len(result.Vulnerabilities)
		secInfo.CriticalVulnCount += result.CriticalCount()
	}

	// Generate SBOM
	sbom, err := security.GenerateSBOM(packages, "ophid-git")
	if err != nil {
		fmt.Printf("Warning: SBOM generation failed: %v\n", err)
	} else {
		sbomPath := filepath.Join(repoPath, "ophid-sbom.json")
		if err := security.WriteSBOM(sbom, sbomPath); err != nil {
			fmt.Printf("Warning: failed to write SBOM: %v\n", err)
		} else {
			secInfo.SBOMPath = sbomPath
		}
	}

	return secInfo, nil
}

// DetectEcosystem detects the ecosystem of a cloned repository
func (gi *GitInstaller) DetectEcosystem(repoPath string) string {
	// Check for Python
	if gi.fileExists(repoPath, "setup.py") ||
		gi.fileExists(repoPath, "pyproject.toml") ||
		gi.fileExists(repoPath, "requirements.txt") {
		return "python"
	}

	// Check for Go
	if gi.fileExists(repoPath, "go.mod") {
		return "go"
	}

	// Check for Node.js
	if gi.fileExists(repoPath, "package.json") {
		return "node"
	}

	// Check for Ruby
	if gi.fileExists(repoPath, "Gemfile") {
		return "ruby"
	}

	return "unknown"
}

// GetVersion attempts to extract version from the repository
func (gi *GitInstaller) GetVersion(ctx context.Context, repoPath string) (string, error) {
	// Try to get the latest git tag
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "describe", "--tags", "--abbrev=0")
	output, err := cmd.Output()
	if err == nil {
		version := strings.TrimSpace(string(output))
		return strings.TrimPrefix(version, "v"), nil
	}

	// Fall back to commit SHA
	cmd = exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--short", "HEAD")
	output, err = cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}

	return "dev", nil
}

// extractRepoName extracts repository name from URL
func (gi *GitInstaller) extractRepoName(url string) string {
	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Get last path component
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return "repo"
}

// fileExists checks if a file exists in a directory
func (gi *GitInstaller) fileExists(dir, filename string) bool {
	_, err := os.Stat(filepath.Join(dir, filename))
	return err == nil
}

// parseDependencyFile parses a dependency file
func (gi *GitInstaller) parseDependencyFile(filePath string) ([]security.Package, error) {
	if strings.HasSuffix(filePath, "requirements.txt") {
		return security.ParseRequirementsTxt(filePath)
	} else if strings.HasSuffix(filePath, "go.mod") {
		return security.ParseGoMod(filePath)
	} else if strings.HasSuffix(filePath, "package.json") {
		return security.ParsePackageJSON(filePath)
	}
	return nil, fmt.Errorf("unsupported dependency file: %s", filePath)
}
