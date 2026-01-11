package tool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManifest_LoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()

	venvMgr := NewVenvManager(tmpDir, "/usr/bin/python3")
	installer, err := NewInstaller(tmpDir, venvMgr)
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	// Add a tool to manifest
	tool := &Tool{
		Name:        "test-tool",
		Version:     "1.0.0",
		Ecosystem:   "python",
		Runtime:     "python3",
		InstallPath: "/path/to/venv",
		Executables: []string{"test"},
		InstalledAt: time.Now(),
	}

	installer.manifest.Tools["test-tool"] = tool
	installer.manifest.UpdatedAt = time.Now()

	// Save manifest
	if err := installer.saveManifest(); err != nil {
		t.Fatalf("saveManifest() error = %v", err)
	}

	// Load manifest
	venvMgr2 := NewVenvManager(tmpDir, "/usr/bin/python3")
	installer2, err := NewInstaller(tmpDir, venvMgr2)
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	// Check tool was loaded
	loadedTool, exists := installer2.manifest.Tools["test-tool"]
	if !exists {
		t.Fatal("Tool was not loaded from manifest")
	}

	if loadedTool.Name != "test-tool" || loadedTool.Version != "1.0.0" {
		t.Errorf("Tool data mismatch: got %+v", loadedTool)
	}
}

func TestManifest_EmptyState(t *testing.T) {
	tmpDir := t.TempDir()

	venvMgr := NewVenvManager(tmpDir, "/usr/bin/python3")
	installer, err := NewInstaller(tmpDir, venvMgr)
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	// Manifest should be empty initially
	if len(installer.manifest.Tools) != 0 {
		t.Errorf("Expected empty manifest, got %d tools", len(installer.manifest.Tools))
	}
}

func TestManifest_JSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "tools", "manifest.json")

	// Create manifest directory
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		t.Fatalf("failed to create manifest directory: %v", err)
	}

	// Create a sample manifest
	manifest := &ToolManifest{
		Tools: map[string]*Tool{
			"ansible": {
				Name:        "ansible",
				Version:     "2.10.0",
				Ecosystem:   "python",
				Runtime:     "python3",
				InstallPath: "/path/to/venv",
				Executables: []string{"ansible", "ansible-playbook"},
				InstalledAt: time.Now(),
			},
		},
		UpdatedAt: time.Now(),
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}

	// Write to file
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Load it back
	venvMgr := NewVenvManager(tmpDir, "/usr/bin/python3")
	installer, err := NewInstaller(tmpDir, venvMgr)
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	// Verify ansible was loaded
	ansible, exists := installer.manifest.Tools["ansible"]
	if !exists {
		t.Fatal("ansible was not loaded from manifest")
	}

	if ansible.Version != "2.10.0" {
		t.Errorf("Version = %s, want 2.10.0", ansible.Version)
	}

	if len(ansible.Executables) != 2 {
		t.Errorf("Executables count = %d, want 2", len(ansible.Executables))
	}
}

func TestInstaller_List(t *testing.T) {
	tmpDir := t.TempDir()

	venvMgr := NewVenvManager(tmpDir, "/usr/bin/python3")
	installer, err := NewInstaller(tmpDir, venvMgr)
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	// Add some tools
	installer.manifest.Tools["tool1"] = &Tool{Name: "tool1", Version: "1.0.0"}
	installer.manifest.Tools["tool2"] = &Tool{Name: "tool2", Version: "2.0.0"}

	// List tools
	tools := installer.List()

	if len(tools) != 2 {
		t.Errorf("List() returned %d tools, want 2", len(tools))
	}
}

func TestInstaller_Get(t *testing.T) {
	tmpDir := t.TempDir()

	venvMgr := NewVenvManager(tmpDir, "/usr/bin/python3")
	installer, err := NewInstaller(tmpDir, venvMgr)
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	// Add a tool
	installer.manifest.Tools["ansible"] = &Tool{
		Name:    "ansible",
		Version: "2.10.0",
	}

	// Get the tool
	tool, err := installer.Get("ansible")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if tool.Name != "ansible" || tool.Version != "2.10.0" {
		t.Errorf("Get() returned %+v, want ansible@2.10.0", tool)
	}

	// Try to get non-existent tool
	_, err = installer.Get("nonexistent")
	if err == nil {
		t.Error("Get() should return error for non-existent tool")
	}
}
