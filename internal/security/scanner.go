package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	osvAPIURL = "https://api.osv.dev/v1/query"
)

// OSVResponse represents the response from OSV.dev API
// Adapted from mcp-osv
type OSVResponse struct {
	Vulns []OSVVulnerability `json:"vulns"`
}

// OSVVulnerability represents a vulnerability from OSV.dev
type OSVVulnerability struct {
	ID       string                `json:"id"`
	Summary  string                `json:"summary"`
	Details  string                `json:"details"`
	Affected []OSVAffected         `json:"affected"`
	Severity []OSVSeverity         `json:"severity,omitempty"`
	Modified string                `json:"modified"`
	Published string               `json:"published"`
	References []OSVReference      `json:"references,omitempty"`
}

// OSVAffected represents affected packages
type OSVAffected struct {
	Package OSVPackage `json:"package"`
	Ranges  []OSVRange `json:"ranges,omitempty"`
	Versions []string  `json:"versions,omitempty"`
}

// OSVPackage represents a package
type OSVPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

// OSVRange represents version ranges
type OSVRange struct {
	Type   string      `json:"type"`
	Events []OSVEvent  `json:"events"`
}

// OSVEvent represents a range event
type OSVEvent struct {
	Introduced string `json:"introduced,omitempty"`
	Fixed      string `json:"fixed,omitempty"`
}

// OSVSeverity represents severity information
type OSVSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

// OSVReference represents external references
type OSVReference struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// QueryRequest represents a query to OSV.dev
type QueryRequest struct {
	Package *PackageQuery `json:"package,omitempty"`
	Version string        `json:"version,omitempty"`
}

// PackageQuery specifies the package to query
type PackageQuery struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

// RateLimiter provides rate limiting for API calls
// Adapted from mcp-osv
type RateLimiter struct {
	limiter *rate.Limiter
	mu      sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rps float64) *RateLimiter {
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(rps), 1),
	}
}

// Wait waits until rate limit allows next request
func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.limiter.Wait(ctx)
}

// Scanner handles vulnerability scanning
type Scanner struct {
	client        *http.Client
	rateLimiter   *RateLimiter
	secretScanner SecretScanner
}

// NewScanner creates a new vulnerability scanner
func NewScanner() *Scanner {
	secretScanner, err := NewGitLeaksScanner()
	if err != nil {
		fmt.Printf("[WARN] failed to initialize secret scanner: %v\n", err)
		secretScanner = nil
	}

	return &Scanner{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		},
		rateLimiter:   NewRateLimiter(1.0), // 1 request per second, same as mcp-osv
		secretScanner: secretScanner,
	}
}

// ScanPackage scans a single package for vulnerabilities
func (s *Scanner) ScanPackage(ctx context.Context, ecosystem, name, version string) (*OSVResponse, error) {
	// Input validation (from mcp-osv pattern)
	if err := validatePackageName(name); err != nil {
		return nil, fmt.Errorf("invalid package name: %w", err)
	}
	if err := validateVersion(version); err != nil {
		return nil, fmt.Errorf("invalid version: %w", err)
	}

	// Rate limit
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	// Build request
	req := QueryRequest{
		Package: &PackageQuery{
			Name:      name,
			Ecosystem: ecosystem,
		},
		Version: version,
	}

	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", osvAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "ophid/0.1.0")

	// Execute request
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OSV API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var osvResp OSVResponse
	if err := json.NewDecoder(resp.Body).Decode(&osvResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &osvResp, nil
}

// ScanPackages scans multiple packages
func (s *Scanner) ScanPackages(ctx context.Context, packages []Package) ([]ScanResult, error) {
	results := make([]ScanResult, 0, len(packages))

	for _, pkg := range packages {
		resp, err := s.ScanPackage(ctx, pkg.Ecosystem, pkg.Name, pkg.Version)
		if err != nil {
			results = append(results, ScanResult{
				Package: pkg,
				Error:   err.Error(),
			})
			continue
		}

		results = append(results, ScanResult{
			Package:         pkg,
			Vulnerabilities: resp.Vulns,
		})
	}

	return results, nil
}

// ScanResult represents the scan result for a package
type ScanResult struct {
	Package         Package
	Vulnerabilities []OSVVulnerability
	Error           string
}

// HasVulnerabilities returns true if any vulnerabilities were found
func (sr *ScanResult) HasVulnerabilities() bool {
	return len(sr.Vulnerabilities) > 0
}

// CriticalCount returns the number of critical vulnerabilities
func (sr *ScanResult) CriticalCount() int {
	count := 0
	for _, vuln := range sr.Vulnerabilities {
		for _, sev := range vuln.Severity {
			if sev.Type == "CVSS_V3" && strings.HasPrefix(sev.Score, "CVSS:3") {
				// Parse score (simplified - production should use proper CVSS parser)
				if strings.Contains(sev.Score, "/C:H") || strings.Contains(sev.Score, "/9.") {
					count++
				}
			}
		}
	}
	return count
}

// Input validation functions (adapted from mcp-osv)
func validatePackageName(name string) error {
	if name == "" {
		return fmt.Errorf("package name cannot be empty")
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") {
		return fmt.Errorf("package name contains invalid characters")
	}
	if len(name) > 256 {
		return fmt.Errorf("package name too long")
	}
	return nil
}

func validateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}
	if strings.Contains(version, "..") {
		return fmt.Errorf("version contains invalid characters")
	}
	if len(version) > 64 {
		return fmt.Errorf("version too long")
	}
	return nil
}

// ScanSecrets scans path for secrets
func (s *Scanner) ScanSecrets(ctx context.Context, path string) (*SecretsReport, error) {
	if s.secretScanner == nil {
		return &SecretsReport{Path: path}, fmt.Errorf("secret scanner not initialized")
	}
	return s.secretScanner.Scan(ctx, path)
}
