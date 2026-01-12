package runtime

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Runtime represents a runtime interpreter installation (Python, Node, Bun, etc.)
type Runtime struct {
	Type       RuntimeType
	Version    string
	Path       string
	OS         string
	Arch       string
	Downloaded time.Time
}

// Manager manages runtime installations (Python, Node, Bun, etc.)
type Manager struct {
	homeDir    string
	downloader *Downloader
	verifier   *Verifier
	extractor  *Extractor
	platform   Platform
}

// NewManager creates a new runtime manager
func NewManager(homeDir string) *Manager {
	cacheDir := filepath.Join(homeDir, "cache", "downloads")
	platform := DetectPlatform()

	return &Manager{
		homeDir:    homeDir,
		downloader: NewDownloader(cacheDir),
		verifier:   NewVerifier(),
		extractor:  NewExtractor(),
		platform:   platform,
	}
}

// Install downloads and installs a runtime from a specification string
// Accepts: "python@3.12.1", "node@20.0.0", or "3.12.1" (defaults to Python)
func (m *Manager) Install(specString string) (*Runtime, error) {
	// Parse runtime specification
	spec, err := ParseRuntimeSpec(specString)
	if err != nil {
		return nil, fmt.Errorf("invalid runtime specification: %w", err)
	}

	return m.InstallFromSpec(spec)
}

// InstallFromSpec downloads and installs a runtime from a RuntimeSpec
func (m *Manager) InstallFromSpec(spec *RuntimeSpec) (*Runtime, error) {
	slog.Info("installing runtime",
		"type", spec.Type.DisplayName(),
		"version", spec.Version,
		"platform", m.platform.String())

	// 1. Check if already installed
	runtimePath := filepath.Join(m.homeDir, "runtimes", fmt.Sprintf("%s-%s", spec.Type, spec.Version))
	if _, err := os.Stat(runtimePath); err == nil {
		slog.Info("runtime already installed",
			"type", spec.Type.DisplayName(),
			"version", spec.Version)
		return &Runtime{
			Type:       spec.Type,
			Version:    spec.Version,
			Path:       runtimePath,
			OS:         m.platform.OS,
			Arch:       m.platform.Arch,
			Downloaded: time.Now(), // Approximate
		}, nil
	}

	// 2. Download runtime based on type
	switch spec.Type {
	case RuntimePython:
		return m.installPython(spec, runtimePath)
	case RuntimeNode:
		return m.installNodeJS(spec, runtimePath)
	default:
		return nil, fmt.Errorf("runtime type %s is not yet implemented", spec.Type.DisplayName())
	}
}

// installPython installs Python runtime from python-build-standalone
func (m *Manager) installPython(spec *RuntimeSpec, runtimePath string) (*Runtime, error) {
	// Download Python standalone build
	tarballPath, err := m.downloader.Download(spec.Version)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Verify checksum
	slog.Info("verifying download integrity", "file", tarballPath)
	if err := m.verifier.VerifyFileExists(tarballPath); err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	// Get expected SHA256 hash from GitHub releases
	expectedHash, err := m.verifier.GetSHA256ForVersion(spec.Version, m.platform, pythonBuildDate)
	if err != nil {
		slog.Warn("failed to fetch SHA256 hash, skipping integrity check",
			"error", err,
			"version", spec.Version)
	} else {
		// Verify SHA256 hash
		slog.Info("verifying SHA256 checksum", "version", spec.Version)
		if err := m.verifier.VerifySHA256(tarballPath, expectedHash); err != nil {
			return nil, fmt.Errorf("SHA256 verification failed: %w\nThis indicates the download may be corrupted or tampered with", err)
		}
		slog.Info("SHA256 verification passed")
	}

	// Extract to ~/.ophid/runtimes
	slog.Info("extracting runtime", "type", spec.Type.DisplayName(), "destination", runtimePath)
	if err := m.extractor.Extract(tarballPath, runtimePath); err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}

	slog.Info("runtime installed successfully",
		"type", spec.Type.DisplayName(),
		"version", spec.Version,
		"path", runtimePath)

	return &Runtime{
		Type:       spec.Type,
		Version:    spec.Version,
		Path:       runtimePath,
		OS:         m.platform.OS,
		Arch:       m.platform.Arch,
		Downloaded: time.Now(),
	}, nil
}

