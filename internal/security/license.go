package security

import (
	"strings"
)

// LicenseType represents license classification
type LicenseType int

const (
	LicenseUnknown LicenseType = iota
	LicensePermissive
	LicenseCopyleft
	LicenseProprietary
)

// LicenseInfo represents license information
type LicenseInfo struct {
	Name string
	Type LicenseType
	OSI  bool // OSI approved
}

// Common licenses and their classification
var knownLicenses = map[string]LicenseInfo{
	// Permissive
	"MIT":          {Name: "MIT License", Type: LicensePermissive, OSI: true},
	"Apache-2.0":   {Name: "Apache License 2.0", Type: LicensePermissive, OSI: true},
	"BSD-2-Clause": {Name: "BSD 2-Clause License", Type: LicensePermissive, OSI: true},
	"BSD-3-Clause": {Name: "BSD 3-Clause License", Type: LicensePermissive, OSI: true},
	"ISC":          {Name: "ISC License", Type: LicensePermissive, OSI: true},
	"0BSD":         {Name: "BSD Zero Clause License", Type: LicensePermissive, OSI: true},

	// Copyleft
	"GPL-2.0":   {Name: "GNU General Public License v2.0", Type: LicenseCopyleft, OSI: true},
	"GPL-3.0":   {Name: "GNU General Public License v3.0", Type: LicenseCopyleft, OSI: true},
	"LGPL-2.1":  {Name: "GNU Lesser General Public License v2.1", Type: LicenseCopyleft, OSI: true},
	"LGPL-3.0":  {Name: "GNU Lesser General Public License v3.0", Type: LicenseCopyleft, OSI: true},
	"AGPL-3.0":  {Name: "GNU Affero General Public License v3.0", Type: LicenseCopyleft, OSI: true},
	"MPL-2.0":   {Name: "Mozilla Public License 2.0", Type: LicenseCopyleft, OSI: true},
	"EPL-2.0":   {Name: "Eclipse Public License 2.0", Type: LicenseCopyleft, OSI: true},
}

// LicenseChecker checks license compatibility
type LicenseChecker struct {
	allowedTypes []LicenseType
}

// NewLicenseChecker creates a new license checker
func NewLicenseChecker(allowedTypes []LicenseType) *LicenseChecker {
	return &LicenseChecker{
		allowedTypes: allowedTypes,
	}
}

// CheckLicense checks if a license is allowed
func (lc *LicenseChecker) CheckLicense(license string) (LicenseInfo, bool) {
	// Normalize license name
	license = strings.TrimSpace(license)

	// Look up license info
	info, found := knownLicenses[license]
	if !found {
		// Try case-insensitive match
		for key, val := range knownLicenses {
			if strings.EqualFold(key, license) {
				info = val
				found = true
				break
			}
		}
	}

	if !found {
		return LicenseInfo{Name: license, Type: LicenseUnknown, OSI: false}, false
	}

	// Check if license type is allowed
	for _, allowedType := range lc.allowedTypes {
		if info.Type == allowedType {
			return info, true
		}
	}

	return info, false
}

// IsPermissive returns true if the license is permissive
func IsPermissive(license string) bool {
	info, found := knownLicenses[license]
	if !found {
		return false
	}
	return info.Type == LicensePermissive
}

// IsCopyleft returns true if the license is copyleft
func IsCopyleft(license string) bool {
	info, found := knownLicenses[license]
	if !found {
		return false
	}
	return info.Type == LicenseCopyleft
}

// IsOSIApproved returns true if the license is OSI approved
func IsOSIApproved(license string) bool {
	info, found := knownLicenses[license]
	if !found {
		return false
	}
	return info.OSI
}
