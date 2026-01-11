package security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zricethezav/gitleaks/v8/detect"
	"github.com/zricethezav/gitleaks/v8/report"
)

// GitLeaksScanner implements SecretScanner using gitleaks v8
type GitLeaksScanner struct {
	detector *detect.Detector
}

// NewGitLeaksScanner creates a new gitleaks-based secret scanner
func NewGitLeaksScanner() (*GitLeaksScanner, error) {
	// Initialize with default config (100+ built-in rules)
	detector, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize gitleaks detector: %w", err)
	}

	return &GitLeaksScanner{
		detector: detector,
	}, nil
}

// Scan scans a file or directory for secrets
func (gs *GitLeaksScanner) Scan(ctx context.Context, path string) (*SecretsReport, error) {
	report := &SecretsReport{
		Path:     path,
		ScanDate: time.Now(),
		Findings: []SecretFinding{},
	}

	// Determine if path is file or directory
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	var filesToScan []string

	if fileInfo.IsDir() {
		// Walk directory tree
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && isScannableFile(filePath) {
				filesToScan = append(filesToScan, filePath)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	} else if isScannableFile(path) {
		filesToScan = append(filesToScan, path)
	}

	report.FilesScanned = len(filesToScan)

	// Scan each file
	for _, filePath := range filesToScan {
		findings, err := gs.ScanFile(ctx, filePath)
		if err != nil {
			// Log warning but continue
			fmt.Printf("Warning: failed to scan %s: %v\n", filePath, err)
			continue
		}
		report.Findings = append(report.Findings, findings...)
	}

	// Count totals
	report.TotalSecrets = len(report.Findings)
	for _, finding := range report.Findings {
		if finding.Severity == "critical" {
			report.CriticalSecrets++
		}
	}

	return report, nil
}

// ScanFile scans a single file for secrets
func (gs *GitLeaksScanner) ScanFile(ctx context.Context, filePath string) ([]SecretFinding, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Create fragment for gitleaks
	fragment := detect.Fragment{
		Raw:      string(content),
		FilePath: filePath,
	}

	// Detect secrets
	findings := gs.detector.Detect(fragment)

	// Convert to our format
	return convertGitleaksFindings(findings), nil
}

// convertGitleaksFindings converts gitleaks findings to our format
func convertGitleaksFindings(findings []report.Finding) []SecretFinding {
	result := make([]SecretFinding, len(findings))

	for i, f := range findings {
		result[i] = SecretFinding{
			Type:        f.RuleID,
			Description: f.Description,
			File:        f.File,
			Line:        f.StartLine,
			Secret:      f.Secret,
			Match:       f.Match,
			Entropy:     float64(f.Entropy), // Convert float32 to float64
			Severity:    ClassifySecretSeverity(f.RuleID),
		}
	}

	return result
}

// isScannableFile determines if a file should be scanned
// Based on mcp-osv pattern but expanded for ophid
func isScannableFile(path string) bool {
	// Skip hidden files and directories
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") && base != ".env" {
		return false
	}

	// Skip binary and archive files
	skipExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".pdf": true, ".pyc": true, ".pyo": true,
	}

	ext := strings.ToLower(filepath.Ext(path))
	if skipExts[ext] {
		return false
	}

	// Scan common config and code files
	scannableExts := map[string]bool{
		".go":     true, ".py":     true, ".js":     true,
		".ts":     true, ".json":   true, ".yaml":   true,
		".yml":    true, ".toml":   true, ".env":    true,
		".sh":     true, ".bash":   true, ".txt":    true,
		".md":     true, ".conf":   true, ".config": true,
		".ini":    true, ".properties": true,
	}

	// Check common files without extensions
	if base == "Dockerfile" || base == "Makefile" || base == "requirements.txt" {
		return true
	}

	return scannableExts[ext]
}
