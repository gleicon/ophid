package security

import (
	"context"
	"time"
)

// SecretFinding represents a detected secret
type SecretFinding struct {
	Type        string  `json:"type"`        // e.g., "aws-access-key"
	Description string  `json:"description"` // Human-readable description
	File        string  `json:"file"`        // File path where found
	Line        int     `json:"line"`        // Line number
	Secret      string  `json:"-"`           // Redacted from JSON
	Match       string  `json:"match"`       // Pattern that matched
	Entropy     float64 `json:"entropy"`     // Entropy score
	Severity    string  `json:"severity"`    // "critical", "high", "medium"
}

// SecretsReport contains results of secret scanning
type SecretsReport struct {
	Path            string          `json:"path"`
	ScanDate        time.Time       `json:"scan_date"`
	Findings        []SecretFinding `json:"findings"`
	FilesScanned    int             `json:"files_scanned"`
	TotalSecrets    int             `json:"total_secrets"`
	CriticalSecrets int             `json:"critical_secrets"`
}

// HasSecrets returns true if any secrets were found
func (sr *SecretsReport) HasSecrets() bool {
	return len(sr.Findings) > 0
}

// HasCriticalSecrets returns true if critical secrets were found
func (sr *SecretsReport) HasCriticalSecrets() bool {
	return sr.CriticalSecrets > 0
}

// SecretScanner interface for scanning secrets
type SecretScanner interface {
	Scan(ctx context.Context, path string) (*SecretsReport, error)
	ScanFile(ctx context.Context, filePath string) ([]SecretFinding, error)
}

// RedactSecret partially redacts a secret for safe display
// Adapted from mcp-osv redaction pattern
func RedactSecret(secret string) string {
	if len(secret) <= 8 {
		return "***REDACTED***"
	}
	return secret[:4] + "***" + secret[len(secret)-4:]
}

// ClassifySecretSeverity determines severity based on secret type
func ClassifySecretSeverity(ruleID string) string {
	criticalTypes := map[string]bool{
		"aws-access-token":    true,
		"github-pat":          true,
		"private-key":         true,
		"slack-webhook-url":   true,
		"stripe-access-token": true,
		"generic-api-key":     true,
	}

	if criticalTypes[ruleID] {
		return "critical"
	}
	return "high"
}
