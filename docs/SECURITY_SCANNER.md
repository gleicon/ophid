# OPHID Security Scanner Design

**Component:** Supply Chain Security & Vulnerability Scanning
**Status:** Design Phase
**Priority:** Critical (Phase 2)

## Overview

The Security Scanner is a core differentiator for OPHID, providing built-in supply chain security for all installed tools and dependencies.

**Inspired by:** mcp-osv, osv-scanner, snyk, Dependabot

## Goals

1. **Automatic vulnerability detection** in dependencies
2. **Supply chain visibility** via SBOM generation
3. **License compliance** checking
4. **Provenance verification** (future)
5. **Auto-remediation** suggestions
6. **Zero configuration** - works out of the box

## Architecture

### Components

```
┌─────────────────────────────────────────────┐
│  Security Scanner                           │
├─────────────────────────────────────────────┤
│                                             │
│  ┌──────────────┐  ┌──────────────────┐    │
│  │ Vulnerability│  │ SBOM Generator   │    │
│  │ Scanner      │  │ (CycloneDX/SPDX) │    │
│  └──────┬───────┘  └─────────┬────────┘    │
│         │                    │              │
│  ┌──────▼───────┐  ┌─────────▼────────┐    │
│  │ OSV Database │  │ License Checker  │    │
│  │ Integration  │  │ (SPDX licenses)  │    │
│  └──────────────┘  └──────────────────┘    │
│                                             │
│  ┌──────────────────────────────────────┐  │
│  │ Remediation Engine                   │  │
│  │ - Auto-fix suggestions               │  │
│  │ - Dependency tree analysis           │  │
│  └──────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

### Data Flow

```
Install Request
     ↓
Parse Manifest (requirements.txt, package.json, etc.)
     ↓
Extract Dependencies
     ↓
┌────▼─────────────────────────────────────┐
│ Parallel Scanning:                       │
│  - Query OSV database for vulnerabilities│
│  - Check licenses against policy         │
│  - Generate SBOM                         │
└────┬─────────────────────────────────────┘
     ↓
Security Report
     ↓
┌────▼─────────────────────────────────────┐
│ Decision:                                │
│  - Block if critical vulnerabilities?    │
│  - Warn if medium/low?                   │
│  - Auto-fix if possible?                 │
└──────────────────────────────────────────┘
     ↓
Install or Abort
```

## Vulnerability Scanning

### OSV Database Integration

**OSV (Open Source Vulnerabilities):** https://osv.dev

**Why OSV:**
- Comprehensive (covers PyPI, npm, RubyGems, etc.)
- Free and open
- Well-maintained by Google
- REST API and offline database
- Used by GitHub, npm, etc.

### Implementation

```go
// internal/security/vulnerability.go
package security

import (
    "context"
    "github.com/google/osv-scanner/pkg/osv"
)

type VulnerabilityScanner struct {
    client    *osv.Client
    cache     *Cache        // Local cache for offline support
    threshold Severity      // Minimum severity to report
}

type Severity string

const (
    SeverityCritical Severity = "CRITICAL"
    SeverityHigh     Severity = "HIGH"
    SeverityMedium   Severity = "MEDIUM"
    SeverityLow      Severity = "LOW"
)

type Vulnerability struct {
    ID          string    // CVE-2023-1234 or GHSA-xxxx-yyyy-zzzz
    Package     string    // "paramiko"
    Version     string    // "3.0.0"
    Severity    Severity  // "HIGH"
    Summary     string    // Brief description
    Details     string    // Full description
    FixedIn     string    // "3.4.0"
    References  []string  // URLs to advisories
    CVSS        float64   // CVSS score
}

