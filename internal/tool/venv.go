package tool

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// VenvManager manages Python virtual environments
type VenvManager struct {
	homeDir     string
	pythonPath  string
}

// NewVenvManager creates a new virtual environment manager
func NewVenvManager(homeDir, pythonPath string) *VenvManager {
	return &VenvManager{
		homeDir:    homeDir,
		pythonPath: pythonPath,
	}
}

// Create creates a new virtual environment for a tool
func (v *VenvManager) Create(toolName string) (string, error) {
	venvPath := filepath.Join(v.homeDir, "tools", toolName, "venv")

	// Check if venv already exists
	if _, err := os.Stat(venvPath); err == nil {
		return venvPath, nil // Already exists
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(venvPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create tool directory: %w", err)
	}

	// Create venv using python -m venv
	cmd := exec.Command(v.pythonPath, "-m", "venv", venvPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create venv: %w\n%s", err, string(output))
	}

	return venvPath, nil
}

// GetPipPath returns the path to pip in the venv
func (v *VenvManager) GetPipPath(venvPath string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvPath, "Scripts", "pip.exe")
	}
	return filepath.Join(venvPath, "bin", "pip")
}

// GetPythonPath returns the path to python in the venv
func (v *VenvManager) GetPythonPath(venvPath string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvPath, "Scripts", "python.exe")
	}
	return filepath.Join(venvPath, "bin", "python")
}

// GetBinDir returns the directory containing executables in the venv
func (v *VenvManager) GetBinDir(venvPath string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvPath, "Scripts")
	}
	return filepath.Join(venvPath, "bin")
}

// Remove removes a virtual environment
func (v *VenvManager) Remove(toolName string) error {
	venvPath := filepath.Join(v.homeDir, "tools", toolName, "venv")

	if err := os.RemoveAll(venvPath); err != nil {
		return fmt.Errorf("failed to remove venv: %w", err)
	}

	return nil
}

// ListExecutables lists all executables in the venv bin directory
func (v *VenvManager) ListExecutables(venvPath string) ([]string, error) {
	binDir := v.GetBinDir(venvPath)

	entries, err := os.ReadDir(binDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read bin directory: %w", err)
	}

	executables := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip common Python infrastructure files
		name := entry.Name()
		if name == "python" || name == "python3" || name == "pip" || name == "pip3" ||
			name == "activate" || name == "activate.fish" || name == "activate.csh" {
			continue
		}

		// On Windows, skip .bat and .ps1 activation scripts
		if runtime.GOOS == "windows" {
			if name == "activate.bat" || name == "Activate.ps1" {
				continue
			}
		}

		executables = append(executables, name)
	}

	return executables, nil
}
