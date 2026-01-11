package runtime

import (
	"fmt"
	"runtime"
)

// Platform represents the detected operating system and architecture
type Platform struct {
	OS   string // "linux", "darwin", "windows"
	Arch string // "x86_64", "aarch64", "arm64"
}

// DetectPlatform detects the current platform (OS and architecture)
func DetectPlatform() Platform {
	return Platform{
		OS:   runtime.GOOS,
		Arch: normalizeArch(runtime.GOARCH),
	}
}

// normalizeArch converts Go's GOARCH to python-build-standalone naming
func normalizeArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "386":
		return "i686"
	default:
		return goarch
	}
}

// ToPythonBuildStandalone converts platform to python-build-standalone naming
// Examples:
//   - Linux x86_64:  "x86_64-unknown-linux-gnu"
//   - macOS arm64:   "aarch64-apple-darwin"
//   - Windows x86_64: "x86_64-pc-windows-msvc"
func (p Platform) ToPythonBuildStandalone() string {
	switch p.OS {
	case "linux":
		return fmt.Sprintf("%s-unknown-linux-gnu", p.Arch)
	case "darwin":
		return fmt.Sprintf("%s-apple-darwin", p.Arch)
	case "windows":
		return fmt.Sprintf("%s-pc-windows-msvc", p.Arch)
	default:
		return fmt.Sprintf("%s-unknown-%s", p.Arch, p.OS)
	}
}

// String returns a human-readable platform string
func (p Platform) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}

// IsSupported checks if the platform is supported by python-build-standalone
func (p Platform) IsSupported() bool {
	// python-build-standalone supports:
	// - Linux: x86_64, aarch64
	// - macOS: x86_64, aarch64
	// - Windows: x86_64 (limited support)

	switch p.OS {
	case "linux":
		return p.Arch == "x86_64" || p.Arch == "aarch64"
	case "darwin":
		return p.Arch == "x86_64" || p.Arch == "aarch64"
	case "windows":
		return p.Arch == "x86_64"
	default:
		return false
	}
}
