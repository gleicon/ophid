package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gleicon/ophid/internal/runtime"
	"github.com/gleicon/ophid/internal/security"
	"github.com/gleicon/ophid/internal/supervisor"
	"github.com/gleicon/ophid/internal/tool"
	"github.com/gleicon/ophid/internal/proxy"
)

var (
	version = "0.1.0-dev"
	homeDir string
)

func main() {
	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get home directory: %v\n", err)
		os.Exit(1)
	}
	homeDir = filepath.Join(home, ".ophid")

	rootCmd := &cobra.Command{
		Use:   "ophid",
		Short: "Operations Python Hybrid Distribution",
		Long: `OPHID is a Go-powered runtime manager for Python operations tools.
It makes Python-based infrastructure tools trivial to install and run,
with zero Python knowledge required.`,
		Version: version,
	}

	rootCmd.AddCommand(runtimeCmd())
	rootCmd.AddCommand(installCmd())
	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(upgradeCmd())
	rootCmd.AddCommand(uninstallCmd())
	rootCmd.AddCommand(searchCmd())
	rootCmd.AddCommand(infoCmd())
	rootCmd.AddCommand(cacheCmd())
	rootCmd.AddCommand(doctorCmd())
	rootCmd.AddCommand(scanCmd())
	rootCmd.AddCommand(proxyCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runtimeCmd manages Python runtimes
func runtimeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Manage Python runtimes",
		Long:  "Download, list, and manage Python runtime installations",
	}

	cmd.AddCommand(runtimeInstallCmd())
	cmd.AddCommand(runtimeListCmd())
	cmd.AddCommand(runtimeRemoveCmd())

	return cmd
}

func runtimeInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install <version>",
		Short: "Install a Python runtime",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version := args[0]

			mgr := runtime.NewManager(homeDir)
			rt, err := mgr.Install(version)
			if err != nil {
				return err
			}

			fmt.Printf("\nPython %s installed:\n", version)
			fmt.Printf("  Path: %s\n", rt.Path)
			fmt.Printf("  Platform: %s/%s\n", rt.OS, rt.Arch)
			return nil
		},
	}
}

func runtimeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed Python runtimes",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := runtime.NewManager(homeDir)
			runtimes, err := mgr.List()
			if err != nil {
				return err
			}

			if len(runtimes) == 0 {
				fmt.Println("No Python runtimes installed")
				return nil
			}

			fmt.Println("Installed Python runtimes:")
			for _, rt := range runtimes {
				fmt.Printf("  python-%s (%s/%s)\n", rt.Version, rt.OS, rt.Arch)
			}

			return nil
		},
	}
}

func runtimeRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <version>",
		Short: "Remove a Python runtime",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version := args[0]

			mgr := runtime.NewManager(homeDir)
			return mgr.Remove(version)
		},
	}
}

