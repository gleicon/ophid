# OPHID - Operations Python Hybrid Distribution

Ophid is a Go-powered runtime manager and tool supervisor for Python operations tools. Makes Python-based infrastructure tools trivial to install and run with minimal Python knowledge required.

## Why Ophid?

This project originated within a set of tools for a private PaaS named lathe. One of this components is [Cook](https://github.com/gleicon/cook) a configuration management DSL using Python and [Guvnor](https://github.com/gleicon/guvnor) a Go-based tool supervisor.

The reason behind it is that Go is a compiled language that is easy to install and run on any platform, while Python is a scripting language that is easy to write and read. By combining the two, we can create a tool that is easy to install and run on any platform, while also being easy to write and read. 

I wanted the tooling I use in Go for Python. I've tried `pipx`, `uv` and others and while great, when starting up a project like cook or ansible I'd have to provide a bootstrap script or image. I also wanted some parts of tools like supervisord which are very practical and basically what I use on VMs and VPS instead of spinning up a Kubernetes cluster.

The cognitive change of shipping something that uses a scripted language with plain text packages versus a compiled language as Go interested me. I also was looking into ways to ensure safety from credential leaks and supply chain attacks after building https://github.com/gleicon/mcp-osv so I got parts of all of them to build Ophid.

While focused on Python nothing stops Ophid to be used with other runtimes. Right now Python is the only one provided as it was already working on Lathe.

## Features

### Runtime Management
- Download and install Python runtimes from python-build-standalone
- Isolated runtime environments per version
- Cross-platform support (Linux, macOS, Windows)

### Security Scanner
- Vulnerability scanning via OSV.dev
- SBOM generation (CycloneDX 1.4)
- License compliance checking
- Requirements.txt and go.mod parsing

### Tool Installation 
- Isolated virtual environments per tool
- Multi-source installation support:
  - PyPI packages (traditional `pip install`)
  - GitHub repositories (`github.com/user/repo`)
  - Git repositories (any Git URL)
  - Local directories (for development)
- Unified security scanning for all sources
- Tool manifest and version tracking
- Executable discovery and management
- Automatic SBOM generation per tool

### Process Supervisor
- Process lifecycle management (start/stop/restart)
- Auto-restart on failure
- Health checking (HTTP, process)
- Background process execution

### Reverse Proxy
- Production-grade HTTP/HTTPS reverse proxy
- Automatic TLS via Let's Encrypt (ACME)
- Load balancing (round-robin, least-conn, IP hash, weighted)
- WebSocket support
- Middleware pipeline (rate limiting, CORS, logging)
- Static file serving
- Hot configuration reload

## Installation

### From Source

```bash
git clone https://github.com/gleicon/ophid
cd ophid
make build
./build/ophid --version
```

### Binary Release

Download from releases page at [github releases](https://github.com/gleicon/ophid/releases)

## Quick Start

```bash
# Install Python runtime
ophid runtime install 3.12.1

# Install a tool from PyPI
ophid install ansible

# And/or Install from GitHub
ophid install gleicon/redis-tools

# You can also install from a local directory
ophid install ./my-local-project

# List installed tools
ophid list

# Run a tool
ophid run ansible --version

# Run tool in background with auto-restart
ophid run ansible-playbook playbook.yml --background --auto-restart

# Scan for vulnerabilities
ophid scan vuln requirements.txt

# Generate SBOM
ophid scan sbom requirements.txt -o sbom.json

# Start reverse proxy
ophid proxy start --domain example.com --target localhost:3000 --tls
```

## Commands

### Runtime Management

```bash
ophid runtime install <version>   # Install Python runtime
ophid runtime list                 # List installed runtimes
ophid runtime remove <version>     # Remove runtime
```

### Tool Management

```bash
# PyPI packages
ophid install <tool>               # Install latest version
ophid install <tool> --version X   # Install specific version

# GitHub repositories
ophid install user/repo            # Install from GitHub (main branch)
ophid install user/repo@v1.0.0     # Install specific tag/branch
ophid install github.com/user/repo # Full GitHub URL

# Git repositories
ophid install https://git.example.com/repo.git

# Local directories
ophid install ./path/to/project    # Relative path
ophid install /absolute/path       # Absolute path

# Common options
ophid list                         # List installed tools
ophid uninstall <tool>             # Uninstall tool
ophid run <tool> [args...]         # Run tool

# Security options
ophid install <tool> --require-scan    # Block if vulnerabilities found
ophid install <tool> --skip-scan       # Skip security scanning
```

### Security Scanning

```bash
ophid scan vuln <file>             # Scan for vulnerabilities
ophid scan license <file>          # Check licenses
ophid scan sbom <file> -o out.json # Generate SBOM
```

### Reverse Proxy

```bash
# Quick start
ophid proxy start --domain example.com --target localhost:3000 --tls

# Simple HTTP proxy
ophid proxy start --listen :8080 --target localhost:3000

# With config file
ophid proxy start --config proxy.toml

# Manage routes
ophid proxy route list
ophid proxy route add --host api.example.com --target localhost:8000
ophid proxy route remove api.example.com

# Server control
ophid proxy status
ophid proxy stop
```

### Flags

- `--background, -b`: Run tool in background
- `--auto-restart`: Auto-restart on failure
- `--force`: Force reinstall
- `--version`: Specify version
- `--output, -o`: Specify output file
- `--format, -f`: Output format (text|json)
- `--allow-copyleft`: Allow copyleft licenses

## Architecture

```
OPHID Components:
├── Runtime Manager    - Python distribution management
├── Security Scanner   - Vulnerability and license scanning (OSV.dev + SBOM)
├── Tool Installer     - Multi-source package management (PyPI/GitHub/Git/Local)
├── Process Supervisor - Process lifecycle and monitoring
└── Reverse Proxy      - HTTP/HTTPS proxy with TLS, load balancing & middleware
```

**Documentation:**
- [Proxy Integration Guide](PROXY_INTEGRATION.md) - Detailed proxy usage patterns
- [Architecture Overview](ARCHITECTURE.md) - System design and components

### Runtime File Structure

```
~/.ophid/
├── runtimes/
│   └── python-3.12.1/          # Isolated Python runtimes
├── tools/
│   ├── manifest.json           # Tool registry
│   └── ansible/
│       └── venv/               # Isolated virtual environment
└── cache/
    └── downloads/              # Downloaded packages
```

## Development

### Build

```bash
make build
```

### Test

```bash
# Run all tests
go test ./... -v

# Run specific package tests
go test ./internal/security/... -v
go test ./internal/tool/... -v
go test ./internal/supervisor/... -v

# Test coverage
go test ./... -cover
```

## Adaptations

### From mcp-osv

- OSV.dev API integration
- Rate limiting patterns
- Input validation
- Vulnerability scanning

### From guvnor

- Health checking patterns
- Process lifecycle management
- Configuration structure
- Reverse proxy integration

### Features Highlights

**Multi-Source Installation:**
- Install from PyPI, GitHub, Git repositories, or local directories
- Automatic security scanning for all installation sources
- SBOM generation and vulnerability detection
- Unified interface across all package sources

**Reverse Proxy:**
- Production-ready HTTP/HTTPS proxy with automatic TLS
- Multiple load balancing strategies
- Middleware support (rate limiting, CORS, logging)
- Zero-downtime configuration reload
- WebSocket proxying support

## Examples

### Install and Run Ansible

```bash
# Install runtime
ophid runtime install 3.12.1

# Install ansible
ophid install ansible

# Run playbook
ophid run ansible-playbook site.yml

# Run in background
ophid run ansible-playbook site.yml --background --auto-restart
```

### Scan Requirements

```bash
# Scan for vulnerabilities
ophid scan vuln requirements.txt

# Check licenses (permissive only)
ophid scan license requirements.txt

# Allow copyleft
ophid scan license requirements.txt --allow-copyleft

# Generate SBOM
ophid scan sbom requirements.txt -o project-sbom.json
```

### Install from GitHub

```bash
# Install from GitHub (like go get)
ophid install ansible/ansible-examples

# Install specific branch or tag
ophid install user/repo@v2.0.0

# Install with security requirements
ophid install user/repo --require-scan

# The installer will:
# 1. Clone the repository
# 2. Detect the ecosystem (Python, Go, Node.js, etc.)
# 3. Scan dependencies for vulnerabilities
# 4. Generate SBOM
# 5. Install in isolated environment
```

### Install from Local Directory

```bash
# Install local project in development mode
ophid install ./my-project

# Security scan is performed automatically
# Creates isolated environment
# Generates SBOM for the project

# Perfect for:
# - Local development
# - Testing before publishing
# - Private internal tools
```

### Reverse Proxy with TLS

```bash
# Start proxy with automatic Let's Encrypt TLS
ophid proxy start \
  --domain example.com \
  --target localhost:3000 \
  --tls

# Multiple backends with load balancing
# (requires config file)
ophid proxy start --config proxy.toml
```

Example `proxy.toml`:
```toml
[general]
listen = ["0.0.0.0:80", "0.0.0.0:443"]

[tls]
enabled = true
acme_email = "admin@example.com"
auto_redirect = true
domains = ["example.com", "api.example.com"]

[[routes]]
host = "example.com"
target = "http://localhost:3000"

[[routes]]
host = "api.example.com"
path = "/v1/*"

[[routes.backends]]
url = "http://10.0.1.10:8000"
weight = 1

[[routes.backends]]
url = "http://10.0.1.11:8000"
weight = 1

[routes.load_balance]
strategy = "least-conn"
```

## License

MIT License

## Credits

- [python-build-standalone](https://github.com/astral-sh/python-build-standalone) by indygreg
- OSV.dev vulnerability database
- Patterns adapted from mcp-osv and guvnor projects
