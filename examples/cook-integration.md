# Cook Integration with OPHID

## Vision: Cook as the Primary Use Case

Cook is the perfect tool to showcase OPHID's value:
- Written in Python (needs runtime management)
- Operations-focused (OPHID's target audience)
- Has complex dependencies (benefits from isolation)
- Needs distribution (OPHID solves this)

## Current State (Manual Installation)

```bash
# User needs Python knowledge
git clone https://github.com/gleicon/cook-py
cd cook-py
python3 -m venv .venv
source .venv/bin/activate
pip install -e ".[all]"

# Finally use it
cook plan server.py
cook apply server.py
```

## With OPHID (Simplified)

```bash
# One-time setup
curl -sSL https://ophid.sh/install | sh

# Install cook
ophid install cook

# Use cook
ophid run cook plan server.py
ophid run cook apply server.py
```

## Cook-Specific Features

### 1. Cook Server Mode with Supervision

```bash
# Start cook in server mode with auto-restart
ophid run cook server --port 8080 --background --auto-restart

# Check status
ophid supervisor status cook

# View logs
ophid supervisor logs cook

# Stop server
ophid supervisor stop cook
```

### 2. Cook State Management

Cook has drift detection. OPHID could enhance this:

```bash
# Run cook drift check as supervised cron job
ophid supervisor add cook-drift \
  --command "cook check-drift --fix" \
  --schedule "*/30 * * * *" \
  --health-check "http://localhost:8080/health"
```

### 3. Multi-Environment Cook Deployments

```yaml
# ophid-cook.yml
tools:
  cook:
    version: "0.2.0"

environments:
  dev:
    services:
      cook-server:
        command: cook server --port 8080
        env:
          COOK_ENV: development
        health_check:
          type: http
          endpoint: http://localhost:8080/health

  prod:
    services:
      cook-server:
        command: cook server --port 8080
        env:
          COOK_ENV: production
        auto_restart: true
        max_retries: 3
```

```bash
ophid env use dev
ophid up

ophid env use prod
ophid up
```

### 4. Cook + SBOM Integration

```bash
# Install cook
ophid install cook

# Generate SBOM for cook's dependencies
ophid scan sbom ~/.ophid/tools/cook/venv/lib/python*/site-packages -o cook-sbom.json

# Scan for vulnerabilities
ophid scan vuln cook-sbom.json

# Before deploying to production
ophid security-gate cook
  → Scans dependencies
  → Checks licenses
  → Validates signatures
  → Generates audit report
```

## Cook Distribution Scenarios

### Scenario 1: Standalone Bundle (Recommended)

```bash
# Create portable bundle
ophid bundle cook -o cook-standalone.tar.gz
  → Includes: Python runtime + Cook + all dependencies
  → Size: ~60MB compressed
  → Works offline

# On target machine (no OPHID needed)
tar xzf cook-standalone.tar.gz
./cook-standalone/cook plan server.py
```

### Scenario 2: Container Image

```bash
# Create container image
ophid docker build cook

# Result: Docker image with cook
docker run ophid/cook:0.2.0 plan server.py
docker run ophid/cook:0.2.0 apply server.py --host example.com
```

### Scenario 3: System Package

```bash
# Create native package
ophid package cook --deb
  → cook_0.2.0_amd64.deb
  → Installs to /opt/ophid/cook
  → Includes systemd service

sudo dpkg -i cook_0.2.0_amd64.deb
sudo systemctl start cook-server
```

## Cook MCP Server Integration

Cook has an MCP server for AI integration. OPHID could manage it:

```bash
# Install cook with MCP server
ophid install cook --extras mcp

# Start MCP server supervised
ophid run cook mcp-server --background --auto-restart

# Configure in Claude Desktop
# OPHID provides MCP endpoint automatically
```

## Cook Recording Mode with OPHID

```bash
# Start recording session (supervised)
ophid run cook record start --background --session prod-setup

# Do manual server configuration...
# Cook records all commands

# Generate configuration from recording
ophid run cook record generate session-prod-setup.json > server.py

# Review and apply
ophid run cook plan server.py
ophid run cook apply server.py
```

## Real-World Cook Deployment with OPHID

### Initial Setup

```bash
# Install OPHID
curl -sSL https://ophid.sh/install | sh

# Install Python runtime
ophid runtime install 3.12.1

# Install cook
ophid install cook
```

### Deploy Application Infrastructure

```python
# myapp-infra.py
from cook import File, Package, Service, Repository

# Update system
Repository("apt-update", action="update")

# Install packages
Package("nginx")
Package("postgresql")
Package("redis-server")

# Configure nginx
File("/etc/nginx/sites-available/myapp",
     template="nginx.conf.j2",
     vars={"domain": "myapp.com", "port": 8000})

# Enable services
Service("nginx", running=True, enabled=True)
Service("postgresql", running=True, enabled=True)
Service("redis", running=True, enabled=True)
```

```bash
# Preview changes
ophid run cook plan myapp-infra.py --host prod-1.example.com

# Apply changes
ophid run cook apply myapp-infra.py --host prod-1.example.com --sudo

# Run drift detection periodically
ophid supervisor add cook-drift \
  --command "cook check-drift myapp-infra.py --fix --host prod-1.example.com" \
  --schedule "*/30 * * * *"
```

### Multi-Server Deployment

```bash
# Deploy to multiple servers
for host in prod-{1..5}.example.com; do
  ophid run cook apply myapp-infra.py --host $host --sudo &
done
wait

# Verify all servers
for host in prod-{1..5}.example.com; do
  ophid run cook state list --host $host
done
```

## Cook Development Workflow with OPHID

### Developer Setup

```bash
# Clone cook repo
git clone https://github.com/gleicon/cook-py
cd cook-py

# Install in editable mode
ophid install . --editable

# Run tests
ophid run pytest tests/

# Make changes and test immediately
ophid run cook plan examples/simple.py
```

### CI/CD Integration

```yaml
# .github/workflows/test.yml
name: Test Cook with OPHID

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Install OPHID
        run: curl -sSL https://ophid.sh/install | sh

      - name: Install Cook
        run: ophid install .

      - name: Run tests
        run: ophid run pytest tests/

      - name: Security scan
        run: |
          ophid scan vuln requirements.txt
          ophid scan sbom . -o sbom.json

      - name: Upload SBOM
        uses: actions/upload-artifact@v3
        with:
          name: sbom
          path: sbom.json
```