func installCmd() *cobra.Command {
	var version string
	var force bool

	cmd := &cobra.Command{
		Use:   "install <tool>",
		Short: "Install a tool",
		Long: `Install a Python operations tool.

Examples:
  ophid install ansible           # Install latest version
  ophid install ansible --version 2.10.0  # Install specific version
  ophid install ansible --force   # Force reinstall`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolName := args[0]

			// Get Python runtime
			runtimeMgr := runtime.NewManager(homeDir)
			pythonRuntime, err := runtimeMgr.Get("3.12.1")
			if err != nil {
				// Try to find any installed runtime
				runtimes, listErr := runtimeMgr.List()
				if listErr != nil || len(runtimes) == 0 {
					return fmt.Errorf("no Python runtime installed. Run: ophid runtime install 3.12.1")
				}
				pythonRuntime = runtimes[0]
			}

			pythonPath := filepath.Join(pythonRuntime.Path, "bin", "python3")

			// Create venv manager
			venvMgr := tool.NewVenvManager(homeDir, pythonPath)

			// Create installer
			installer, err := tool.NewInstaller(homeDir, venvMgr)
			if err != nil {
				return fmt.Errorf("failed to create installer: %w", err)
			}

			// Install tool
			opts := tool.InstallOptions{
				Version: version,
				Force:   force,
			}

			if _, err := installer.Install(toolName, opts); err != nil {
				return fmt.Errorf("installation failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&version, "version", "latest", "Tool version to install")
	cmd.Flags().BoolVar(&force, "force", false, "Force reinstall")

	return cmd
}

func runCmd() *cobra.Command {
	var background bool
	var autoRestart bool

	cmd := &cobra.Command{
		Use:   "run <tool> [args...]",
		Short: "Run a tool explicitly",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmdObj *cobra.Command, args []string) error {
			toolName := args[0]
			toolArgs := args[1:]

			// Get Python runtime
			runtimeMgr := runtime.NewManager(homeDir)
			runtimes, err := runtimeMgr.List()
			if err != nil || len(runtimes) == 0 {
				return fmt.Errorf("no Python runtime installed")
			}

			pythonPath := filepath.Join(runtimes[0].Path, "bin", "python3")
			venvMgr := tool.NewVenvManager(homeDir, pythonPath)

			// Create installer to get tool info
			installer, err := tool.NewInstaller(homeDir, venvMgr)
			if err != nil {
				return fmt.Errorf("failed to create installer: %w", err)
			}

			// Get tool
			t, err := installer.Get(toolName)
			if err != nil {
				return fmt.Errorf("tool %s not installed. Run: ophid install %s", toolName, toolName)
			}

			// Find executable in venv
			binDir := venvMgr.GetBinDir(t.InstallPath)
			executable := filepath.Join(binDir, toolName)

			if background {
				// Run as supervised process
				mgr := supervisor.NewManager()

				config := supervisor.ProcessConfig{
					Name:        toolName,
					Command:     executable,
					Args:        toolArgs,
					AutoRestart: autoRestart,
					MaxRetries:  3,
				}

				ctx := context.Background()
				if err := mgr.Start(ctx, config); err != nil {
					return fmt.Errorf("failed to start process: %w", err)
				}

				fmt.Printf("Started %s in background (PID: %d)\n", toolName, mgr.List()[toolName].Cmd.Process.Pid)
				return nil
			}

			// Run directly
			runCmd := exec.Command(executable, toolArgs...)
			runCmd.Stdout = os.Stdout
			runCmd.Stderr = os.Stderr
			runCmd.Stdin = os.Stdin

			return runCmd.Run()
		},
	}

	cmd.Flags().BoolVarP(&background, "background", "b", false, "Run in background")
	cmd.Flags().BoolVar(&autoRestart, "auto-restart", false, "Auto-restart on failure (requires --background)")

	return cmd
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get Python runtime (just for venv manager setup)
			runtimeMgr := runtime.NewManager(homeDir)
			runtimes, err := runtimeMgr.List()
			if err != nil || len(runtimes) == 0 {
				fmt.Println("No Python runtime installed")
				return nil
			}

			pythonPath := filepath.Join(runtimes[0].Path, "bin", "python3")
			venvMgr := tool.NewVenvManager(homeDir, pythonPath)

			// Create installer
			installer, err := tool.NewInstaller(homeDir, venvMgr)
			if err != nil {
				return fmt.Errorf("failed to create installer: %w", err)
			}

			// List tools
			tools := installer.List()
			if len(tools) == 0 {
				fmt.Println("No tools installed")
				return nil
			}

			fmt.Println("Installed tools:")
			for _, t := range tools {
				fmt.Printf("  %s@%s\n", t.Name, t.Version)
				if len(t.Executables) > 0 {
					fmt.Printf("    Executables: %s\n", strings.Join(t.Executables, ", "))
				}
			}

			return nil
		},
	}
}

func upgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade <tool>",
		Short: "Upgrade a tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement
			fmt.Printf("Upgrading %s...\n", args[0])
			fmt.Println("⚠️  Not yet implemented - coming soon!")
			return nil
		},
	}
}

func uninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <tool>",
		Short: "Uninstall a tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolName := args[0]

			// Get Python runtime
			runtimeMgr := runtime.NewManager(homeDir)
			runtimes, err := runtimeMgr.List()
			if err != nil || len(runtimes) == 0 {
				return fmt.Errorf("no Python runtime installed")
			}

			pythonPath := filepath.Join(runtimes[0].Path, "bin", "python3")
			venvMgr := tool.NewVenvManager(homeDir, pythonPath)

			// Create installer
			installer, err := tool.NewInstaller(homeDir, venvMgr)
			if err != nil {
				return fmt.Errorf("failed to create installer: %w", err)
			}

			// Uninstall tool
			if err := installer.Uninstall(toolName); err != nil {
				return fmt.Errorf("uninstall failed: %w", err)
			}

			return nil
		},
	}
}

func searchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search for tools",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement
			fmt.Printf("Searching for '%s'...\n", args[0])
			fmt.Println("⚠️  Not yet implemented - coming soon!")
			return nil
		},
	}
}

func infoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <tool>",
		Short: "Show tool information",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement
			fmt.Printf("Tool: %s\n", args[0])
			fmt.Println("⚠️  Not yet implemented - coming soon!")
			return nil
		},
	}
}

func cacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage package cache",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "clean",
		Short: "Clean package cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement
			fmt.Println("Cleaning cache...")
			fmt.Println("⚠️  Not yet implemented - coming soon!")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "stats",
		Short: "Show cache statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement
			fmt.Println("Cache statistics:")
			fmt.Println("⚠️  Not yet implemented - coming soon!")
			return nil
		},
	})

	return cmd
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose OPHID issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement
			fmt.Println("Running diagnostics...")
			fmt.Println("⚠️  Not yet implemented - coming soon!")
			return nil
		},
	}
}

func scanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Security and supply chain scanning",
	}

	cmd.AddCommand(scanVulnCmd())
	cmd.AddCommand(scanLicenseCmd())
	cmd.AddCommand(scanSBOMCmd())

	return cmd
}

func scanVulnCmd() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "vuln [requirements.txt|go.mod]",
		Short: "Scan for vulnerabilities",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]

			// Parse dependency file
			packages, err := parseDependencyFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to parse %s: %w", filePath, err)
			}

			if len(packages) == 0 {
				fmt.Println("No packages found in file")
				return nil
			}

			fmt.Printf("Scanning %d packages for vulnerabilities...\n", len(packages))

			// Create scanner
			scanner := security.NewScanner()
			ctx := context.Background()

			// Scan packages
			results, err := scanner.ScanPackages(ctx, packages)
			if err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}

			// Display results
			return displayVulnResults(results, outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "format", "f", "text", "Output format (text|json)")
	return cmd
}

func scanLicenseCmd() *cobra.Command {
	var allowCopyleft bool

	cmd := &cobra.Command{
		Use:   "license [requirements.txt|go.mod]",
		Short: "Check package licenses",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]

			// Parse dependency file
			packages, err := parseDependencyFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to parse %s: %w", filePath, err)
			}

			if len(packages) == 0 {
				fmt.Println("No packages found in file")
				return nil
			}

			fmt.Printf("Checking licenses for %d packages...\n\n", len(packages))

			// Create license checker
			allowedTypes := []security.LicenseType{security.LicensePermissive}
			if allowCopyleft {
				allowedTypes = append(allowedTypes, security.LicenseCopyleft)
			}
			checker := security.NewLicenseChecker(allowedTypes)

			// Display results
			return displayLicenseResults(packages, checker)
		},
	}

	cmd.Flags().BoolVar(&allowCopyleft, "allow-copyleft", false, "Allow copyleft licenses")
	return cmd
}

func scanSBOMCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "sbom [requirements.txt|go.mod]",
		Short: "Generate SBOM (Software Bill of Materials)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]

			// Parse dependency file
			packages, err := parseDependencyFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to parse %s: %w", filePath, err)
			}

			if len(packages) == 0 {
				fmt.Println("No packages found in file")
				return nil
			}

			fmt.Printf("Generating SBOM for %d packages...\n", len(packages))

			// Generate SBOM
			sbom, err := security.GenerateSBOM(packages, "ophid")
			if err != nil {
				return fmt.Errorf("failed to generate SBOM: %w", err)
			}

			// Determine output path
			if outputPath == "" {
				outputPath = "sbom.json"
			}

			// Write SBOM
			if err := security.WriteSBOM(sbom, outputPath); err != nil {
				return fmt.Errorf("failed to write SBOM: %w", err)
			}

			fmt.Printf("SBOM written to %s\n", outputPath)
			fmt.Printf("  Format: CycloneDX 1.4\n")
			fmt.Printf("  Components: %d\n", len(sbom.Components))

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (default: sbom.json)")
	return cmd
}

// Helper functions

func parseDependencyFile(filePath string) ([]security.Package, error) {
	if strings.HasSuffix(filePath, "requirements.txt") {
		return security.ParseRequirementsTxt(filePath)
	} else if strings.HasSuffix(filePath, "go.mod") {
		return security.ParseGoMod(filePath)
	} else if strings.HasSuffix(filePath, "package.json") {
		return security.ParsePackageJSON(filePath)
	}
	return nil, fmt.Errorf("unsupported file type: %s (supported: requirements.txt, go.mod, package.json)", filePath)
}

func displayVulnResults(results []security.ScanResult, format string) error {
	if format == "json" {
		// TODO: Implement JSON output
		return fmt.Errorf("JSON output not yet implemented")
	}

	totalVulns := 0
	criticalCount := 0

	for _, result := range results {
		if result.Error != "" {
			fmt.Printf("✗ %s@%s: %s\n", result.Package.Name, result.Package.Version, result.Error)
			continue
		}

		if len(result.Vulnerabilities) == 0 {
			fmt.Printf("✓ %s@%s: No vulnerabilities found\n", result.Package.Name, result.Package.Version)
			continue
		}

		totalVulns += len(result.Vulnerabilities)
		critical := result.CriticalCount()
		criticalCount += critical

		fmt.Printf("⚠ %s@%s: %d vulnerabilities found", result.Package.Name, result.Package.Version, len(result.Vulnerabilities))
		if critical > 0 {
			fmt.Printf(" (%d critical)", critical)
		}
		fmt.Println()

		for _, vuln := range result.Vulnerabilities {
			fmt.Printf("  - %s: %s\n", vuln.ID, vuln.Summary)
			if len(vuln.Severity) > 0 {
				fmt.Printf("    Severity: %s %s\n", vuln.Severity[0].Type, vuln.Severity[0].Score)
			}
		}
	}

	fmt.Println()
	fmt.Printf("Summary: %d vulnerabilities found", totalVulns)
	if criticalCount > 0 {
		fmt.Printf(" (%d critical)", criticalCount)
	}
	fmt.Println()

	if totalVulns > 0 {
		return fmt.Errorf("vulnerabilities detected")
	}

	return nil
}

func displayLicenseResults(packages []security.Package, checker *security.LicenseChecker) error {
	unknownCount := 0
	incompatibleCount := 0

	for _, pkg := range packages {
		// Note: This is simplified - in production, we'd fetch actual licenses from registries
		// For now, we'll just check if common licenses are in the package name or use placeholder
		license := "Unknown"

		info, allowed := checker.CheckLicense(license)

		if info.Type == security.LicenseUnknown {
			fmt.Printf("? %s@%s: Unknown license\n", pkg.Name, pkg.Version)
			unknownCount++
		} else if !allowed {
			fmt.Printf("✗ %s@%s: %s (not allowed)\n", pkg.Name, pkg.Version, info.Name)
			incompatibleCount++
		} else {
			fmt.Printf("✓ %s@%s: %s\n", pkg.Name, pkg.Version, info.Name)
		}
	}

	fmt.Println()
	fmt.Printf("Summary: %d packages checked\n", len(packages))
	fmt.Printf("  Unknown licenses: %d\n", unknownCount)
	fmt.Printf("  Incompatible licenses: %d\n", incompatibleCount)

	if incompatibleCount > 0 {
		return fmt.Errorf("incompatible licenses detected")
	}

	return nil
}

func proxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Reverse proxy management",
		Long:  "Start and manage the HTTP/HTTPS reverse proxy server",
	}

	cmd.AddCommand(proxyStartCmd())
	cmd.AddCommand(proxyStatusCmd())
	cmd.AddCommand(proxyStopCmd())
	cmd.AddCommand(proxyRouteCmd())

	return cmd
}

func proxyStartCmd() *cobra.Command {
	var configPath string
	var domain string
	var target string
	var listen string
	var tlsAuto bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the reverse proxy server",
		Long: `Start the reverse proxy server with the given configuration.

Examples:
  # Start with config file
  ophid proxy start --config proxy.toml

  # Quick start with automatic TLS
  ophid proxy start --domain example.com --target localhost:3000 --tls auto

  # Simple HTTP proxy
  ophid proxy start --listen :8080 --target localhost:3000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var config *proxy.Config

			if configPath != "" {
				// TODO: Load config from file
				return fmt.Errorf("config file loading not yet implemented")
			} else if domain != "" && target != "" {
				// Quick setup mode
				config = &proxy.Config{
					General: proxy.GeneralConfig{
						Listen: []string{":80", ":443"},
					},
					TLS: proxy.TLSConfig{
						Enabled:      tlsAuto,
						AutoRedirect: tlsAuto,
						ACMEProvider: "letsencrypt",
						Domains:      []string{domain},
						CacheDir:     filepath.Join(homeDir, "certs"),
					},
					Routes: []proxy.Route{
						{
							Host:   domain,
							Target: target,
						},
					},
				}
			} else if listen != "" && target != "" {
				// Simple HTTP proxy
				config = &proxy.Config{
					General: proxy.GeneralConfig{
						Listen: []string{listen},
					},
					Routes: []proxy.Route{
						{
							Target: target,
						},
					},
				}
			} else {
				return fmt.Errorf("either --config, or --domain and --target, or --listen and --target must be specified")
			}

			// Create and start server
			fmt.Println("Starting reverse proxy server...")
			server, err := proxy.NewServer(config)
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			if err := server.Start(); err != nil {
				return fmt.Errorf("server error: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Path to config file")
	cmd.Flags().StringVar(&domain, "domain", "", "Domain name for quick setup")
	cmd.Flags().StringVar(&target, "target", "", "Target backend URL")
	cmd.Flags().StringVar(&listen, "listen", "", "Listen address (e.g., :8080)")
	cmd.Flags().BoolVar(&tlsAuto, "tls", false, "Enable automatic TLS with Let's Encrypt")

	return cmd
}

func proxyStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show proxy server status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement status check
			fmt.Println("Proxy status:")
			fmt.Println("⚠️  Not yet implemented - coming soon!")
			return nil
		},
	}
}

func proxyStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the proxy server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement graceful shutdown
			fmt.Println("Stopping proxy server...")
			fmt.Println("⚠️  Not yet implemented - coming soon!")
			return nil
		},
	}
}

func proxyRouteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Manage proxy routes",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all routes",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement route listing
			fmt.Println("Routes:")
			fmt.Println("⚠️  Not yet implemented - coming soon!")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "add",
		Short: "Add a new route",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement route addition
			fmt.Println("Adding route...")
			fmt.Println("⚠️  Not yet implemented - coming soon!")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "remove <host>",
		Short: "Remove a route",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement route removal
			fmt.Printf("Removing route for %s...\n", args[0])
			fmt.Println("⚠️  Not yet implemented - coming soon!")
			return nil
		},
	})

	return cmd
}
