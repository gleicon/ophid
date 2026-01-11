# OPHID Reverse Proxy Design

**Component:** HTTP/HTTPS Reverse Proxy
**Status:** Design Phase
**Priority:** High (Phase 5)

## Overview

The Reverse Proxy provides production-grade HTTP/HTTPS proxying with automatic TLS, load balancing, and WebSocket support - enabling OPHID to serve as a complete deployment platform.

**Inspired by:** nginx, caddy, traefik, gunicorn

## Goals

1. **Automatic TLS** via Let's Encrypt (ACME protocol)
2. **Load balancing** with multiple strategies
3. **WebSocket support** for real-time apps
4. **Static file serving** for frontend assets
5. **Rate limiting** and DDoS protection
6. **Access logging** and metrics
7. **Zero-config defaults** with power-user options
8. **Hot reload** without downtime

## Architecture

### High-Level Design

```
┌────────────────────────────────────────────────┐
│  Reverse Proxy                                 │
├────────────────────────────────────────────────┤
│                                                │
│  ┌──────────────┐  ┌──────────────────────┐   │
│  │ HTTP Server  │  │ TLS Manager          │   │
│  │ - HTTP/1.1   │  │ - Let's Encrypt      │   │
│  │ - HTTP/2     │  │ - Auto renewal       │   │
│  │ - HTTP/3(fut)│  │ - Cert storage       │   │
│  └──────┬───────┘  └─────────┬────────────┘   │
│         │                    │                 │
│  ┌──────▼───────┐  ┌─────────▼────────────┐   │
│  │ Router       │  │ Load Balancer        │   │
│  │ - Host match │  │ - Round robin        │   │
│  │ - Path match │  │ - Least conn         │   │
│  │ - Method     │  │ - IP hash            │   │
│  └──────┬───────┘  │ - Health checks      │   │
│         │          └─────────┬────────────┘   │
│  ┌──────▼───────┐  ┌─────────▼────────────┐   │
│  │ Middleware   │  │ Backend Manager      │   │
│  │ - Rate limit │  │ - Connection pool    │   │
│  │ - Auth       │  │ - Retry logic        │   │
│  │ - CORS       │  │ - Circuit breaker    │   │
│  │ - Logging    │  │ - Failover           │   │
│  └──────────────┘  └──────────────────────┘   │
│                                                │
└────────────────────────────────────────────────┘
```

### Request Flow

```
Client Request
     ↓
TLS Termination (if HTTPS)
     ↓
Router (match host/path)
     ↓
Middleware Pipeline
  - Rate limiting
  - Authentication
  - CORS headers
  - Request logging
     ↓
Load Balancer (select backend)
     ↓
Backend Connection
  - Connection pool
  - Health check
  - Retry on failure
     ↓
Proxy to Backend
  - HTTP/WebSocket
  - Stream response
     ↓
Middleware Pipeline (response)
  - Response headers
  - Compression
  - Caching
     ↓
Send to Client
```

## Core Components

### HTTP Server

```go
// internal/proxy/server.go
package proxy

import (
    "net/http"
    "golang.org/x/crypto/acme/autocert"
)

type Server struct {
    router     *Router
    tlsManager *TLSManager
    config     *Config

    httpServer  *http.Server
    httpsServer *http.Server
}

func NewServer(config *Config) *Server {
    return &Server{
        router:     NewRouter(),
        tlsManager: NewTLSManager(config.TLS),
        config:     config,
    }
}

func (s *Server) Start() error {
    // Start HTTP server (port 80)
    go s.startHTTP()

    // Start HTTPS server (port 443)
    if s.config.TLS.Enabled {
        return s.startHTTPS()
    }

    return nil
}

func (s *Server) startHTTP() error {
    s.httpServer = &http.Server{
        Addr:    ":80",
        Handler: s.router,
    }

    // Redirect HTTP -> HTTPS if TLS enabled
    if s.config.TLS.AutoRedirect {
        s.httpServer.Handler = http.HandlerFunc(s.redirectToHTTPS)
    }

    return s.httpServer.ListenAndServe()
}

func (s *Server) startHTTPS() error {
    s.httpsServer = &http.Server{
        Addr:      ":443",
        Handler:   s.router,
        TLSConfig: s.tlsManager.TLSConfig(),
    }

    return s.httpsServer.ListenAndServeTLS("", "")
}

func (s *Server) redirectToHTTPS(w http.ResponseWriter, r *http.Request) {
    target := "https://" + r.Host + r.RequestURI
    http.Redirect(w, r, target, http.StatusMovedPermanently)
}

func (s *Server) Reload() error {
    // Hot reload configuration without downtime
    // 1. Load new config
    // 2. Create new router
    // 3. Swap atomically
    return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
    // Graceful shutdown
    errChan := make(chan error, 2)

    go func() {
        errChan <- s.httpServer.Shutdown(ctx)
    }()

    go func() {
        if s.httpsServer != nil {
            errChan <- s.httpsServer.Shutdown(ctx)
        } else {
            errChan <- nil
        }
    }()

    err1 := <-errChan
    err2 := <-errChan

    if err1 != nil {
        return err1
    }
    return err2
}
```