func (vs *VulnerabilityScanner) Scan(deps []*Dependency) ([]*Vulnerability, error) {
    ctx := context.Background()

    // Build OSV query
    query := &osv.BatchedQuery{
        Queries: make([]*osv.Query, len(deps)),
    }

    for i, dep := range deps {
        query.Queries[i] = &osv.Query{
            Package: osv.Package{
                Name:      dep.Name,
                Ecosystem: dep.Ecosystem, // "PyPI", "npm", etc.
            },
            Version: dep.Version,
        }
    }

    // Query OSV database (with caching)
    results, err := vs.client.QueryBatch(ctx, query)
    if err != nil {
        // Fall back to cached data if offline
        return vs.cache.Get(deps)
    }

    // Parse and filter results
    vulns := vs.parseResults(results)
    return vs.filterBySeverity(vulns), nil
}

func (vs *VulnerabilityScanner) SuggestFix(vuln *Vulnerability) (*Fix, error) {
    // Analyze dependency tree to find upgrade path
    // Check if upgrade breaks other dependencies
    // Return fix suggestion

    return &Fix{
        Package:     vuln.Package,
        FromVersion: vuln.Version,
        ToVersion:   vuln.FixedIn,
        Reason:      fmt.Sprintf("Fixes %s", vuln.ID),
        Safe:        vs.isUpgradeSafe(vuln.Package, vuln.FixedIn),
    }, nil
}
```

### Scanning Modes

**1. On Install (Default)**
```bash
$ ophid install cook
[Processing] Installing cook@0.1.0
[Processing] Scanning dependencies...
OK No vulnerabilities found
```

**2. Explicit Scan with Threshold**
```bash
$ ophid install cook --scan --severity medium
WARNING Found 1 medium severity vulnerability
  - CVE-2023-5678 in jinja2 3.0.0

? Proceed anyway? (y/N)
```

**3. Auto-Fix Mode**
```bash
$ ophid install cook --scan --fix
[Processing] Scanning dependencies...
WARNING Found 2 vulnerabilities
OK Auto-fixing...
  - paramiko 3.0.0 → 3.4.0 (fixes CVE-2023-1234)
  - jinja2 3.0.0 → 3.1.2 (fixes CVE-2023-5678)
OK Re-scanning... No vulnerabilities
```

**4. Continuous Monitoring**
```bash
$ ophid scan --watch
[Processing] Monitoring installed tools for new vulnerabilities...
[Processing] Checking every 24 hours

[2025-01-10 14:30] WARNING New vulnerability in cook:
  CVE-2025-9999 in click 8.0.0 (HIGH)
  Fix: ophid upgrade cook
```

**5. CI/CD Mode**
```bash
$ ophid scan --exit-code
Exit code 0: No vulnerabilities
Exit code 1: Vulnerabilities found

# In GitHub Actions:
- name: Security Scan
  run: ophid scan --exit-code || exit 1
```

## SBOM Generation

### Software Bill of Materials

**Format:** CycloneDX (primary) + SPDX (optional)

**Why CycloneDX:**
- Industry standard for security
- Supports vulnerability references
- Machine-readable (JSON/XML)
- Used by OWASP, CISA, etc.

### Implementation

```go
// internal/security/sbom.go
package security

import (
    cdx "github.com/CycloneDX/cyclonedx-go"
)

type SBOMGenerator struct {
    format string // "cyclonedx", "spdx"
}

type SBOM struct {
    Format      string       // "CycloneDX"
    Version     string       // "1.5"
    Tool        string       // "OPHID v1.0.0"
    Timestamp   time.Time
    Components  []Component
    Dependencies []Dependency
}

func (sg *SBOMGenerator) Generate(manifest *Manifest) (*SBOM, error) {
    bom := cdx.NewBOM()

    // Set metadata
    bom.Metadata = &cdx.Metadata{
        Timestamp: time.Now().Format(time.RFC3339),
        Tools: []cdx.Tool{
            {
                Vendor: "OPHID",
                Name:   "ophid",
                Version: VERSION,
            },
        },
    }

    // Add components (dependencies)
    for _, dep := range manifest.Dependencies {
        component := cdx.Component{
            Type:    cdx.ComponentTypeLibrary,
            Name:    dep.Name,
            Version: dep.Version,
            PackageURL: dep.PURL(), // pkg:pypi/click@8.0.0
            Licenses: sg.getLicenses(dep),
            Hashes: []cdx.Hash{
                {
                    Algorithm: cdx.HashAlgoSHA256,
                    Value:     dep.SHA256,
                },
            },
        }

        bom.Components = &[]cdx.Component{component}
    }

    return bom, nil
}

