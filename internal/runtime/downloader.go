package runtime

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/schollz/progressbar/v3"
)

const (
	// pythonBuildStandaloneURL is the base URL for python-build-standalone releases
	pythonBuildStandaloneURL = "https://github.com/indygreg/python-build-standalone/releases/download"

	// pythonBuildDate is the release date to use
	// TODO: Make this configurable or fetch latest
	pythonBuildDate = "20240107"

	// nodejsDistURL is the base URL for official Node.js distributions
	nodejsDistURL = "https://nodejs.org/dist"
)

// Downloader handles downloading Python runtimes
type Downloader struct {
	cacheDir string
	platform Platform
}

// NewDownloader creates a new downloader
func NewDownloader(cacheDir string) *Downloader {
	return &Downloader{
		cacheDir: cacheDir,
		platform: DetectPlatform(),
	}
}

// Download downloads a Python runtime and returns the path to the tarball
func (d *Downloader) Download(version string) (string, error) {
	// Check if platform is supported
	if !d.platform.IsSupported() {
		return "", fmt.Errorf("unsupported platform: %s", d.platform)
	}

	// Build download URL
	url := d.buildURL(version)

	// Create cache directory
	if err := os.MkdirAll(d.cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Determine output path
	filename := filepath.Base(url)
	outputPath := filepath.Join(d.cacheDir, filename)

	// Check if already downloaded
	if _, err := os.Stat(outputPath); err == nil {
		slog.Info("using cached download", "filename", filename)
		return outputPath, nil
	}

	// Download with progress bar
	slog.Info("downloading Python runtime", "version", version, "platform", d.platform.String())

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Create output file
	out, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Create progress bar
	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"downloading",
	)

	// Copy with progress
	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		os.Remove(outputPath) // Clean up partial download
		return "", fmt.Errorf("download failed: %w", err)
	}

	fmt.Println() // New line after progress bar
	return outputPath, nil
}

// buildURL builds the download URL for a specific Python version
func (d *Downloader) buildURL(version string) string {
	// Format: cpython-{version}+{date}-{triple}-install_only.tar.gz
	// Example: cpython-3.12.1+20240107-x86_64-unknown-linux-gnu-install_only.tar.gz

	triple := d.platform.ToPythonBuildStandalone()
	filename := fmt.Sprintf("cpython-%s+%s-%s-install_only.tar.gz",
		version, pythonBuildDate, triple)

	return fmt.Sprintf("%s/%s/%s", pythonBuildStandaloneURL, pythonBuildDate, filename)
}

// GetCachePath returns the expected cache path for a downloaded tarball
func (d *Downloader) GetCachePath(version string) string {
	url := d.buildURL(version)
	filename := filepath.Base(url)
	return filepath.Join(d.cacheDir, filename)
}

// DownloadNodeJS downloads a Node.js runtime and returns the path to the tarball
func (d *Downloader) DownloadNodeJS(version string, platform Platform) (string, error) {
	// Check if platform is supported
	if !platform.IsSupported() {
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}

	// Build download URL
	url := d.buildNodeJSURL(version, platform)

	// Create cache directory
	if err := os.MkdirAll(d.cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Determine output path
	filename := filepath.Base(url)
	outputPath := filepath.Join(d.cacheDir, filename)

	// Check if already downloaded
	if _, err := os.Stat(outputPath); err == nil {
		slog.Info("using cached download", "filename", filename)
		return outputPath, nil
	}

	// Download with progress bar
	slog.Info("downloading Node.js runtime", "version", version, "platform", platform.String())

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Create output file
	out, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Create progress bar
	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"downloading",
	)

	// Copy with progress
	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		os.Remove(outputPath) // Clean up partial download
		return "", fmt.Errorf("download failed: %w", err)
	}

	fmt.Println() // New line after progress bar
	return outputPath, nil
}

// buildNodeJSURL builds the download URL for a specific Node.js version
func (d *Downloader) buildNodeJSURL(version string, platform Platform) string {
	// Format: node-v{version}-{os}-{arch}.tar.gz
	// Example: node-v20.0.0-darwin-x64.tar.gz
	//          node-v20.0.0-linux-x64.tar.gz

	var os, arch string

	switch platform.OS {
	case "darwin":
		os = "darwin"
	case "linux":
		os = "linux"
	case "windows":
		os = "win"
	default:
		os = platform.OS
	}

	switch platform.Arch {
	case "x86_64", "amd64":
		arch = "x64"
	case "aarch64", "arm64":
		arch = "arm64"
	default:
		arch = platform.Arch
	}

	// Windows uses .zip, others use .tar.gz
	var ext string
	if platform.OS == "windows" {
		ext = "zip"
	} else {
		ext = "tar.gz"
	}

	filename := fmt.Sprintf("node-v%s-%s-%s.%s", version, os, arch, ext)
	return fmt.Sprintf("%s/v%s/%s", nodejsDistURL, version, filename)
}
