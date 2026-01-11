package security

import (
	"testing"
)

func TestValidatePackageName(t *testing.T) {
	tests := []struct {
		name    string
		pkg     string
		wantErr bool
	}{
		{"valid package", "requests", false},
		{"valid with dash", "python-dateutil", false},
		{"empty package", "", true},
		{"path traversal", "../evil", true},
		{"slash in name", "some/package", true},
		{"too long", string(make([]byte, 300)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePackageName(tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePackageName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{"valid version", "1.2.3", false},
		{"valid semver", "2.28.0", false},
		{"empty version", "", true},
		{"path traversal", "../1.0.0", true},
		{"too long", string(make([]byte, 100)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestScanResult_HasVulnerabilities(t *testing.T) {
	tests := []struct {
		name   string
		result ScanResult
		want   bool
	}{
		{
			name: "no vulnerabilities",
			result: ScanResult{
				Package:         Package{Name: "test", Version: "1.0.0"},
				Vulnerabilities: []OSVVulnerability{},
			},
			want: false,
		},
		{
			name: "has vulnerabilities",
			result: ScanResult{
				Package: Package{Name: "test", Version: "1.0.0"},
				Vulnerabilities: []OSVVulnerability{
					{ID: "GHSA-1234", Summary: "Test vulnerability"},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.HasVulnerabilities()
			if got != tt.want {
				t.Errorf("HasVulnerabilities() = %v, want %v", got, tt.want)
			}
		})
	}
}
