package security

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParseRequirementsTxt parses a Python requirements.txt file
func ParseRequirementsTxt(path string) ([]Package, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open requirements.txt: %w", err)
	}
	defer file.Close()

	packages := []Package{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse package line
		pkg, err := parseRequirementLine(line)
		if err != nil {
			continue // Skip invalid lines
		}

		packages = append(packages, pkg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return packages, nil
}

// parseRequirementLine parses a single requirements.txt line
func parseRequirementLine(line string) (Package, error) {
	// Remove inline comments
	if idx := strings.Index(line, "#"); idx != -1 {
		line = line[:idx]
	}
	line = strings.TrimSpace(line)

	// Handle various formats:
	// - package==1.0.0
	// - package>=1.0.0
	// - package~=1.0.0
	// - package

	var name, version string

	for _, sep := range []string{"==", ">=", "<=", "~=", ">", "<"} {
		if idx := strings.Index(line, sep); idx != -1 {
			name = strings.TrimSpace(line[:idx])
			version = strings.TrimSpace(line[idx+len(sep):])
			break
		}
	}

	// If no version specifier, entire line is the name
	if name == "" {
		name = line
		version = "latest" // OSV.dev can handle this
	}

	// Handle extras: package[extra]==1.0.0
	if idx := strings.Index(name, "["); idx != -1 {
		name = name[:idx]
	}

	if name == "" {
		return Package{}, fmt.Errorf("empty package name")
	}

	return Package{
		Name:      name,
		Version:   version,
		Ecosystem: "PyPI",
	}, nil
}

// ParsePackageJSON parses a package.json file
func ParsePackageJSON(path string) ([]Package, error) {
	// TODO: Implement package.json parsing
	// For now, return empty list
	return []Package{}, fmt.Errorf("package.json parsing not yet implemented")
}

// ParseGoMod parses a go.mod file
// Adapted from mcp-osv pattern
func ParseGoMod(path string) ([]Package, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open go.mod: %w", err)
	}
	defer file.Close()

	packages := []Package{}
	scanner := bufio.NewScanner(file)
	inRequire := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Detect require block
		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}

		if inRequire && line == ")" {
			inRequire = false
			continue
		}

		// Parse require lines
		if inRequire || strings.HasPrefix(line, "require ") {
			pkg, err := parseGoModLine(line)
			if err != nil {
				continue
			}
			packages = append(packages, pkg)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return packages, nil
}

func parseGoModLine(line string) (Package, error) {
	// Remove "require " prefix
	line = strings.TrimPrefix(line, "require ")
	line = strings.TrimSpace(line)

	// Split into name and version
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return Package{}, fmt.Errorf("invalid go.mod line")
	}

	name := parts[0]
	version := parts[1]

	return Package{
		Name:      name,
		Version:   version,
		Ecosystem: "Go",
	}, nil
}
