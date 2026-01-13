package tool

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gleicon/ophid/internal/security"
)

// LocalInstaller handles installation from local directories
type LocalInstaller struct {
	homeDir string
	scanner *security.Scanner
}

// NewLocalInstaller creates a new local installer
func NewLocalInstaller(homeDir string, scanner *security.Scanner) *LocalInstaller {
	return &LocalInstaller{
		homeDir: homeDir,
		scanner: scanner,
	}
}

// ValidateLocalPath validates that a local path exists and is installable
func (li *LocalInstaller) ValidateLocalPath(path string) error {
	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	// Must be a directory
	if !info.IsDir() {
		return fmt.Errorf("path must be a directory: %s", path)
	}

	// Check for common project files
	hasProjectFile := false
	projectFiles := []string{
		"setup.py",
		"pyproject.toml",
		"package.json",
		"go.mod",
		"Cargo.toml",
		"Gemfile",
	}

	for _, pf := range projectFiles {
		if li.fileExists(path, pf) {
			hasProjectFile = true
			break
		}
	}

	if !hasProjectFile {
		return fmt.Errorf("directory does not appear to be a valid project (missing setup.py, pyproject.toml, package.json, etc.)")
	}

	return nil
}

// ScanLocalPath scans a local directory for security issues
func (li *LocalInstaller) ScanLocalPath(ctx context.Context, path string) (*SecurityInfo, error) {
	secInfo := &SecurityInfo{
		LicenseCompliant: true,
	}

	// SECRET SCANNING
	slog.Info("scanning local directory for secrets", "path", path)
	secretsReport, err := li.scanner.ScanSecrets(ctx, path)
	if err != nil {
		slog.Warn("secret scan failed", "path", path, "error", err)
	} else {
		secInfo.SecretsReport = secretsReport
		secInfo.SecretsScanDate = time.Now()

		if secretsReport.HasSecrets() {
			fmt.Printf("[WARN] ALERT: Found %d secret(s)", secretsReport.TotalSecrets)
			if secretsReport.CriticalSecrets > 0 {
				fmt.Printf(" (%d critical)", secretsReport.CriticalSecrets)
			}
			fmt.Println()

			for i, finding := range secretsReport.Findings {
				if i >= 5 {
					fmt.Printf("  ... and %d more\n", len(secretsReport.Findings)-5)
					break
				}
				fmt.Printf("  - [%s] %s\n", finding.Severity, finding.Type)
				fmt.Printf("    File: %s:%d\n", finding.File, finding.Line)
				fmt.Printf("    Secret: %s\n", security.RedactSecret(finding.Secret))
			}
			fmt.Println("\n[WARN] CRITICAL: Review and rotate any exposed secrets immediately")
		} else {
			fmt.Println("[OK] No secrets found")
		}
	}

	// DEPENDENCY VULNERABILITY SCANNING
	// Look for dependency files
	depFiles := []string{
		filepath.Join(path, "requirements.txt"),
		filepath.Join(path, "setup.py"),
		filepath.Join(path, "pyproject.toml"),
		filepath.Join(path, "go.mod"),
		filepath.Join(path, "package.json"),
	}

	var packages []security.Package
	var foundFile string

	for _, depFile := range depFiles {
		if _, err := os.Stat(depFile); err == nil {
			foundFile = depFile
			parsedPackages, err := li.parseDependencyFile(depFile)
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
	results, err := li.scanner.ScanPackages(ctx, packages)
	if err != nil {
		return secInfo, fmt.Errorf("vulnerability scan failed: %w", err)
	}

	// Count vulnerabilities
	for _, result := range results {
		secInfo.VulnCount += len(result.Vulnerabilities)
		secInfo.CriticalVulnCount += result.CriticalCount()
	}

	// Display scan results
	if secInfo.VulnCount > 0 {
		fmt.Printf("\n[WARN] Found %d vulnerabilities", secInfo.VulnCount)
		if secInfo.CriticalVulnCount > 0 {
			fmt.Printf(" (%d critical)", secInfo.CriticalVulnCount)
		}
		fmt.Println()

		for _, result := range results {
			if len(result.Vulnerabilities) > 0 {
				for _, vuln := range result.Vulnerabilities {
					fmt.Printf("  - %s in %s@%s: %s\n",
						vuln.ID,
						result.Package.Name,
						result.Package.Version,
						vuln.Summary)
				}
			}
		}
		fmt.Println()
	} else {
		fmt.Println("[OK] No vulnerabilities found")
	}

	// Generate SBOM
	sbom, err := security.GenerateSBOM(packages, "ophid-local")
	if err != nil {
		fmt.Printf("Warning: SBOM generation failed: %v\n", err)
	} else {
		sbomPath := filepath.Join(path, "ophid-sbom.json")
		if err := security.WriteSBOM(sbom, sbomPath); err != nil {
			fmt.Printf("Warning: failed to write SBOM: %v\n", err)
		} else {
			secInfo.SBOMPath = sbomPath
			fmt.Printf("[SUCCESS] SBOM generated: %s\n", sbomPath)
		}
	}

	return secInfo, nil
}

// DetectEcosystem detects the ecosystem of a local directory
func (li *LocalInstaller) DetectEcosystem(path string) string {
	// Check for Python
	if li.fileExists(path, "setup.py") ||
		li.fileExists(path, "pyproject.toml") ||
		li.fileExists(path, "requirements.txt") {
		return "python"
	}

	// Check for Go
	if li.fileExists(path, "go.mod") {
		return "go"
	}

	// Check for Node.js
	if li.fileExists(path, "package.json") {
		return "node"
	}

	// Check for Ruby
	if li.fileExists(path, "Gemfile") {
		return "ruby"
	}

	// Check for Rust
	if li.fileExists(path, "Cargo.toml") {
		return "rust"
	}

	return "unknown"
}

// ExtractMetadata extracts metadata from local project
func (li *LocalInstaller) ExtractMetadata(path string) map[string]string {
	metadata := make(map[string]string)
	metadata["source_path"] = path

	// Try to detect version from common files
	ecosystem := li.DetectEcosystem(path)
	metadata["ecosystem"] = ecosystem

	// Add project name from directory
	metadata["project_name"] = filepath.Base(path)

	return metadata
}

// fileExists checks if a file exists in a directory
func (li *LocalInstaller) fileExists(dir, filename string) bool {
	_, err := os.Stat(filepath.Join(dir, filename))
	return err == nil
}

// parseDependencyFile parses a dependency file
func (li *LocalInstaller) parseDependencyFile(filePath string) ([]security.Package, error) {
	if strings.HasSuffix(filePath, "requirements.txt") {
		return security.ParseRequirementsTxt(filePath)
	} else if strings.HasSuffix(filePath, "go.mod") {
		return security.ParseGoMod(filePath)
	} else if strings.HasSuffix(filePath, "package.json") {
		return security.ParsePackageJSON(filePath)
	}
	return nil, fmt.Errorf("unsupported dependency file: %s", filePath)
}
