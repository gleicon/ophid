package tool

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVenvManager_GetPipPath(t *testing.T) {
	homeDir := "/home/user/.ophid"
	pythonPath := "/usr/bin/python3"
	venvMgr := NewVenvManager(homeDir, pythonPath)

	venvPath := "/home/user/.ophid/tools/test/venv"

	pipPath := venvMgr.GetPipPath(venvPath)

	if runtime.GOOS == "windows" {
		expected := filepath.Join(venvPath, "Scripts", "pip.exe")
		if pipPath != expected {
			t.Errorf("GetPipPath() = %s, want %s", pipPath, expected)
		}
	} else {
		expected := filepath.Join(venvPath, "bin", "pip")
		if pipPath != expected {
			t.Errorf("GetPipPath() = %s, want %s", pipPath, expected)
		}
	}
}

func TestVenvManager_GetPythonPath(t *testing.T) {
	homeDir := "/home/user/.ophid"
	pythonPath := "/usr/bin/python3"
	venvMgr := NewVenvManager(homeDir, pythonPath)

	venvPath := "/home/user/.ophid/tools/test/venv"

	pythonExePath := venvMgr.GetPythonPath(venvPath)

	if runtime.GOOS == "windows" {
		expected := filepath.Join(venvPath, "Scripts", "python.exe")
		if pythonExePath != expected {
			t.Errorf("GetPythonPath() = %s, want %s", pythonExePath, expected)
		}
	} else {
		expected := filepath.Join(venvPath, "bin", "python")
		if pythonExePath != expected {
			t.Errorf("GetPythonPath() = %s, want %s", pythonExePath, expected)
		}
	}
}

func TestVenvManager_GetBinDir(t *testing.T) {
	homeDir := "/home/user/.ophid"
	pythonPath := "/usr/bin/python3"
	venvMgr := NewVenvManager(homeDir, pythonPath)

	venvPath := "/home/user/.ophid/tools/test/venv"

	binDir := venvMgr.GetBinDir(venvPath)

	if runtime.GOOS == "windows" {
		expected := filepath.Join(venvPath, "Scripts")
		if binDir != expected {
			t.Errorf("GetBinDir() = %s, want %s", binDir, expected)
		}
	} else {
		expected := filepath.Join(venvPath, "bin")
		if binDir != expected {
			t.Errorf("GetBinDir() = %s, want %s", binDir, expected)
		}
	}
}

func TestVenvManager_ListExecutables(t *testing.T) {
	// Create temp venv structure
	tmpDir := t.TempDir()
	venvPath := filepath.Join(tmpDir, "venv")
	binDir := filepath.Join(venvPath, "bin")

	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	// Create some executable files
	executables := []string{"ansible", "ansible-playbook", "molecule"}
	for _, exe := range executables {
		exePath := filepath.Join(binDir, exe)
		if err := os.WriteFile(exePath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create executable: %v", err)
		}
	}

	// Create infrastructure files that should be filtered out
	infraFiles := []string{"python", "python3", "pip", "activate"}
	for _, file := range infraFiles {
		filePath := filepath.Join(binDir, file)
		if err := os.WriteFile(filePath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create infra file: %v", err)
		}
	}

	venvMgr := NewVenvManager(tmpDir, "/usr/bin/python3")
	found, err := venvMgr.ListExecutables(venvPath)
	if err != nil {
		t.Fatalf("ListExecutables() error = %v", err)
	}

	if len(found) != len(executables) {
		t.Errorf("ListExecutables() found %d executables, want %d", len(found), len(executables))
	}

	// Check that infrastructure files were filtered out
	for _, exe := range found {
		if exe == "python" || exe == "pip" || exe == "activate" {
			t.Errorf("ListExecutables() included infrastructure file: %s", exe)
		}
	}
}
