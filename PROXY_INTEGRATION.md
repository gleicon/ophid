# OPHID Proxy Integration Guide

## Overview

The OPHID reverse proxy can operate in three modes:
1. **Standalone** - Independent proxy for any backend
2. **Supervisor-Integrated** - Auto-fronting supervised processes
3. **Full Platform** - Complete deployment stack

## Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                         OPHID PLATFORM                           │
└──────────────────────────────────────────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
            ┌───────▼────────┐      ┌──────▼─────────┐
            │   CLI Layer    │      │  Config Files  │
            │  (cmd/ophid)   │      │  (TOML/JSON)   │
            └───────┬────────┘      └──────┬─────────┘
                    │                      │
        ┌───────────┼──────────────────────┼────────────┐
        │           │                      │            │
   ┌────▼─────┐ ┌──▼──────┐ ┌──────▼──────┐ ┌────▼────────┐
   │ Runtime  │ │ Security│ │ Tool        │ │ Proxy       │
   │ Manager  │ │ Scanner │ │ Installer   │ │ Server      │
   └────┬─────┘ └──┬──────┘ └───────┬─────┘ └──────┬──────┘
        │          │                │              │
        │      ┌───▼────────────────▼───┐          │
        └──────▶  Unified Security Core  │         │
               │  - OSV.dev API          │         │
               │  - SBOM Generator       │         │
               │  - License Checker      │         │
               └─────────────────────────┘         │
                        │                          │
                        │                          │
        ┌───────────────┴──────────────────────────┴────────┐
        │                                                   │
   ┌────▼──────────┐                            ┌───────────▼────┐
   │  Supervisor   │◀──────────────────────────▶│  HTTP Server   │
   │  Manager      │    Process Registration    │  (Proxy Core)  │
   └────┬──────────┘    & Health Status         └───────┬────────┘
        │                                               │
        │                                               │
   ┌────▼─────────────────┐                    ┌────────▼────────┐
   │  Managed Processes   │                    │  Router &       │
   │  - Tool A (port 8001)│                    │  Load Balancer  │
   │  - Tool B (port 8002)│                    └────────┬────────┘
   │  - Tool C (port 8003)│                             │
   └──────────────────────┘                    ┌────────▼────────┐
                                               │  Middleware     │
                                               │  - Rate Limit   │
                                               │  - CORS         │
                                               │  - Logging      │
                                               └────────┬────────┘
                                                        │
                                               ┌────────▼────────┐
                                               │  TLS Manager    │
                                               │  (Let's Encrypt)│
                                               └────────┬────────┘
                                                        │
                                                   ┌────▼─────┐
                                                   │  Client  │
                                                   └──────────┘
```

## Mode 1: Standalone Proxy

Use the proxy independently to front any HTTP service.

### Use Cases
- Proxy existing services (not installed via OPHID)
- Add TLS to non-HTTPS services
- Load balance multiple backend instances
- Add rate limiting / CORS to existing APIs

### Quick Start

```bash
# Simple HTTP proxy
ophid proxy start --listen :8080 --target localhost:3000

# HTTPS with automatic TLS
ophid proxy start \
  --domain api.example.com \
  --target localhost:3000 \
  --tls

# Multiple backends with load balancing (requires config file)
ophid proxy start --config proxy.toml
```

### Example Config: `proxy.toml`

```toml
[general]
listen = ["0.0.0.0:80", "0.0.0.0:443"]
access_log = "/var/log/ophid/access.log"
error_log = "/var/log/ophid/error.log"

[tls]
enabled = true
acme_provider = "letsencrypt"
acme_email = "admin@example.com"
auto_redirect = true  # HTTP → HTTPS
domains = ["api.example.com", "www.example.com"]
cache_dir = "~/.ophid/certs"

# Single backend route
[[routes]]
host = "www.example.com"
target = "http://localhost:3000"

# Load balanced route
[[routes]]
host = "api.example.com"
path = "/v1/*"

[[routes.backends]]
url = "http://10.0.1.10:8000"
weight = 1

[[routes.backends]]
url = "http://10.0.1.11:8000"
weight = 1

[[routes.backends]]
url = "http://10.0.1.12:8000"
weight = 1

[routes.load_balance]
strategy = "least-conn"  # round-robin | least-conn | ip-hash | weighted
health_check = "/health"
health_interval = "10s"

# Rate limiting middleware
[[routes.middleware]]
type = "ratelimit"
[routes.middleware.options]
rate = 100  # requests per minute
burst = 20

# CORS middleware
[[routes.middleware]]
type = "cors"
[routes.middleware.options]
allow_origins = ["https://app.example.com"]
allow_methods = ["GET", "POST", "PUT", "DELETE"]

# Static files
[[routes]]
host = "static.example.com"
path = "/*"
static = true
static_root = "/var/www/static"
```

### Standalone Architecture

```
Internet
    │
    ▼
┌─────────────────┐
│  TLS Manager    │ ──▶ Let's Encrypt ACME
│  (autocert)     │
└────────┬────────┘
         │
    ┌────▼────┐
    │ Router  │
    └────┬────┘
         │
    ┌────▼────────┐
    │ Middleware  │
    │ Pipeline    │
    └────┬────────┘
         │
    ┌────▼──────────┐
    │ Load Balancer │
    └────┬──────────┘
         │
    ┌────▼─────────────────┐
    │  Backend Servers     │
    │  (External Services) │
    └──────────────────────┘
```

## Mode 2: Supervisor-Integrated

Auto-register supervised processes as proxy backends.

### Use Cases
- Deploy tools with automatic public endpoints
- Zero-config service exposure
- Unified health monitoring
- Process lifecycle + traffic routing

### Quick Start

```bash
# Install tool from any source
ophid install gleicon/my-api

# Run with auto-proxy registration
ophid run my-api --background --auto-restart --register-proxy

# OPHID automatically:
# 1. Starts process under supervisor
# 2. Detects port (8000)
# 3. Creates proxy route: my-api.local → localhost:8000
# 4. Configures health checks
# 5. Routes traffic
```

### Advanced Configuration

```bash
# Custom port and host
ophid run my-api \
  --background \
  --register-proxy \
  --port 9000 \
  --proxy-host my-api.example.com \
  --proxy-tls

# Multiple instances (load balanced automatically)
ophid run my-api --background --register-proxy --port 8001 --name my-api-1
ophid run my-api --background --register-proxy --port 8002 --name my-api-2
ophid run my-api --background --register-proxy --port 8003 --name my-api-3

# OPHID creates a load-balanced route:
# my-api.local → [localhost:8001, localhost:8002, localhost:8003]
```

### Integration Flow

```
ophid run my-api --background --register-proxy
    │
    ▼
┌─────────────────────┐
│  Tool Installer     │
│  - Verify installed │
│  - Get exec path    │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Supervisor         │
│  - Start process    │
│  - Monitor health   │
│  - Auto-restart     │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Port Detection     │
│  - Read stdout/logs │
│  - Detect bind port │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Proxy Registration │
│  - Create route     │
│  - Add backend      │
│  - Config reload    │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Proxy Server       │
│  - Route traffic    │
│  - Health checks    │
└─────────────────────┘
```

### Supervisor ↔ Proxy Communication

The supervisor and proxy communicate via shared state:

```go
// When process starts
supervisor.OnProcessStart(func(proc *Process) {
    if proc.ProxyEnabled {
        // Detect port
        port := detectPort(proc)

        // Register with proxy
        proxy.AddBackend(&Backend{
            Name: proc.Name,
            URL:  fmt.Sprintf("http://localhost:%d", port),
            Health: proc.HealthCheck,
        })

        // Hot reload proxy config
        proxy.Reload()
    }
})

// When process stops
supervisor.OnProcessStop(func(proc *Process) {
    if proc.ProxyEnabled {
        // Remove from proxy
        proxy.RemoveBackend(proc.Name)
        proxy.Reload()
    }
})

// Health monitoring
supervisor.OnHealthChange(func(proc *Process, healthy bool) {
    if proc.ProxyEnabled {
        if healthy {
            proxy.MarkHealthy(proc.Name)
        } else {
            proxy.MarkUnhealthy(proc.Name)
        }
    }
})
```

## Mode 3: Full Platform Stack

Combine all OPHID features for complete deployment automation.

### Architecture

```
┌─────────────────────────────────────────────────────┐
│                  OPHID Full Stack                   │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ┌──────────────────────────────────────────────┐   │
│  │           Internet (HTTPS)                   │   │
│  └────────────────────┬─────────────────────────┘   │
│                       │                             │
│  ┌────────────────────▼─────────────────────────┐   │
│  │         Reverse Proxy + TLS                  │   │
│  │  • Let's Encrypt automatic TLS               │   │
│  │  • Load balancing (4 strategies)             │   │
│  │  • Rate limiting, CORS, logging              │   │
│  │  • WebSocket support                         │   │
│  │  • Authentication and authorization          │   │
│  │  • Customizable headers and response codes   │   │
│  └────────────────────┬─────────────────────────┘   │
│                       │                             │
│  ┌────────────────────▼─────────────────────────┐   │
│  │         Process Supervisor                   │   │
│  │  • Lifecycle management                      │   │
│  │  • Auto-restart on failure                   │   │
│  │  • Health monitoring                         │   │
│  │  • Resource tracking                         │   │
│  └────────────────────┬─────────────────────────┘   │
│                       │                             │
│  ┌────────────────────▼─────────────────────────┐   │
│  │         Isolated Tool Environments           │   │
│  │                                              │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐    │   │
│  │  │ Tool A   │  │ Tool B   │  │ Tool C   │    │   │
│  │  │ (PyPI)   │  │ (GitHub) │  │ (Local)  │    │   │
│  │  │ venv     │  │ venv     │  │ venv     │    │   │
│  │  │ :8001    │  │ :8002    │  │ :8003    │    │   │
│  │  └──────────┘  └──────────┘  └──────────┘    │   │
│  └────────────────────┬─────────────────────────┘  │
│                       │                             │
│  ┌────────────────────▼─────────────────────────┐   │
│  │         Security Scanner                     │   │
│  │  • Pre-install vulnerability scanning        │   │
│  │  • SBOM generation per tool                  │   │
│  │  • License compliance checking               │   │
│  │  • Continuous monitoring                     │   │
│  └──────────────────────────────────────────────┘   │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### Example: Deploy Microservices

```bash
# 1. Install services from different sources
echo "Installing services..."
ophid install user-service                    # PyPI
ophid install myorg/auth-service              # GitHub
ophid install myorg/payment-service@v2.0.0    # GitHub tag
ophid install ./notification-service          # Local dev

# 2. Start all with supervision + proxy
echo "Starting services..."
ophid run user-service \
  --background \
  --auto-restart \
  --register-proxy \
  --proxy-host users.example.com \
  --port 8001

ophid run auth-service \
  --background \
  --auto-restart \
  --register-proxy \
  --proxy-host auth.example.com \
  --port 8002

ophid run payment-service \
  --background \
  --auto-restart \
  --register-proxy \
  --proxy-host payments.example.com \
  --port 8003

ophid run notification-service \
  --background \
  --auto-restart \
  --register-proxy \
  --proxy-host notify.example.com \
  --port 8004

# 3. Enable TLS for all
ophid proxy start --config production.toml
```

### Production Config: `production.toml`

```toml
[general]
listen = ["0.0.0.0:80", "0.0.0.0:443"]

[tls]
enabled = true
acme_email = "ops@example.com"
auto_redirect = true
domains = [
    "users.example.com",
    "auth.example.com",
    "payments.example.com",
    "notify.example.com"
]

# User service - load balanced
[[routes]]
host = "users.example.com"

[[routes.backends]]
url = "http://localhost:8001"  # Managed by supervisor

[routes.load_balance]
strategy = "round-robin"
health_check = "/health"

[[routes.middleware]]
type = "ratelimit"
[routes.middleware.options]
rate = 1000
burst = 50

# Auth service - strict rate limiting
[[routes]]
host = "auth.example.com"
[[routes.backends]]
url = "http://localhost:8002"

[[routes.middleware]]
type = "ratelimit"
[routes.middleware.options]
rate = 100  # Stricter for auth
burst = 10

# Payment service - extra security
[[routes]]
host = "payments.example.com"
[[routes.backends]]
url = "http://localhost:8003"

[[routes.middleware]]
type = "ratelimit"
[routes.middleware.options]
rate = 200
burst = 20

# Notification service
[[routes]]
host = "notify.example.com"
[[routes.backends]]
url = "http://localhost:8004"
```

## Management Commands

### Check Status

```bash
# List all installed tools
ophid list
# Output:
# user-service@1.2.0        (PyPI)
# auth-service@2.0.0        (GitHub: myorg/auth-service)
# payment-service@2.0.0     (GitHub: myorg/payment-service@v2.0.0)
# notification-service@dev  (Local: ./notification-service)

# Show supervised processes
ophid supervisor status
# Output:
# PID    NAME                    STATUS   PORT   UPTIME
# 1234   user-service            running  8001   2h 15m
# 1235   auth-service            running  8002   2h 14m
# 1236   payment-service         running  8003   2h 13m
# 1237   notification-service    running  8004   2h 12m

# Show proxy routes
ophid proxy status
# Output:
# ROUTE                      BACKEND(S)              HEALTH   REQUESTS
# users.example.com          localhost:8001          healthy  12.5k
# auth.example.com           localhost:8002          healthy  8.2k
# payments.example.com       localhost:8003          healthy  3.1k
# notify.example.com         localhost:8004          healthy  15.8k
```

### Update Service

```bash
# Update from source
ophid install payment-service@v2.1.0 --force

# Restart with zero downtime (if load balanced)
ophid supervisor restart payment-service

# Or blue-green deployment
ophid run payment-service --background --port 9003 --name payment-v2
ophid proxy route update payments.example.com --add-backend localhost:9003
# Shift traffic gradually...
ophid proxy route update payments.example.com --remove-backend localhost:8003
ophid supervisor stop payment-service
```

### Security Monitoring

```bash
# Scan all installed tools
ophid scan all

# Check specific tool
ophid scan vuln user-service

# View security report
ophid info user-service
# Output:
# user-service@1.2.0
# Source: PyPI
# Security Status: ⚠ Warning
#   - 2 vulnerabilities (1 medium, 1 low)
#   - Last scanned: 30 minutes ago
#   - SBOM: ~/.ophid/tools/user-service/sbom.json
# Recommendation: Update to 1.2.1
```

## When to Use Each Mode

| Mode | Use Case | Command Pattern |
|------|----------|-----------------|
| **Standalone** | Proxy existing services | `ophid proxy start --target <url>` |
| **Standalone + TLS** | Add HTTPS to any service | `ophid proxy start --domain <host> --target <url> --tls` |
| **Supervised** | Deploy one OPHID-managed tool | `ophid run <tool> --register-proxy` |
| **Full Stack** | Deploy multiple services | Multiple `ophid install` + `run --register-proxy` |
| **Development** | Local tool testing | `ophid install ./project && ophid run project --register-proxy` |
| **Production** | Enterprise deployment | Full Stack + config file |

## Key Benefits

  * **Decoupled yet Integrated**: Proxy works standalone but integrates seamlessly with supervisor
  * **Security-First**: All tools scanned before installation, regardless of source
  * **Multi-Source**: Install from PyPI, GitHub, Git, or local directories
  * **Zero-Config**: `--register-proxy` flag does all the wiring
  * **Production-Ready**: TLS, load balancing, health checks, rate limiting built-in
  * **Operations-Friendly**: One binary, one command pattern, unified management