### TLS Manager (Let's Encrypt)

```go
// internal/proxy/tls.go
package proxy

import (
    "crypto/tls"
    "golang.org/x/crypto/acme/autocert"
)

type TLSManager struct {
    autocertManager *autocert.Manager
    config          *TLSConfig
}

type TLSConfig struct {
    Enabled        bool
    AutoRedirect   bool
    ACMEProvider   string // "letsencrypt", "zerossl"
    ACMEEmail      string
    CacheDir       string // ~/.ophid/certs
    Domains        []string
}

func NewTLSManager(config *TLSConfig) *TLSManager {
    manager := &autocert.Manager{
        Prompt:      autocert.AcceptTOS,
        Email:       config.ACMEEmail,
        HostPolicy:  autocert.HostWhitelist(config.Domains...),
        Cache:       autocert.DirCache(config.CacheDir),
    }

    // Use production or staging server
    if config.ACMEProvider == "letsencrypt" {
        // Default: production
        // For testing: manager.Client = &acme.Client{DirectoryURL: letsencrypt.StagingURL}
    }

    return &TLSManager{
        autocertManager: manager,
        config:          config,
    }
}

func (tm *TLSManager) TLSConfig() *tls.Config {
    return &tls.Config{
        GetCertificate: tm.autocertManager.GetCertificate,
        NextProtos:     []string{"h2", "http/1.1"}, // HTTP/2 support
        MinVersion:     tls.VersionTLS12,
    }
}

func (tm *TLSManager) HTTPHandler() http.Handler {
    // ACME challenge handler (for HTTP-01)
    return tm.autocertManager.HTTPHandler(nil)
}
```

### Router

```go
// internal/proxy/router.go
package proxy

type Router struct {
    routes []*Route
    mu     sync.RWMutex
}

type Route struct {
    // Matching
    Host      string            // "example.com", "*.example.com"
    Path      string            // "/api/*", exact match or wildcard
    Method    string            // "GET", "POST", "*"

    // Target
    Backend   string            // "myapp", "http://localhost:3000"
    Backends  []*Backend        // For load balancing

    // Options
    Middleware  []Middleware
    WebSocket   bool
    StripPrefix string
    AddHeaders  map[string]string
}

type Backend struct {
    Name    string
    URL     *url.URL
    Weight  int       // For weighted load balancing
    Health  *HealthStatus
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    // 1. Find matching route
    route := r.match(req)
    if route == nil {
        http.NotFound(w, req)
        return
    }

    // 2. Run middleware pipeline
    handler := r.buildHandler(route)
    for i := len(route.Middleware) - 1; i >= 0; i-- {
        handler = route.Middleware[i](handler)
    }

    // 3. Execute handler
    handler.ServeHTTP(w, req)
}

func (r *Router) match(req *http.Request) *Route {
    r.mu.RLock()
    defer r.mu.RUnlock()

    for _, route := range r.routes {
        if r.matchRoute(route, req) {
            return route
        }
    }

    return nil
}

func (r *Router) matchRoute(route *Route, req *http.Request) bool {
    // Match host
    if route.Host != "" && !matchHost(route.Host, req.Host) {
        return false
    }

    // Match path
    if route.Path != "" && !matchPath(route.Path, req.URL.Path) {
        return false
    }

    // Match method
    if route.Method != "*" && route.Method != req.Method {
        return false
    }

    return true
}

func matchHost(pattern, host string) bool {
    // Support wildcards: *.example.com
    if pattern == "*" {
        return true
    }

    if strings.HasPrefix(pattern, "*.") {
        suffix := strings.TrimPrefix(pattern, "*")
        return strings.HasSuffix(host, suffix)
    }

    return pattern == host
}

func matchPath(pattern, path string) bool {
    // Exact match
    if pattern == path {
        return true
    }

    // Wildcard: /api/*
    if strings.HasSuffix(pattern, "/*") {
        prefix := strings.TrimSuffix(pattern, "/*")
        return strings.HasPrefix(path, prefix)
    }

    return false
}

func (r *Router) buildHandler(route *Route) http.Handler {
    if route.WebSocket {
        return &WebSocketProxy{route: route}
    } else {
        return &HTTPProxy{route: route}
    }
}
```