func (sg *SBOMGenerator) Export(sbom *SBOM, format string) ([]byte, error) {
    switch format {
    case "json":
        return json.MarshalIndent(sbom, "", "  ")
    case "xml":
        return xml.MarshalIndent(sbom, "", "  ")
    default:
        return nil, fmt.Errorf("unknown format: %s", format)
    }
}
```

### SBOM Output

```bash
$ ophid install cook --scan
OK cook@0.1.0 installed
OK SBOM generated: ~/.ophid/tools/cook@0.1.0/sbom.json

$ cat ~/.ophid/tools/cook@0.1.0/sbom.json
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "metadata": {
    "timestamp": "2025-01-10T14:30:00Z",
    "tools": [
      {
        "vendor": "OPHID",
        "name": "ophid",
        "version": "1.0.0"
      }
    ]
  },
  "components": [
    {
      "type": "library",
      "name": "click",
      "version": "8.1.7",
      "purl": "pkg:pypi/click@8.1.7",
      "licenses": [
        {
          "license": {
            "id": "BSD-3-Clause"
          }
        }
      ],
      "hashes": [
        {
          "alg": "SHA-256",
          "content": "abc123..."
        }
      ]
    }
  ]
}
```

### SBOM Use Cases

1. **Compliance:** Provide to security team
2. **Auditing:** Track what's installed
3. **Incident response:** Know what's affected
4. **Supply chain transparency:** See all dependencies

## License Compliance

### License Checker

**Goal:** Ensure dependencies use acceptable licenses

**Common licenses:**
- **Permissive:** MIT, Apache-2.0, BSD (Allowed)
- **Weak copyleft:** MPL, LGPL (Warning)
- **Strong copyleft:** GPL, AGPL (might be blocked)

### Implementation

```go
// internal/security/license.go
package security

type LicenseChecker struct {
    policy *LicensePolicy
}

type LicensePolicy struct {
    Allowed    []string // ["MIT", "Apache-2.0", "BSD-3-Clause"]
    Warned     []string // ["LGPL-2.1", "MPL-2.0"]
    Blocked    []string // ["GPL-3.0", "AGPL-3.0"]
}

type LicenseViolation struct {
    Package  string
    Version  string
    License  string
    Severity string // "warning", "error"
    Policy   string // "blocked", "warned"
}

func (lc *LicenseChecker) Check(deps []*Dependency) ([]*LicenseViolation, error) {
    violations := []*LicenseViolation{}

    for _, dep := range deps {
        license := dep.License

        if lc.isBlocked(license) {
            violations = append(violations, &LicenseViolation{
                Package:  dep.Name,
                Version:  dep.Version,
                License:  license,
                Severity: "error",
                Policy:   "blocked",
            })
        } else if lc.isWarned(license) {
            violations = append(violations, &LicenseViolation{
                Package:  dep.Name,
                Version:  dep.Version,
                License:  license,
                Severity: "warning",
                Policy:   "warned",
            })
        }
    }

    return violations, nil
}

func (lc *LicenseChecker) isBlocked(license string) bool {
    for _, blocked := range lc.policy.Blocked {
        if strings.EqualFold(license, blocked) {
            return true
        }
    }
    return false
}
```

### License Policy Configuration

```toml
# ~/.ophid/config.toml
[security.licenses]
allowed = [
    "MIT",
    "Apache-2.0",
    "BSD-2-Clause",
    "BSD-3-Clause",
    "ISC",
]

warned = [
    "LGPL-2.1",
    "LGPL-3.0",
    "MPL-2.0",
]

