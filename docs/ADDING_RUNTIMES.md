# Adding New Runtimes to OPHID

This guide explains how to add support for new runtime types (interpreters) to OPHID. 
Currently only Python and Node.js are implemented, with Bun and Deno as planned additions.

## Architecture Overview

Runtime support in OPHID consists of:

1. **Runtime Type Definition** (internal/runtime/runtime_type.go)
2. **Download Logic** (internal/runtime/downloader.go)
3. **Installation Logic** (internal/runtime/manager.go)
4. **Verification** (internal/runtime/verifier.go - optional)
5. **Extraction** (internal/runtime/extractor.go - reused)

## Step-by-Step Implementation

### Step 1: Define Runtime Type

Edit `internal/runtime/runtime_type.go`:

```go
const (
    RuntimePython RuntimeType = "python"
    RuntimeNode RuntimeType = "node"
    RuntimeBun RuntimeType = "bun"         // Add new constant
    RuntimeDeno RuntimeType = "deno"       // Add new constant
    RuntimeYourRuntime RuntimeType = "yourruntime"  // Your runtime
)
```

Mark as implemented in `IsImplemented()` method:

```go
func (rt RuntimeType) IsImplemented() bool {
    switch rt {
    case RuntimePython, RuntimeNode, RuntimeYourRuntime:  // Add here
        return true
    case RuntimeBun, RuntimeDeno:
        return false
    default:
        return false
    }
}
```

Add display name in `DisplayName()` method:

```go
func (rt RuntimeType) DisplayName() string {
    switch rt {
    case RuntimePython:
        return "Python"
    case RuntimeNode:
        return "Node.js"
    case RuntimeYourRuntime:
        return "Your Runtime"  // Add here
    default:
        return string(rt)
    }
}
```

### Step 2: Add Download Function

Edit `internal/runtime/downloader.go`:

1. Add download URL constant:

```go
const (
    pythonBuildStandaloneURL = "https://github.com/indygreg/python-build-standalone/releases/download"
    nodejsDistURL = "https://nodejs.org/dist"
    yourRuntimeURL = "https://your-runtime-dist-url.com"  // Add your URL
)
```

2. Add download method:

```go
// DownloadYourRuntime downloads your runtime from official distribution
func (d *Downloader) DownloadYourRuntime(version string, platform Platform) (string, error) {
    // Check platform support
    if !platform.IsSupported() {
        return "", fmt.Errorf("unsupported platform: %s", platform)
    }

    // Build download URL
    url := d.buildYourRuntimeURL(version, platform)

    // Create cache directory
    if err := os.MkdirAll(d.cacheDir, 0755); err != nil {
        return "", fmt.Errorf("failed to create cache dir: %w", err)
    }

    // Determine output path
    filename := filepath.Base(url)
    outputPath := filepath.Join(d.cacheDir, filename)

    // Check if already cached
    if _, err := os.Stat(outputPath); err == nil {
        slog.Info("using cached download", "filename", filename)
        return outputPath, nil
    }

    // Download with progress bar
    slog.Info("downloading runtime", "version", version, "platform", platform.String())

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
    bar := progressbar.DefaultBytes(resp.ContentLength, "downloading")

    // Copy with progress
    _, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
    if err != nil {
        os.Remove(outputPath)
        return "", fmt.Errorf("download failed: %w", err)
    }

    fmt.Println()
    return outputPath, nil
}

// buildYourRuntimeURL builds the download URL for your runtime
func (d *Downloader) buildYourRuntimeURL(version string, platform Platform) string {
    // Example URL patterns:
    // - https://your-runtime.com/releases/v1.0.0/runtime-linux-x64.tar.gz
    // - https://your-runtime.com/dist/v1.0.0/runtime-darwin-arm64.tar.gz

    var os, arch string

    switch platform.OS {
    case "darwin":
        os = "darwin"
    case "linux":
        os = "linux"
    case "windows":
        os = "windows"
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

    // Adjust extension based on platform
    ext := "tar.gz"
    if platform.OS == "windows" {
        ext = "zip"
    }

    filename := fmt.Sprintf("yourruntime-v%s-%s-%s.%s", version, os, arch, ext)
    return fmt.Sprintf("%s/v%s/%s", yourRuntimeURL, version, filename)
}
```

### Step 3: Add Installation Method

Edit `internal/runtime/manager.go`:

1. Add case to switch statement in `InstallFromSpec()`:

```go
// Download runtime based on type
switch spec.Type {
case RuntimePython:
    return m.installPython(spec, runtimePath)
case RuntimeNode:
    return m.installNodeJS(spec, runtimePath)
case RuntimeYourRuntime:
    return m.installYourRuntime(spec, runtimePath)  // Add here
default:
    return nil, fmt.Errorf("runtime type %s is not yet implemented", spec.Type.DisplayName())
}
```