### Load Balancer

```go
// internal/proxy/loadbalancer.go
package proxy

type LoadBalanceStrategy string

const (
    StrategyRoundRobin  LoadBalanceStrategy = "round-robin"
    StrategyLeastConn   LoadBalanceStrategy = "least-conn"
    StrategyIPHash      LoadBalanceStrategy = "ip-hash"
    StrategyWeighted    LoadBalanceStrategy = "weighted"
)

type LoadBalancer struct {
    backends  []*Backend
    strategy  LoadBalanceStrategy
    current   int32 // For round-robin
    mu        sync.RWMutex
}

func NewLoadBalancer(strategy LoadBalanceStrategy) *LoadBalancer {
    return &LoadBalancer{
        backends: []*Backend{},
        strategy: strategy,
    }
}

func (lb *LoadBalancer) SelectBackend(req *http.Request) *Backend {
    lb.mu.RLock()
    defer lb.mu.RUnlock()

    // Filter healthy backends
    healthy := lb.healthyBackends()
    if len(healthy) == 0 {
        return nil
    }

    switch lb.strategy {
    case StrategyRoundRobin:
        return lb.roundRobin(healthy)
    case StrategyLeastConn:
        return lb.leastConn(healthy)
    case StrategyIPHash:
        return lb.ipHash(req, healthy)
    case StrategyWeighted:
        return lb.weighted(healthy)
    default:
        return healthy[0]
    }
}

func (lb *LoadBalancer) roundRobin(backends []*Backend) *Backend {
    idx := atomic.AddInt32(&lb.current, 1)
    return backends[int(idx)%len(backends)]
}

func (lb *LoadBalancer) leastConn(backends []*Backend) *Backend {
    var selected *Backend
    minConn := int32(1<<31 - 1)

    for _, backend := range backends {
        if backend.Health.Connections < minConn {
            minConn = backend.Health.Connections
            selected = backend
        }
    }

    return selected
}

func (lb *LoadBalancer) ipHash(req *http.Request, backends []*Backend) *Backend {
    // Extract client IP
    clientIP := extractIP(req)

    // Hash IP to backend
    hash := fnv.New32a()
    hash.Write([]byte(clientIP))
    idx := hash.Sum32() % uint32(len(backends))

    return backends[idx]
}

func (lb *LoadBalancer) weighted(backends []*Backend) *Backend {
    // Weighted random selection
    totalWeight := 0
    for _, b := range backends {
        totalWeight += b.Weight
    }

    random := rand.Intn(totalWeight)
    for _, b := range backends {
        random -= b.Weight
        if random < 0 {
            return b
        }
    }

    return backends[0]
}

func (lb *LoadBalancer) healthyBackends() []*Backend {
    healthy := []*Backend{}
    for _, backend := range lb.backends {
        if backend.Health.Status == HealthStatusHealthy {
            healthy = append(healthy, backend)
        }
    }
    return healthy
}
```

### HTTP Proxy Handler

```go
// internal/proxy/http_proxy.go
package proxy

import (
    "net/http/httputil"
)

type HTTPProxy struct {
    route      *Route
    loadBalancer *LoadBalancer
    transport  *http.Transport
}

func (hp *HTTPProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    // 1. Select backend
    backend := hp.loadBalancer.SelectBackend(req)
    if backend == nil {
        http.Error(w, "No healthy backends", http.StatusServiceUnavailable)
        return
    }

    // 2. Modify request
    hp.prepareRequest(req, backend)

    // 3. Create reverse proxy
    proxy := &httputil.ReverseProxy{
        Director: func(r *http.Request) {
            r.URL.Scheme = backend.URL.Scheme
            r.URL.Host = backend.URL.Host
            r.Host = backend.URL.Host

            // Strip prefix if configured
            if hp.route.StripPrefix != "" {
                r.URL.Path = strings.TrimPrefix(r.URL.Path, hp.route.StripPrefix)
            }

            // Add custom headers
            for k, v := range hp.route.AddHeaders {
                r.Header.Set(k, v)
            }

            // Preserve original headers
            r.Header.Set("X-Forwarded-For", req.RemoteAddr)
            r.Header.Set("X-Forwarded-Proto", req.URL.Scheme)
            r.Header.Set("X-Forwarded-Host", req.Host)
        },
        Transport: hp.transport,
        ErrorHandler: hp.errorHandler,
    }

    // 4. Proxy request
    proxy.ServeHTTP(w, req)
}

func (hp *HTTPProxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
    // Log error
    log.Printf("Proxy error: %v", err)

    // Retry with another backend?
    // Circuit breaker logic?

    http.Error(w, "Bad Gateway", http.StatusBadGateway)
}
```

