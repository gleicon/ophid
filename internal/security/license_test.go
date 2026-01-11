package security

import (
	"testing"
)

func TestIsPermissive(t *testing.T) {
	tests := []struct {
		license string
		want    bool
	}{
		{"MIT", true},
		{"Apache-2.0", true},
		{"BSD-3-Clause", true},
		{"GPL-3.0", false},
		{"AGPL-3.0", false},
		{"Unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.license, func(t *testing.T) {
			got := IsPermissive(tt.license)
			if got != tt.want {
				t.Errorf("IsPermissive(%s) = %v, want %v", tt.license, got, tt.want)
			}
		})
	}
}

func TestIsCopyleft(t *testing.T) {
	tests := []struct {
		license string
		want    bool
	}{
		{"GPL-3.0", true},
		{"LGPL-2.1", true},
		{"AGPL-3.0", true},
		{"MPL-2.0", true},
		{"MIT", false},
		{"Apache-2.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.license, func(t *testing.T) {
			got := IsCopyleft(tt.license)
			if got != tt.want {
				t.Errorf("IsCopyleft(%s) = %v, want %v", tt.license, got, tt.want)
			}
		})
	}
}

func TestIsOSIApproved(t *testing.T) {
	tests := []struct {
		license string
		want    bool
	}{
		{"MIT", true},
		{"Apache-2.0", true},
		{"GPL-3.0", true},
		{"Unknown-License", false},
	}

	for _, tt := range tests {
		t.Run(tt.license, func(t *testing.T) {
			got := IsOSIApproved(tt.license)
			if got != tt.want {
				t.Errorf("IsOSIApproved(%s) = %v, want %v", tt.license, got, tt.want)
			}
		})
	}
}

func TestLicenseChecker_CheckLicense(t *testing.T) {
	// Only allow permissive licenses
	checker := NewLicenseChecker([]LicenseType{LicensePermissive})

	tests := []struct {
		license string
		allowed bool
	}{
		{"MIT", true},
		{"Apache-2.0", true},
		{"GPL-3.0", false},
		{"AGPL-3.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.license, func(t *testing.T) {
			_, allowed := checker.CheckLicense(tt.license)
			if allowed != tt.allowed {
				t.Errorf("CheckLicense(%s) allowed = %v, want %v", tt.license, allowed, tt.allowed)
			}
		})
	}
}

func TestLicenseChecker_AllowCopyleft(t *testing.T) {
	// Allow both permissive and copyleft
	checker := NewLicenseChecker([]LicenseType{LicensePermissive, LicenseCopyleft})

	tests := []struct {
		license string
		allowed bool
	}{
		{"MIT", true},
		{"GPL-3.0", true},
		{"AGPL-3.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.license, func(t *testing.T) {
			_, allowed := checker.CheckLicense(tt.license)
			if allowed != tt.allowed {
				t.Errorf("CheckLicense(%s) allowed = %v, want %v", tt.license, allowed, tt.allowed)
			}
		})
	}
}
