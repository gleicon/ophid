package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gleicon/ophid/internal/security"
)

// Installer handles tool installation
type Installer struct {
	homeDir       string
	venvManager   *VenvManager
	manifest      *ToolManifest
	manifestPath  string
	sourceDetector *SourceDetector
	gitInstaller  *GitInstaller
	localInstaller *LocalInstaller
	scanner       *security.Scanner
}

// NewInstaller creates a new tool installer
func NewInstaller(homeDir string, venvManager *VenvManager) (*Installer, error) {
	manifestPath := filepath.Join(homeDir, "tools", "manifest.json")
	scanner := security.NewScanner()

	installer := &Installer{
		homeDir:       homeDir,
		venvManager:   venvManager,
		manifestPath:  manifestPath,
		sourceDetector: NewSourceDetector(),
		gitInstaller:  NewGitInstaller(homeDir, scanner),
		localInstaller: NewLocalInstaller(homeDir, scanner),
		scanner:       scanner,
	}

	// Load existing manifest
	if err := installer.loadManifest(); err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	return installer, nil
}

// Install installs a tool from any supported source
func (i *Installer) Install(name string, opts InstallOptions) (*Tool, error) {
	ctx := context.Background()

	fmt.Printf("Installing %s...\n", name)

	// Detect installation source
	source, err := i.sourceDetector.DetectSource(name, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to detect source: %w", err)
	}

	fmt.Printf("Installation source: %s\n", source.Type)

	// Route to appropriate installer
	switch source.Type {
	case SourcePyPI:
		return i.installFromPyPI(ctx, name, source, opts)
	case SourceGitHub, SourceGit:
		return i.installFromGit(ctx, name, source, opts)
	case SourceLocal:
		return i.installFromLocal(ctx, name, source, opts)
	default:
		return nil, fmt.Errorf("unsupported source type: %s", source.Type)
	}
}

// installFromPyPI installs a package from PyPI
func (i *Installer) installFromPyPI(ctx context.Context, name string, source InstallSource, opts InstallOptions) (*Tool, error) {
	fmt.Printf("Installing %s from PyPI...\n", name)

	// PHASE 1: PRE-FLIGHT SECURITY SCAN (BEFORE creating venv or installing)
	var secInfo SecurityInfo
	if !opts.SkipScan {
		// Get version for scanning
		version := opts.Version
		if version == "" || version == "latest" {
			var err error
			version, err = i.getLatestPyPIVersion(ctx, name)
			if err != nil {
				fmt.Printf("⚠ Warning: failed to get version from PyPI: %v\n", err)
				version = "latest"
			} else {
				fmt.Printf("Latest version: %s\n", version)
			}
		}

		// Scan for vulnerabilities BEFORE installing
		fmt.Println("\n🔒 Running pre-installation security scan...")
		secInfo = i.scanPyPIPackage(ctx, name, version)

		// Check if we should block installation
		if opts.RequireScan && secInfo.CriticalVulnCount > 0 {
			return nil, fmt.Errorf("critical vulnerabilities found (%d) - installation blocked\nRun 'ophid scan vuln %s' for details",
				secInfo.CriticalVulnCount, name)
		}

		if secInfo.VulnCount > 0 {
			fmt.Printf("⚠ Warning: %d vulnerabilities found (%d critical)\n",
				secInfo.VulnCount, secInfo.CriticalVulnCount)
			if !opts.RequireScan {
				fmt.Println("Proceeding with installation (use --require-scan to block)")
			}
		} else {
			fmt.Println("✓ No vulnerabilities found")
		}
	}

	// PHASE 2: CREATE VENV (only if pre-flight passed)
	venvPath, err := i.venvManager.Create(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create venv: %w", err)
	}

	// Get pip path
	pipPath := i.venvManager.GetPipPath(venvPath)

	// Build pip install command
	args := []string{"install"}

	if opts.Force {
		args = append(args, "--force-reinstall")
	}

	if opts.NoDeps {
		args = append(args, "--no-deps")
	}

	if opts.Editable {
		args = append(args, "-e")
	}

	// Add package specification
	pkgSpec := name
	if opts.Version != "" && opts.Version != "latest" {
		pkgSpec = fmt.Sprintf("%s==%s", name, opts.Version)
	}

	if len(opts.Extras) > 0 {
		pkgSpec = fmt.Sprintf("%s[%s]", pkgSpec, strings.Join(opts.Extras, ","))
	}

	args = append(args, pkgSpec)

	// Run pip install
	fmt.Printf("Running: %s %s\n", pipPath, strings.Join(args, " "))
	cmd := exec.Command(pipPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pip install failed: %w", err)
	}

	// Get installed version
	installedVersion, err := i.getInstalledVersion(pipPath, name)
	if err != nil {
		installedVersion = opts.Version
		if installedVersion == "" || installedVersion == "latest" {
			installedVersion = "unknown"
		}
	}

	// List executables
	executables, err := i.venvManager.ListExecutables(venvPath)
	if err != nil {
		executables = []string{}
	}

	// Note: Security scan already performed in pre-flight phase

	// Create tool record
	tool := &Tool{
		Name:        name,
		Version:     installedVersion,
		Ecosystem:   "python",
		Runtime:     "python3",
		InstallPath: venvPath,
		Executables: executables,
		Source:      source,
		Security:    secInfo,
		InstalledAt: time.Now(),
	}

	// Add to manifest
	i.manifest.Tools[name] = tool
	i.manifest.UpdatedAt = time.Now()

	// Save manifest
	if err := i.saveManifest(); err != nil {
		return nil, fmt.Errorf("failed to save manifest: %w", err)
	}

	fmt.Printf("\n✓ %s@%s installed successfully\n", name, installedVersion)
	if len(executables) > 0 {
		fmt.Printf("  Executables: %s\n", strings.Join(executables, ", "))
	}
	if secInfo.VulnCount > 0 {
		fmt.Printf("  ⚠ Vulnerabilities: %d total, %d critical\n", secInfo.VulnCount, secInfo.CriticalVulnCount)
	}

	return tool, nil
}