### WebSocket Proxy

```go
// internal/proxy/websocket.go
package proxy

import (
    "github.com/gorilla/websocket"
)

type WebSocketProxy struct {
    route    *Route
    upgrader websocket.Upgrader
}

func (wsp *WebSocketProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    // 1. Select backend
    backend := wsp.route.Backends[0] // TODO: Load balancing

    // 2. Upgrade client connection
    clientConn, err := wsp.upgrader.Upgrade(w, req, nil)
    if err != nil {
        http.Error(w, "WebSocket upgrade failed", http.StatusBadGateway)
        return
    }
    defer clientConn.Close()

    // 3. Connect to backend
    backendURL := *backend.URL
    backendURL.Scheme = "ws" // or "wss"
    backendConn, _, err := websocket.DefaultDialer.Dial(backendURL.String(), nil)
    if err != nil {
        log.Printf("Backend connection failed: %v", err)
        return
    }
    defer backendConn.Close()

    // 4. Proxy messages bidirectionally
    errChan := make(chan error, 2)

    // Client -> Backend
    go func() {
        for {
            msgType, msg, err := clientConn.ReadMessage()
            if err != nil {
                errChan <- err
                return
            }

            err = backendConn.WriteMessage(msgType, msg)
            if err != nil {
                errChan <- err
                return
            }
        }
    }()

    // Backend -> Client
    go func() {
        for {
            msgType, msg, err := backendConn.ReadMessage()
            if err != nil {
                errChan <- err
                return
            }

            err = clientConn.WriteMessage(msgType, msg)
            if err != nil {
                errChan <- err
                return
            }
        }
    }()

    // Wait for error or close
    <-errChan
}
```

## Middleware

### Rate Limiting

```go
// internal/proxy/middleware/ratelimit.go
package middleware

import (
    "golang.org/x/time/rate"
)

type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.Mutex
    rate     rate.Limit // requests per second
    burst    int
}

func NewRateLimiter(rps int, burst int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        rate:     rate.Limit(rps),
        burst:    burst,
    }
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Get limiter for IP
        ip := extractIP(r)
        limiter := rl.getLimiter(ip)

        if !limiter.Allow() {
            http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
            return
        }

        next.ServeHTTP(w, r)
    })
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    limiter, exists := rl.limiters[ip]
    if !exists {
        limiter = rate.NewLimiter(rl.rate, rl.burst)
        rl.limiters[ip] = limiter
    }

    return limiter
}
```

### Access Logging

```go
// internal/proxy/middleware/logging.go
package middleware

type LoggingMiddleware struct {
    logger *log.Logger
}

func (lm *LoggingMiddleware) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Wrap response writer to capture status
        wrapped := &responseWriter{ResponseWriter: w}

        next.ServeHTTP(wrapped, r)

        // Log request
        lm.logger.Printf("%s %s %d %s %s",
            r.Method,
            r.RequestURI,
            wrapped.status,
            time.Since(start),
            r.RemoteAddr,
        )
    })
}

type responseWriter struct {
    http.ResponseWriter
    status int
}

func (rw *responseWriter) WriteHeader(status int) {
    rw.status = status
    rw.ResponseWriter.WriteHeader(status)
}
```

### CORS