2. Add installation method:

```go
// installYourRuntime installs your runtime from official distributions
func (m *Manager) installYourRuntime(spec *RuntimeSpec, runtimePath string) (*Runtime, error) {
    // Download runtime
    tarballPath, err := m.downloader.DownloadYourRuntime(spec.Version, m.platform)
    if err != nil {
        return nil, fmt.Errorf("download failed: %w", err)
    }

    // Verify file exists
    slog.Info("verifying download integrity", "file", tarballPath)
    if err := m.verifier.VerifyFileExists(tarballPath); err != nil {
        return nil, fmt.Errorf("verification failed: %w", err)
    }

    // Optional: Add SHA256 verification if available
    // See installPython() for example of hash verification

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
```

### Step 4: Test Your Runtime

Build and test:

```bash
# Build ophid
go build ./cmd/ophid

# Test installation
./ophid runtime install yourruntime@1.0.0

# List runtimes
./ophid runtime list

# Verify installation
ls ~/.ophid/runtimes/yourruntime-1.0.0
```

## Important Considerations

### Platform Detection

OPHID uses `internal/runtime/platform.go` for platform detection. Ensure your runtime supports the detected platforms:

```go
type Platform struct {
    OS   string  // darwin, linux, windows
    Arch string  // x86_64, aarch64, arm64
}
```

### Archive Extraction

The `Extractor` (internal/runtime/extractor.go) handles:
- tar.gz files (most common)
- zip files (Windows)

If your runtime uses a different archive format, you may need to extend the extractor.

### Directory Structure

Downloaded runtimes are stored in:
```
~/.ophid/runtimes/{runtime}-{version}/
```

For example:
- python-3.12.1/
- node-20.0.0/
- yourruntime-1.0.0/

### SHA256 Verification (Optional)

For security, implement SHA256 verification similar to Python:

1. Add method to `internal/runtime/verifier.go`
2. Fetch checksums from official source
3. Verify before extraction

Don't always trust sites and URLs, and if you are using internal repositories or storage make sure to create the proper policies.

See `GetSHA256ForVersion()` in verifier.go for reference implementation.

## Example: Node.js Implementation

Node.js is a good reference for adding new runtimes:

**URL Pattern:**
```
https://nodejs.org/dist/v20.0.0/node-v20.0.0-darwin-x64.tar.gz
https://nodejs.org/dist/v20.0.0/node-v20.0.0-linux-x64.tar.gz
```

**Implementation:**
- internal/runtime/runtime_type.go: RuntimeNode constant
- internal/runtime/downloader.go: DownloadNodeJS() and buildNodeJSURL()
- internal/runtime/manager.go: installNodeJS()

**Usage:**
```bash
ophid runtime install node@20.0.0
ophid runtime install node@18.19.0
ophid runtime list
```

## Production Readiness Checklist for New Runtimes 

Use this list as a guideline or tasklist in case of a coding agent

- [ ] Add runtime type constant to runtime_type.go
- [ ] Mark as implemented in IsImplemented()
- [ ] Add DisplayName() case
- [ ] Add download URL constant in downloader.go
- [ ] Implement Download{Runtime}() method
- [ ] Implement build{Runtime}URL() method
- [ ] Add case in InstallFromSpec() switch
- [ ] Implement install{Runtime}() method
- [ ] Test on multiple platforms (Linux, macOS, Windows if applicable)
- [ ] Update documentation
- [ ] Add example to README.md

## Troubleshooting

### Download Failures

Check:
- URL format is correct
- Version number formatting (v prefix, no v, etc.)
- Platform/architecture naming conventions
- Network connectivity

### Extraction Failures

Check:
- Archive format (tar.gz, zip)
- Archive structure (does it contain a root directory?)
- File permissions after extraction

### Platform Issues

Ensure your runtime provides official binaries for:
- Linux (x86_64, aarch64)
- macOS (x86_64, arm64)
- Windows (x86_64, aarch64)


## Resources

**Existing Implementations:**
- Python: internal/runtime/manager.go:installPython()
- Node.js: internal/runtime/manager.go:installNodeJS()

**Key Files:**
- internal/runtime/runtime_type.go: Type definitions
- internal/runtime/downloader.go: Download logic
- internal/runtime/manager.go: Installation orchestration
- internal/runtime/platform.go: Platform detection
- internal/runtime/extractor.go: Archive extraction

**Testing:**
- Build: `go build ./cmd/ophid`
- Test: `./ophid runtime install <runtime>@<version>`
- Verify: `ls ~/.ophid/runtimes/`