// installFromGit installs a package from a Git repository
func (i *Installer) installFromGit(ctx context.Context, name string, source InstallSource, opts InstallOptions) (*Tool, error) {
	// Clone repository
	repoPath, err := i.gitInstaller.CloneRepository(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	fmt.Printf("Repository cloned to: %s\n", repoPath)

	// Detect ecosystem
	ecosystem := i.gitInstaller.DetectEcosystem(repoPath)
	fmt.Printf("Detected ecosystem: %s\n", ecosystem)

	if ecosystem == "unknown" {
		return nil, fmt.Errorf("could not detect project type in repository")
	}

	// Security scan (if not skipped)
	var secInfo *SecurityInfo
	if !opts.SkipScan {
		fmt.Println("\n🔒 Running security scan...")
		secInfo, err = i.gitInstaller.ScanRepository(ctx, repoPath)
		if err != nil {
			if opts.RequireScan {
				return nil, fmt.Errorf("security scan failed: %w", err)
			}
			fmt.Printf("⚠ Warning: security scan failed: %v\n", err)
			secInfo = &SecurityInfo{}
		}

		// Check if critical vulnerabilities found
		if opts.RequireScan && secInfo.CriticalVulnCount > 0 {
			return nil, fmt.Errorf("critical vulnerabilities found (%d) - installation blocked", secInfo.CriticalVulnCount)
		}
	} else {
		secInfo = &SecurityInfo{}
	}

	// Install based on ecosystem
	var venvPath string
	var executables []string

	if ecosystem == "python" {
		// Create venv
		venvPath, err = i.venvManager.Create(name)
		if err != nil {
			return nil, fmt.Errorf("failed to create venv: %w", err)
		}

		// Install from local path
		pipPath := i.venvManager.GetPipPath(venvPath)
		installCmd := exec.CommandContext(ctx, pipPath, "install", "-e", repoPath)
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr

		if err := installCmd.Run(); err != nil {
			return nil, fmt.Errorf("pip install failed: %w", err)
		}

		// List executables
		executables, _ = i.venvManager.ListExecutables(venvPath)
	} else {
		venvPath = repoPath
	}

	// Get version
	version, err := i.gitInstaller.GetVersion(ctx, repoPath)
	if err != nil {
		version = "dev"
	}

	// Create tool record
	tool := &Tool{
		Name:        name,
		Version:     version,
		Ecosystem:   ecosystem,
		Runtime:     ecosystem,
		InstallPath: venvPath,
		Executables: executables,
		Source:      source,
		Security:    *secInfo,
		InstalledAt: time.Now(),
	}

	// Add to manifest
	i.manifest.Tools[name] = tool
	i.manifest.UpdatedAt = time.Now()

	// Save manifest
	if err := i.saveManifest(); err != nil {
		return nil, fmt.Errorf("failed to save manifest: %w", err)
	}

	fmt.Printf("\n✓ %s@%s installed successfully from Git\n", name, version)
	if len(executables) > 0 {
		fmt.Printf("  Executables: %s\n", strings.Join(executables, ", "))
	}
	if secInfo.VulnCount > 0 {
		fmt.Printf("  ⚠ Vulnerabilities: %d total, %d critical\n", secInfo.VulnCount, secInfo.CriticalVulnCount)
	}

	return tool, nil
}

// installFromLocal installs a package from a local directory
func (i *Installer) installFromLocal(ctx context.Context, name string, source InstallSource, opts InstallOptions) (*Tool, error) {
	// Validate local path
	if err := i.localInstaller.ValidateLocalPath(source.Path); err != nil {
		return nil, fmt.Errorf("invalid local path: %w", err)
	}

	fmt.Printf("Installing from local path: %s\n", source.Path)

	// Detect ecosystem
	ecosystem := i.localInstaller.DetectEcosystem(source.Path)
	fmt.Printf("Detected ecosystem: %s\n", ecosystem)

	if ecosystem == "unknown" {
		return nil, fmt.Errorf("could not detect project type in directory")
	}

	// Security scan (if not skipped)
	var secInfo *SecurityInfo
	if !opts.SkipScan {
		fmt.Println("\n🔒 Running security scan...")
		secInfo, err := i.localInstaller.ScanLocalPath(ctx, source.Path)
		if err != nil {
			if opts.RequireScan {
				return nil, fmt.Errorf("security scan failed: %w", err)
			}
			fmt.Printf("⚠ Warning: security scan failed: %v\n", err)
			secInfo = &SecurityInfo{}
		}

		// Check if critical vulnerabilities found
		if opts.RequireScan && secInfo.CriticalVulnCount > 0 {
			return nil, fmt.Errorf("critical vulnerabilities found (%d) - installation blocked", secInfo.CriticalVulnCount)
		}
	} else {
		secInfo = &SecurityInfo{}
	}

	// Install based on ecosystem
	var venvPath string
	var executables []string

	if ecosystem == "python" {
		// Create venv
		venvPath, err := i.venvManager.Create(name)
		if err != nil {
			return nil, fmt.Errorf("failed to create venv: %w", err)
		}

		// Install from local path (editable mode)
		pipPath := i.venvManager.GetPipPath(venvPath)
		installCmd := exec.CommandContext(ctx, pipPath, "install", "-e", source.Path)
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr

		if err := installCmd.Run(); err != nil {
			return nil, fmt.Errorf("pip install failed: %w", err)
		}

		// List executables
		executables, _ = i.venvManager.ListExecutables(venvPath)
	} else {
		venvPath = source.Path
	}

	// Extract metadata
	metadata := i.localInstaller.ExtractMetadata(source.Path)

	// Create tool record
	tool := &Tool{
		Name:        name,
		Version:     "local",
		Ecosystem:   ecosystem,
		Runtime:     ecosystem,
		InstallPath: venvPath,
		Executables: executables,
		Source:      source,
		Security:    *secInfo,
		Metadata:    metadata,
		InstalledAt: time.Now(),
	}

	// Add to manifest
	i.manifest.Tools[name] = tool
	i.manifest.UpdatedAt = time.Now()

	// Save manifest
	if err := i.saveManifest(); err != nil {
		return nil, fmt.Errorf("failed to save manifest: %w", err)
	}

	fmt.Printf("\n✓ %s installed successfully from local directory\n", name)
	if len(executables) > 0 {
		fmt.Printf("  Executables: %s\n", strings.Join(executables, ", "))
	}
	if secInfo.VulnCount > 0 {
		fmt.Printf("  ⚠ Vulnerabilities: %d total, %d critical\n", secInfo.VulnCount, secInfo.CriticalVulnCount)
	}

	return tool, nil
}

// scanPyPIPackage scans a PyPI package for vulnerabilities
func (i *Installer) scanPyPIPackage(ctx context.Context, name, version string) SecurityInfo {
	secInfo := SecurityInfo{
		LicenseCompliant: true,
	}

	// Create package for scanning
	pkg := security.Package{
		Name:      name,
		Version:   version,
		Ecosystem: "pypi",
	}

	// Scan for vulnerabilities
	results, err := i.scanner.ScanPackages(ctx, []security.Package{pkg})
	if err != nil {
		fmt.Printf("⚠ Warning: vulnerability scan failed: %v\n", err)
		return secInfo
	}

	// Count vulnerabilities
	if len(results) > 0 {
		secInfo.VulnCount = len(results[0].Vulnerabilities)
		secInfo.CriticalVulnCount = results[0].CriticalCount()

		if secInfo.VulnCount > 0 {
			fmt.Printf("⚠ Warning: Found %d vulnerabilities", secInfo.VulnCount)
			if secInfo.CriticalVulnCount > 0 {
				fmt.Printf(" (%d critical)", secInfo.CriticalVulnCount)
			}
			fmt.Println()
		}
	}

	return secInfo
}

// Uninstall removes a tool
func (i *Installer) Uninstall(name string) error {
	tool, exists := i.manifest.Tools[name]
	if !exists {
		return fmt.Errorf("tool %s is not installed", name)
	}

	// Remove venv
	if err := i.venvManager.Remove(name); err != nil {
		return fmt.Errorf("failed to remove venv: %w", err)
	}

	// Remove from manifest
	delete(i.manifest.Tools, name)
	i.manifest.UpdatedAt = time.Now()

	// Save manifest
	if err := i.saveManifest(); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	fmt.Printf("✓ %s@%s uninstalled\n", name, tool.Version)

	return nil
}

// List lists all installed tools
func (i *Installer) List() []*Tool {
	tools := make([]*Tool, 0, len(i.manifest.Tools))
	for _, tool := range i.manifest.Tools {
		tools = append(tools, tool)
	}
	return tools
}

// Get retrieves a specific tool
func (i *Installer) Get(name string) (*Tool, error) {
	tool, exists := i.manifest.Tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s is not installed", name)
	}
	return tool, nil
}