blocked = [
    "GPL-2.0",
    "GPL-3.0",
    "AGPL-3.0",
]
```

### License Checking in Action

```bash
$ ophid install some-tool --scan
[Processing] Installing some-tool@1.0.0
[Processing] Checking licenses...

WARNING License violations found:
  - cryptography 38.0.0: Apache-2.0 OR BSD-3-Clause (OK)
  - pyreadline 2.1: GPL-3.0 (BLOCKED)

BLOCKED Installation blocked due to license policy
   Package 'pyreadline' uses GPL-3.0 which is not allowed

? Override policy? (y/N) n
```

## Remediation Engine

### Auto-Fix Suggestions

**Goal:** Suggest safe ways to fix vulnerabilities

```go
// internal/security/remediation.go
package security

type Remediation struct {
    Vulnerability *Vulnerability
    Fixes         []*Fix
    Recommended   *Fix
}

type Fix struct {
    Type        string // "upgrade", "replace", "patch"
    Package     string
    FromVersion string
    ToVersion   string
    Reason      string
    Safe        bool   // Won't break other deps
    Confidence  float64 // 0.0 - 1.0
}

type RemediationEngine struct {
    depTree *DependencyTree
}

func (re *RemediationEngine) Suggest(vuln *Vulnerability) (*Remediation, error) {
    fixes := []*Fix{}

    // 1. Try direct upgrade
    if vuln.FixedIn != "" {
        fix := &Fix{
            Type:        "upgrade",
            Package:     vuln.Package,
            FromVersion: vuln.Version,
            ToVersion:   vuln.FixedIn,
            Reason:      fmt.Sprintf("Fixes %s", vuln.ID),
            Safe:        re.isUpgradeSafe(vuln.Package, vuln.FixedIn),
            Confidence:  0.9,
        }
        fixes = append(fixes, fix)
    }

    // 2. Try alternative package
    if alt := re.findAlternative(vuln.Package); alt != nil {
        fix := &Fix{
            Type:        "replace",
            Package:     alt.Name,
            FromVersion: vuln.Version,
            ToVersion:   alt.Version,
            Reason:      fmt.Sprintf("Alternative to %s", vuln.Package),
            Safe:        re.isReplacementSafe(vuln.Package, alt.Name),
            Confidence:  0.6,
        }
        fixes = append(fixes, fix)
    }

    // 3. Pick recommended fix
    recommended := re.pickBest(fixes)

    return &Remediation{
        Vulnerability: vuln,
        Fixes:         fixes,
        Recommended:   recommended,
    }, nil
}

func (re *RemediationEngine) isUpgradeSafe(pkg, version string) bool {
    // Check if upgrade breaks any other dependencies
    // Use dependency tree analysis

    conflicts := re.depTree.FindConflicts(pkg, version)
    return len(conflicts) == 0
}
```

### Remediation Output

```bash
$ ophid scan cook --fix

[Processing] Scanning cook@0.1.0...
WARNING Found 1 vulnerability:

CVE-2023-1234 in paramiko 3.0.0 (HIGH)
  Summary: Authentication bypass in SSH server
  CVSS: 7.5

Recommended Fix:
  OK Upgrade paramiko 3.0.0 → 3.4.0
  Confidence: High (no dependency conflicts)

? Apply fix? (Y/n) y

[Processing] Upgrading paramiko...
OK paramiko 3.4.0 installed
[Processing] Re-scanning...
OK No vulnerabilities found
```

## Security Report

### Report Format

```go
// pkg/types/security.go
package types

type SecurityReport struct {
    Tool            string
    Version         string
    ScanTime        time.Time
    Vulnerabilities []*Vulnerability
    Licenses        []*LicenseViolation
    SBOM            *SBOM
    Summary         *SecuritySummary
}

type SecuritySummary struct {
    TotalDependencies int
    CriticalVulns     int
    HighVulns         int
    MediumVulns       int
    LowVulns          int
    LicenseViolations int
    Score             float64 // 0-100 security score
}
```

### Report Output

```bash
$ ophid scan cook --report