```go
// internal/proxy/middleware/cors.go
package middleware

type CORSMiddleware struct {
    allowOrigins []string
    allowMethods []string
    allowHeaders []string
}

func (cm *CORSMiddleware) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Set CORS headers
        origin := r.Header.Get("Origin")
        if cm.isAllowedOrigin(origin) {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Set("Access-Control-Allow-Methods", strings.Join(cm.allowMethods, ", "))
            w.Header().Set("Access-Control-Allow-Headers", strings.Join(cm.allowHeaders, ", "))
        }

        // Handle preflight
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

## Configuration

### Proxy Configuration

```toml
# ~/.ophid/proxy/config.toml
[general]
listen = ["0.0.0.0:80", "0.0.0.0:443"]
access_log = "/var/log/ophid/proxy-access.log"
error_log = "/var/log/ophid/proxy-error.log"

[tls]
enabled = true
acme_provider = "letsencrypt"  # or "zerossl"
acme_email = "admin@example.com"
auto_redirect = true  # HTTP -> HTTPS
cache_dir = "~/.ophid/certs"

# Simple route
[[routes]]
host = "example.com"
target = "http://localhost:3000"
websocket = true

# Advanced route with load balancing
[[routes]]
host = "api.example.com"
path = "/v1/*"
strip_prefix = "/v1"

[[routes.backends]]
url = "http://10.0.1.10:8000"
weight = 1

[[routes.backends]]
url = "http://10.0.1.11:8000"
weight = 1

[routes.loadbalance]
strategy = "least-conn"
health_check = "/health"
health_interval = "10s"

# Middleware
[[routes.middleware]]
type = "ratelimit"
rate = "100/minute"

[[routes.middleware]]
type = "cors"
allow_origins = ["https://app.example.com"]

# Static files
[[routes]]
host = "static.example.com"
path = "/assets/*"
type = "static"
root = "/var/www/static"
```

## CLI Commands

```bash
# Start proxy
ophid proxy start
ophid proxy start --config proxy.toml
ophid proxy start --listen :8080

# Quick setup
ophid proxy start \
  --domain example.com \
  --target localhost:3000 \
  --tls auto

# Manage routes
ophid proxy route add \
  --host api.example.com \
  --target localhost:8000 \
  --path "/v1/*"

ophid proxy route list
ophid proxy route remove api.example.com

# Status
ophid proxy status
ophid proxy logs --follow

# Reload
ophid proxy reload
ophid proxy stop
```

## Health Checks

```go
// internal/proxy/health.go
package proxy

type HealthChecker struct {
    backends map[string]*Backend
    interval time.Duration
}

func (hc *HealthChecker) Start() {
    ticker := time.NewTicker(hc.interval)
    defer ticker.Stop()

    for range ticker.C {
        for _, backend := range hc.backends {
            hc.check(backend)
        }
    }
}

func (hc *HealthChecker) check(backend *Backend) {
    // HTTP health check
    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Get(backend.HealthCheckURL)

    if err != nil || resp.StatusCode >= 400 {
        backend.Health.Status = HealthStatusUnhealthy
        backend.Health.FailCount++
    } else {
        backend.Health.Status = HealthStatusHealthy
        backend.Health.FailCount = 0
    }
}
```

## Integration

### With Supervisor

```bash
# Auto-register supervised process
ophid supervise start myapp --port 3000 --register-proxy

# Creates route:
# myapp.local -> localhost:3000
```

### With Security Scanner

```bash
# Only proxy if security scan passes
ophid proxy route add \
  --host api.example.com \
  --target myapp:8000 \
  --require-scan
```

## Performance Considerations

### Connection Pooling

```go
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
}
```

### HTTP/2 Support

```go
tlsConfig := &tls.Config{
    NextProtos: []string{"h2", "http/1.1"},
}
```

## Open Questions

1. **Static Files:**
   - Built-in static server or delegate to backend?
   - Caching strategy?

2. **Caching:**
   - HTTP cache (like Varnish)?
   - CDN integration?

3. **Authentication:**
   - Built-in auth middleware?
   - OAuth/OIDC support?

4. **Observability:**
   - Metrics (Prometheus)?
   - Distributed tracing (OpenTelemetry)?

5. **Advanced Features:**
   - Request/response transformation?
   - gRPC proxying?
   - TCP/UDP proxying?

## Next Steps

1. PASS Design complete
2. Pending Implement HTTP server
3. Pending Implement TLS manager (Let's Encrypt)
4. Pending Implement router and load balancer
5. Pending Implement WebSocket proxy
6. Pending Add middleware (rate limit, CORS, logging)
7. Pending Write tests
8. Pending Documentation

**Related:** [PLATFORM_VISION.md](../PLATFORM_VISION.md)