// loadManifest loads the tool manifest
func (i *Installer) loadManifest() error {
	// Create default manifest if file doesn't exist
	if _, err := os.Stat(i.manifestPath); os.IsNotExist(err) {
		i.manifest = &ToolManifest{
			Tools:     make(map[string]*Tool),
			UpdatedAt: time.Now(),
		}
		return nil
	}

	// Read manifest file
	data, err := os.ReadFile(i.manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	// Parse JSON
	var manifest ToolManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	i.manifest = &manifest
	return nil
}

// saveManifest saves the tool manifest
func (i *Installer) saveManifest() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(i.manifestPath), 0755); err != nil {
		return fmt.Errorf("failed to create manifest directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(i.manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Write file
	if err := os.WriteFile(i.manifestPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// getInstalledVersion gets the installed version of a package
func (i *Installer) getInstalledVersion(pipPath, name string) (string, error) {
	cmd := exec.Command(pipPath, "show", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	// Parse output for Version: line
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Version:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "", fmt.Errorf("version not found")
}

// getLatestPyPIVersion queries PyPI JSON API for latest version
func (i *Installer) getLatestPyPIVersion(ctx context.Context, name string) (string, error) {
	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query PyPI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("PyPI returned status %d", resp.StatusCode)
	}

	var result struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse PyPI response: %w", err)
	}

	return result.Info.Version, nil
}
