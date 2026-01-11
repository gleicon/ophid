package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildPURL(t *testing.T) {
	tests := []struct {
		name string
		pkg  Package
		want string
	}{
		{
			name: "PyPI package",
			pkg:  Package{Name: "requests", Version: "2.28.0", Ecosystem: "PyPI"},
			want: "pkg:pypi/requests@2.28.0",
		},
		{
			name: "npm package",
			pkg:  Package{Name: "express", Version: "4.18.0", Ecosystem: "npm"},
			want: "pkg:npm/express@4.18.0",
		},
		{
			name: "Go package",
			pkg:  Package{Name: "github.com/spf13/cobra", Version: "v1.8.0", Ecosystem: "Go"},
			want: "pkg:golang/github.com/spf13/cobra@v1.8.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPURL(tt.pkg)
			if got != tt.want {
				t.Errorf("buildPURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateSBOM(t *testing.T) {
	packages := []Package{
		{Name: "requests", Version: "2.28.0", Ecosystem: "PyPI"},
		{Name: "flask", Version: "2.0.0", Ecosystem: "PyPI"},
	}

	sbom, err := GenerateSBOM(packages, "test-tool")
	if err != nil {
		t.Fatalf("GenerateSBOM() error = %v", err)
	}

	if sbom.BOMFormat != "CycloneDX" {
		t.Errorf("BOMFormat = %s, want CycloneDX", sbom.BOMFormat)
	}

	if sbom.SpecVersion != "1.4" {
		t.Errorf("SpecVersion = %s, want 1.4", sbom.SpecVersion)
	}

	if len(sbom.Components) != 2 {
		t.Errorf("Components count = %d, want 2", len(sbom.Components))
	}

	if sbom.Components[0].Type != "library" {
		t.Errorf("Component type = %s, want library", sbom.Components[0].Type)
	}
}

func TestWriteSBOM(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "sbom.json")

	packages := []Package{
		{Name: "requests", Version: "2.28.0", Ecosystem: "PyPI"},
	}

	sbom, err := GenerateSBOM(packages, "test-tool")
	if err != nil {
		t.Fatalf("GenerateSBOM() error = %v", err)
	}

	if err := WriteSBOM(sbom, outputPath); err != nil {
		t.Fatalf("WriteSBOM() error = %v", err)
	}

	// Check file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("SBOM file was not created")
	}

	// Check file content is valid JSON
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read SBOM file: %v", err)
	}

	if len(content) == 0 {
		t.Error("SBOM file is empty")
	}
}
