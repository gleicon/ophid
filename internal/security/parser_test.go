package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRequirementLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    Package
		wantErr bool
	}{
		{
			name: "simple package with ==",
			line: "requests==2.28.0",
			want: Package{Name: "requests", Version: "2.28.0", Ecosystem: "PyPI"},
		},
		{
			name: "package with >=",
			line: "flask>=2.0.0",
			want: Package{Name: "flask", Version: "2.0.0", Ecosystem: "PyPI"},
		},
		{
			name: "package with extras",
			line: "requests[security]==2.28.0",
			want: Package{Name: "requests", Version: "2.28.0", Ecosystem: "PyPI"},
		},
		{
			name: "package without version",
			line: "pytest",
			want: Package{Name: "pytest", Version: "latest", Ecosystem: "PyPI"},
		},
		{
			name: "package with inline comment",
			line: "django==4.0.0  # Web framework",
			want: Package{Name: "django", Version: "4.0.0", Ecosystem: "PyPI"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRequirementLine(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRequirementLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.Name != tt.want.Name || got.Version != tt.want.Version || got.Ecosystem != tt.want.Ecosystem {
				t.Errorf("parseRequirementLine() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseRequirementsTxt(t *testing.T) {
	// Create temp requirements.txt
	tmpDir := t.TempDir()
	reqFile := filepath.Join(tmpDir, "requirements.txt")

	content := `# Test requirements
requests==2.28.0
flask>=2.0.0

# Development dependencies
pytest==7.0.0
`
	if err := os.WriteFile(reqFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	packages, err := ParseRequirementsTxt(reqFile)
	if err != nil {
		t.Fatalf("ParseRequirementsTxt() error = %v", err)
	}

	if len(packages) != 3 {
		t.Errorf("ParseRequirementsTxt() got %d packages, want 3", len(packages))
	}

	// Check first package
	if packages[0].Name != "requests" || packages[0].Version != "2.28.0" {
		t.Errorf("First package = %+v, want requests==2.28.0", packages[0])
	}
}

func TestParseGoModLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    Package
		wantErr bool
	}{
		{
			name: "simple require",
			line: "github.com/spf13/cobra v1.8.0",
			want: Package{Name: "github.com/spf13/cobra", Version: "v1.8.0", Ecosystem: "Go"},
		},
		{
			name: "require with prefix",
			line: "require github.com/sirupsen/logrus v1.9.3",
			want: Package{Name: "github.com/sirupsen/logrus", Version: "v1.9.3", Ecosystem: "Go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGoModLine(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGoModLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && (got.Name != tt.want.Name || got.Version != tt.want.Version) {
				t.Errorf("parseGoModLine() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseGoMod(t *testing.T) {
	// Create temp go.mod
	tmpDir := t.TempDir()
	goModFile := filepath.Join(tmpDir, "go.mod")

	content := `module github.com/example/project

go 1.21

require (
	github.com/spf13/cobra v1.8.0
	github.com/sirupsen/logrus v1.9.3
)
`
	if err := os.WriteFile(goModFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	packages, err := ParseGoMod(goModFile)
	if err != nil {
		t.Fatalf("ParseGoMod() error = %v", err)
	}

	if len(packages) != 2 {
		t.Errorf("ParseGoMod() got %d packages, want 2", len(packages))
	}
}
