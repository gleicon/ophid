package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Runtime represents a Python interpreter installation
type Runtime struct {
	Version    string
	Path       string
	OS         string
	Arch       string
	Downloaded time.Time
}

// Manager manages Python runtime installations
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

// Install downloads and installs a Python runtime
func (m *Manager) Install(version string) (*Runtime, error) {
	fmt.Printf(" Installing Python %s for %s\n", version, m.platform)

	// 1. Check if already installed
	runtimePath := filepath.Join(m.homeDir, "runtimes", fmt.Sprintf("python-%s", version))
	if _, err := os.Stat(runtimePath); err == nil {
		fmt.Printf(" Python %s already installed\n", version)
		return &Runtime{
			Version:    version,
			Path:       runtimePath,
			OS:         m.platform.OS,
			Arch:       m.platform.Arch,
			Downloaded: time.Now(), // Approximate
		}, nil
	}

	// 2. Download Python standalone build
	tarballPath, err := m.downloader.Download(version)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// 3. Verify checksum (optional for now - TODO)
	fmt.Println(" Verifying download...")
	if err := m.verifier.VerifyFileExists(tarballPath); err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}
	// TODO: Add SHA256 verification when we have checksums

	// 4. Extract to ~/.ophid/runtimes
	fmt.Printf(" Extracting to %s\n", runtimePath)
	if err := m.extractor.Extract(tarballPath, runtimePath); err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}

	fmt.Printf(" Python %s installed successfully\n", version)

	// 5. Return Runtime struct
	return &Runtime{
		Version:    version,
		Path:       runtimePath,
		OS:         m.platform.OS,
		Arch:       m.platform.Arch,
		Downloaded: time.Now(),
	}, nil
}

// List lists installed Python runtimes
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

		// Extract version from directory name (python-3.12.1)
		name := entry.Name()
		if len(name) > 7 && name[:7] == "python-" {
			version := name[7:]
			info, _ := entry.Info()

			runtimes = append(runtimes, &Runtime{
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
func (m *Manager) Get(version string) (*Runtime, error) {
	runtimePath := filepath.Join(m.homeDir, "runtimes", fmt.Sprintf("python-%s", version))

	if _, err := os.Stat(runtimePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Python %s is not installed", version)
	}

	info, err := os.Stat(runtimePath)
	if err != nil {
		return nil, err
	}

	return &Runtime{
		Version:    version,
		Path:       runtimePath,
		OS:         m.platform.OS,
		Arch:       m.platform.Arch,
		Downloaded: info.ModTime(),
	}, nil
}

// Remove removes a Python runtime
func (m *Manager) Remove(version string) error {
	runtimePath := filepath.Join(m.homeDir, "runtimes", fmt.Sprintf("python-%s", version))

	if _, err := os.Stat(runtimePath); os.IsNotExist(err) {
		return fmt.Errorf("Python %s is not installed", version)
	}

	if err := os.RemoveAll(runtimePath); err != nil {
		return fmt.Errorf("failed to remove runtime: %w", err)
	}

	fmt.Printf(" Python %s removed\n", version)
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