════════════════════════════════════════════════
  Security Report: cook@0.1.0
════════════════════════════════════════════════

Scan Time: 2025-01-10 14:30:00

SUMMARY
  Dependencies:       15
  Vulnerabilities:    2
    - Critical:       0
    - High:           1
    - Medium:         1
    - Low:            0
  License Issues:     0
  Security Score:     85/100

VULNERABILITIES

  [HIGH] CVE-2023-1234
  Package:   paramiko 3.0.0
  Fixed in:  3.4.0
  CVSS:      7.5
  Summary:   Authentication bypass in SSH server

  [MEDIUM] CVE-2023-5678
  Package:   jinja2 3.0.0
  Fixed in:  3.1.2
  CVSS:      5.3
  Summary:   Template injection vulnerability

REMEDIATION

  Run: ophid scan cook --fix

SBOM

  Generated: ~/.ophid/tools/cook@0.1.0/sbom.json
  Format:    CycloneDX 1.5

════════════════════════════════════════════════
```

## Integration Points

### CLI Commands

```bash
# Scan on install (default)
ophid install cook

# Scan with custom threshold
ophid install cook --scan --severity high

# Auto-fix vulnerabilities
ophid install cook --scan --fix

# Scan existing installation
ophid scan cook

# Scan all tools
ophid scan --all

# Continuous monitoring
ophid scan --watch

# Generate report
ophid scan cook --report

# Export SBOM
ophid sbom cook --format json

# CI/CD mode (exit code)
ophid scan --exit-code
```

### Configuration

```toml
# ~/.ophid/config.toml
[security]
enabled = true
scan_on_install = true
severity_threshold = "medium"  # critical, high, medium, low
auto_fix = false
watch_interval = "24h"

[security.osv]
api_url = "https://api.osv.dev/v1"
cache_ttl = "24h"
offline_fallback = true

[security.sbom]
format = "cyclonedx"  # cyclonedx, spdx
auto_generate = true
output_dir = "~/.ophid/sbom"

[security.licenses]
check = true
policy = "strict"  # strict, permissive
allowed = ["MIT", "Apache-2.0", "BSD-3-Clause"]
blocked = ["GPL-3.0", "AGPL-3.0"]
```

## Performance Considerations

### Caching Strategy

1. **OSV Database:** Cache for 24 hours
2. **License data:** Cache per package version
3. **SBOM:** Generate once, update on dependency change

### Parallel Scanning

```go
func (vs *VulnerabilityScanner) ScanParallel(deps []*Dependency) ([]*Vulnerability, error) {
    // Split deps into batches
    batchSize := 50
    batches := splitIntoBatches(deps, batchSize)

    // Scan in parallel
    resultChan := make(chan []*Vulnerability, len(batches))
    errChan := make(chan error, len(batches))

    for _, batch := range batches {
        go func(b []*Dependency) {
            vulns, err := vs.Scan(b)
            if err != nil {
                errChan <- err
                return
            }
            resultChan <- vulns
        }(batch)
    }

    // Collect results
    // ...
}
```

## Open Questions

1. **OSV Database:**
   - Vendor database locally or query API?
   - Update frequency for local database?

2. **Auto-Fix:**
   - Should it be opt-in or opt-out?
   - How aggressive should upgrade suggestions be?

3. **Licenses:**
   - Default policy (strict or permissive)?
   - Support for custom license policies per organization?

4. **SBOM:**
   - Sign SBOMs for integrity?
   - Upload to central registry?

5. **Performance:**
   - Async scanning in background?
   - Incremental scans (only new deps)?

## Next Steps

1. PASS Design complete
2. Pending Implement OSV client integration
3. Pending Implement SBOM generator
4. Pending Implement license checker
5. Pending Implement remediation engine
6. Pending Add CLI commands
7. Pending Write tests
8. Pending Documentation

**Related:** [PLATFORM_VISION.md](../PLATFORM_VISION.md)
