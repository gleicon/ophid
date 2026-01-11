package tool

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// SourceDetector detects and parses installation sources
type SourceDetector struct{}

// NewSourceDetector creates a new source detector
func NewSourceDetector() *SourceDetector {
	return &SourceDetector{}
}

// DetectSource detects the source type from a package specification
// Examples:
//   - "ansible" -> PyPI
//   - "github.com/user/repo" -> GitHub
//   - "https://github.com/user/repo" -> GitHub
//   - "git+https://example.com/repo.git" -> Git
//   - "./mypackage" or "/absolute/path" -> Local
//   - "file:///path/to/package" -> Local
func (sd *SourceDetector) DetectSource(spec string, opts InstallOptions) (InstallSource, error) {
	// If source is explicitly provided, use it
	if opts.Source.Type != "" {
		return opts.Source, nil
	}

	// Local path detection
	if sd.isLocalPath(spec) {
		absPath, err := filepath.Abs(spec)
		if err != nil {
			return InstallSource{}, fmt.Errorf("invalid local path: %w", err)
		}

		if _, err := os.Stat(absPath); err != nil {
			return InstallSource{}, fmt.Errorf("local path does not exist: %s", absPath)
		}

		return InstallSource{
			Type: SourceLocal,
			Path: absPath,
		}, nil
	}

	// Git URL detection (git+https://, git+ssh://, git://)
	if strings.HasPrefix(spec, "git+") || strings.HasPrefix(spec, "git://") {
		return sd.parseGitURL(spec)
	}

	// GitHub URL detection
	if sd.isGitHubURL(spec) {
		return sd.parseGitHubURL(spec)
	}

	// GitHub shorthand (github.com/user/repo or user/repo)
	if sd.isGitHubShorthand(spec) {
		return sd.parseGitHubShorthand(spec)
	}

	// HTTP(S) URL - could be a Git repo or direct download
	if strings.HasPrefix(spec, "http://") || strings.HasPrefix(spec, "https://") {
		return sd.parseHTTPURL(spec)
	}

	// file:// URL
	if strings.HasPrefix(spec, "file://") {
		path := strings.TrimPrefix(spec, "file://")
		absPath, err := filepath.Abs(path)
		if err != nil {
			return InstallSource{}, fmt.Errorf("invalid file URL: %w", err)
		}

		return InstallSource{
			Type: SourceLocal,
			Path: absPath,
		}, nil
	}

	// Default: assume PyPI package
	return InstallSource{
		Type: SourcePyPI,
		URL:  spec,
	}, nil
}

// isLocalPath checks if the spec is a local file path
func (sd *SourceDetector) isLocalPath(spec string) bool {
	// Relative paths
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") {
		return true
	}

	// Absolute paths
	if strings.HasPrefix(spec, "/") || (len(spec) > 1 && spec[1] == ':') {
		return true
	}

	// Check if it's an existing directory or file
	if _, err := os.Stat(spec); err == nil {
		return true
	}

	return false
}

// isGitHubURL checks if the spec is a GitHub URL
func (sd *SourceDetector) isGitHubURL(spec string) bool {
	return strings.Contains(spec, "github.com")
}

// isGitHubShorthand checks if the spec is GitHub shorthand (user/repo)
func (sd *SourceDetector) isGitHubShorthand(spec string) bool {
	// Must have exactly one slash and no spaces
	parts := strings.Split(spec, "/")
	if len(parts) != 2 {
		return false
	}

	// Both parts must be non-empty and alphanumeric (with hyphens/underscores)
	for _, part := range parts {
		if len(part) == 0 {
			return false
		}
		for _, c := range part {
			if !isAlphanumericOrDash(c) {
				return false
			}
		}
	}

	return true
}

// parseGitURL parses a git:// or git+https:// URL
func (sd *SourceDetector) parseGitURL(spec string) (InstallSource, error) {
	// Remove git+ prefix if present
	gitURL := strings.TrimPrefix(spec, "git+")

	// Parse URL and extract ref if present (e.g., url@branch or url#commit)
	source := InstallSource{
		Type: SourceGit,
		URL:  gitURL,
	}

	// Check for @ref syntax
	if strings.Contains(gitURL, "@") {
		parts := strings.SplitN(gitURL, "@", 2)
		source.URL = parts[0]
		source.Branch = parts[1]
	}

	// Check for #ref syntax (fragment)
	parsedURL, err := url.Parse(source.URL)
	if err != nil {
		return source, fmt.Errorf("invalid git URL: %w", err)
	}

	if parsedURL.Fragment != "" {
		source.Commit = parsedURL.Fragment
		parsedURL.Fragment = ""
		source.URL = parsedURL.String()
	}

	return source, nil
}

// parseGitHubURL parses a GitHub URL
func (sd *SourceDetector) parseGitHubURL(spec string) (InstallSource, error) {
	parsedURL, err := url.Parse(spec)
	if err != nil {
		return InstallSource{}, fmt.Errorf("invalid GitHub URL: %w", err)
	}

	// Extract user/repo from path
	path := strings.TrimPrefix(parsedURL.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	source := InstallSource{
		Type: SourceGitHub,
		URL:  fmt.Sprintf("https://github.com/%s.git", path),
	}

	// Extract branch/tag from fragment
	if parsedURL.Fragment != "" {
		source.Branch = parsedURL.Fragment
	}

	return source, nil
}

// parseGitHubShorthand parses GitHub shorthand (user/repo)
func (sd *SourceDetector) parseGitHubShorthand(spec string) (InstallSource, error) {
	// Remove any @ suffix for version/ref
	ref := ""
	if strings.Contains(spec, "@") {
		parts := strings.SplitN(spec, "@", 2)
		spec = parts[0]
		ref = parts[1]
	}

	return InstallSource{
		Type:   SourceGitHub,
		URL:    fmt.Sprintf("https://github.com/%s.git", spec),
		Branch: ref,
	}, nil
}

// parseHTTPURL parses an HTTP(S) URL
func (sd *SourceDetector) parseHTTPURL(spec string) (InstallSource, error) {
	// Check if it ends with .git
	if strings.HasSuffix(spec, ".git") {
		if strings.Contains(spec, "github.com") {
			return sd.parseGitHubURL(spec)
		}
		return InstallSource{
			Type: SourceGit,
			URL:  spec,
		}, nil
	}

	// For now, treat as Git repository
	return InstallSource{
		Type: SourceGit,
		URL:  spec,
	}, nil
}

// isAlphanumericOrDash checks if a rune is alphanumeric, dash, or underscore
func isAlphanumericOrDash(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.'
}