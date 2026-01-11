package security

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SBOM represents a Software Bill of Materials in CycloneDX format
type SBOM struct {
	BOMFormat   string       `json:"bomFormat"`
	SpecVersion string       `json:"specVersion"`
	Version     int          `json:"version"`
	Metadata    SBOMMetadata `json:"metadata"`
	Components  []Component  `json:"components"`
}

// SBOMMetadata contains SBOM metadata
type SBOMMetadata struct {
	Timestamp string        `json:"timestamp"`
	Tools     []SBOMTool    `json:"tools"`
	Component *Component    `json:"component,omitempty"`
}

// SBOMTool represents the tool that generated the SBOM
type SBOMTool struct {
	Vendor  string `json:"vendor"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Component represents a software component
type Component struct {
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	PackageURL string            `json:"purl,omitempty"`
	Licenses   []License         `json:"licenses,omitempty"`
	Hashes     []Hash            `json:"hashes,omitempty"`
	ExternalRefs []ExternalRef   `json:"externalReferences,omitempty"`
}

// License represents a software license
type License struct {
	License LicenseChoice `json:"license"`
}

// LicenseChoice represents a license choice
type LicenseChoice struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Hash represents a cryptographic hash
type Hash struct {
	Algorithm string `json:"alg"`
	Content   string `json:"content"`
}

// ExternalRef represents an external reference
type ExternalRef struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// GenerateSBOM generates an SBOM from packages
func GenerateSBOM(packages []Package, toolName string) (*SBOM, error) {
	components := make([]Component, 0, len(packages))

	for _, pkg := range packages {
		component := Component{
			Type:       "library",
			Name:       pkg.Name,
			Version:    pkg.Version,
			PackageURL: buildPURL(pkg),
		}

		components = append(components, component)
	}

	sbom := &SBOM{
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.4",
		Version:     1,
		Metadata: SBOMMetadata{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Tools: []SBOMTool{
				{
					Vendor:  "OPHID",
					Name:    "ophid",
					Version: "0.1.0",
				},
			},
		},
		Components: components,
	}

	return sbom, nil
}

// WriteSBOM writes an SBOM to a file
func WriteSBOM(sbom *SBOM, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(sbom); err != nil {
		return fmt.Errorf("failed to encode SBOM: %w", err)
	}

	return nil
}

// buildPURL builds a Package URL (purl) for a package
func buildPURL(pkg Package) string {
	// Package URL format: pkg:<type>/<namespace>/<name>@<version>
	ecosystem := pkg.Ecosystem
	switch ecosystem {
	case "PyPI":
		return fmt.Sprintf("pkg:pypi/%s@%s", pkg.Name, pkg.Version)
	case "npm":
		return fmt.Sprintf("pkg:npm/%s@%s", pkg.Name, pkg.Version)
	case "Go":
		return fmt.Sprintf("pkg:golang/%s@%s", pkg.Name, pkg.Version)
	default:
		return fmt.Sprintf("pkg:generic/%s@%s", pkg.Name, pkg.Version)
	}
}