// installNodeJS installs Node.js runtime from official distributions
func (m *Manager) installNodeJS(spec *RuntimeSpec, runtimePath string) (*Runtime, error) {
	// Download Node.js from official distribution
	tarballPath, err := m.downloader.DownloadNodeJS(spec.Version, m.platform)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Verify file exists
	slog.Info("verifying download integrity", "file", tarballPath)
	if err := m.verifier.VerifyFileExists(tarballPath); err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	// Extract to ~/.ophid/runtimes
	slog.Info("extracting runtime", "type", spec.Type.DisplayName(), "destination", runtimePath)
	if err := m.extractor.Extract(tarballPath, runtimePath); err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}

	slog.Info("runtime installed successfully",
		"type", spec.Type.DisplayName(),
		"version", spec.Version,
		"path", runtimePath)

	return &Runtime{
		Type:       spec.Type,
		Version:    spec.Version,
		Path:       runtimePath,
		OS:         m.platform.OS,
		Arch:       m.platform.Arch,
		Downloaded: time.Now(),
	}, nil
}

// List lists installed runtimes (Python, Node, Bun, etc.)
func (m *Manager) List() ([]*Runtime, error) {
	runtimesDir := filepath.Join(m.homeDir, "runtimes")

	// Check if runtimes directory exists
	if _, err := os.Stat(runtimesDir); os.IsNotExist(err) {
		return []*Runtime{}, nil
	}

	// Read directory
	entries, err := os.ReadDir(runtimesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read runtimes dir: %w", err)
	}

	runtimes := []*Runtime{}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Extract type and version from directory name (python-3.12.1, node-20.0.0, etc.)
		name := entry.Name()
		parts := strings.SplitN(name, "-", 2)
		if len(parts) == 2 {
			runtimeType := RuntimeType(parts[0])
			version := parts[1]

			// Skip if runtime type is not valid
			if !runtimeType.IsValid() {
				continue
			}

			info, _ := entry.Info()

			runtimes = append(runtimes, &Runtime{
				Type:       runtimeType,
				Version:    version,
				Path:       filepath.Join(runtimesDir, name),
				OS:         m.platform.OS,
				Arch:       m.platform.Arch,
				Downloaded: info.ModTime(),
			})
		}
	}

	return runtimes, nil
}

// Get retrieves a specific runtime
// Accepts: "python@3.12.1", "node@20.0.0", or "3.12.1" (defaults to Python)
func (m *Manager) Get(specString string) (*Runtime, error) {
	// Parse runtime specification
	spec, err := ParseRuntimeSpec(specString)
	if err != nil {
		return nil, fmt.Errorf("invalid runtime specification: %w", err)
	}

	runtimePath := filepath.Join(m.homeDir, "runtimes", fmt.Sprintf("%s-%s", spec.Type, spec.Version))

	if _, err := os.Stat(runtimePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s %s is not installed", spec.Type.DisplayName(), spec.Version)
	}

	info, err := os.Stat(runtimePath)
	if err != nil {
		return nil, err
	}

	return &Runtime{
		Type:       spec.Type,
		Version:    spec.Version,
		Path:       runtimePath,
		OS:         m.platform.OS,
		Arch:       m.platform.Arch,
		Downloaded: info.ModTime(),
	}, nil
}

// Remove removes a runtime (Python, Node, Bun, etc.)
// Accepts: "python@3.12.1", "node@20.0.0", or "3.12.1" (defaults to Python)
func (m *Manager) Remove(specString string) error {
	// Parse runtime specification
	spec, err := ParseRuntimeSpec(specString)
	if err != nil {
		return fmt.Errorf("invalid runtime specification: %w", err)
	}

	runtimePath := filepath.Join(m.homeDir, "runtimes", fmt.Sprintf("%s-%s", spec.Type, spec.Version))

	if _, err := os.Stat(runtimePath); os.IsNotExist(err) {
		return fmt.Errorf("%s %s is not installed", spec.Type.DisplayName(), spec.Version)
	}

	if err := os.RemoveAll(runtimePath); err != nil {
		return fmt.Errorf("failed to remove runtime: %w", err)
	}

	slog.Info("runtime removed",
		"type", spec.Type.DisplayName(),
		"version", spec.Version,
		"path", runtimePath)

	fmt.Printf("✓ %s %s removed\n", spec.Type.DisplayName(), spec.Version)
	return nil
}

// EnsureRuntime ensures a Python runtime matching requirements is available
func (m *Manager) EnsureRuntime(requirement string) (*Runtime, error) {
	// TODO: Parse requirement (e.g., ">=3.10,<4")
	// For now, just use the requirement as an exact version
	version := requirement

	// Check if already installed
	runtime, err := m.Get(version)
	if err == nil {
		return runtime, nil
	}

	// Not installed, install it
	return m.Install(version)
}
