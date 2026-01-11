package tool

import (
	"time"
)

// SourceType represents the installation source type
type SourceType string

const (
	SourcePyPI   SourceType = "pypi"   // PyPI package registry
	SourceGitHub SourceType = "github" // GitHub repository
	SourceGit    SourceType = "git"    // Generic Git repository
	SourceLocal  SourceType = "local"  // Local directory
	SourceNPM    SourceType = "npm"    // NPM package registry
)

// InstallSource describes where a package comes from
type InstallSource struct {
	Type       SourceType        `json:"type"`
	URL        string            `json:"url,omitempty"`         // Git URL or registry URL
	Path       string            `json:"path,omitempty"`        // Local path
	Branch     string            `json:"branch,omitempty"`      // Git branch
	Tag        string            `json:"tag,omitempty"`         // Git tag
	Commit     string            `json:"commit,omitempty"`      // Git commit SHA
	Subdirectory string          `json:"subdirectory,omitempty"` // Subdirectory within source
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// SecurityInfo tracks security scan results
type SecurityInfo struct {
	SBOMPath         string    `json:"sbom_path,omitempty"`
	VulnScanDate     time.Time `json:"vuln_scan_date,omitempty"`
	VulnCount        int       `json:"vuln_count"`
	CriticalVulnCount int      `json:"critical_vuln_count"`
	LicenseCompliant bool      `json:"license_compliant"`
	Licenses         []string  `json:"licenses,omitempty"`
}

// Tool represents an installed tool
type Tool struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Ecosystem   string            `json:"ecosystem"` // "python", "node", "ruby", "go"
	Runtime     string            `json:"runtime"`   // Runtime version requirement
	InstallPath string            `json:"install_path"`
	Executables []string          `json:"executables"` // List of executable names
	Source      InstallSource     `json:"source"`      // Installation source
	Security    SecurityInfo      `json:"security"`    // Security scan information
	Metadata    map[string]string `json:"metadata,omitempty"`
	InstalledAt time.Time         `json:"installed_at"`
	UpdatedAt   time.Time         `json:"updated_at,omitempty"`
}

// InstallOptions configures tool installation
type InstallOptions struct {
	// Common options
	Version      string   // Specific version or "latest"
	Force        bool     // Force reinstall
	SkipScan     bool     // Skip security scanning (not recommended)
	RequireScan  bool     // Require security scan to pass (default: warn only)

	// Source specification
	Source       InstallSource // Installation source (auto-detected if empty)

	// Python-specific
	Extras       []string // Python extras (e.g., "security" for requests[security])
	Editable     bool     // Install in editable mode (-e for pip)
	NoDeps       bool     // Don't install dependencies
	Requirements string   // Path to requirements.txt

	// Git/GitHub-specific
	GitRef       string   // Git reference (branch, tag, or commit)
	Subdirectory string   // Subdirectory within repository

	// Local-specific
	LocalPath    string   // Local directory path
}

// ToolManifest tracks all installed tools
type ToolManifest struct {
	Tools      map[string]*Tool `json:"tools"` // tool name -> Tool
	UpdatedAt  time.Time        `json:"updated_at"`
}
