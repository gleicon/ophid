package proxy

import (
	"net/http"
	"net/url"
	"sync"
	"time"
)

// LoadBalanceStrategy represents a load balancing strategy
type LoadBalanceStrategy string

const (
	StrategyRoundRobin  LoadBalanceStrategy = "round-robin"
	StrategyLeastConn   LoadBalanceStrategy = "least-conn"
	StrategyIPHash      LoadBalanceStrategy = "ip-hash"
	StrategyWeighted    LoadBalanceStrategy = "weighted"
)

// HealthStatus represents backend health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// Config is the main proxy configuration
type Config struct {
	General GeneralConfig `json:"general"`
	TLS     TLSConfig     `json:"tls"`
	Routes  []Route       `json:"routes"`
}

// GeneralConfig contains general proxy settings
type GeneralConfig struct {
	Listen    []string `json:"listen"`     // Listen addresses, e.g., ["0.0.0.0:80", "0.0.0.0:443"]
	AccessLog string   `json:"access_log"` // Access log path
	ErrorLog  string   `json:"error_log"`  // Error log path
}

// TLSConfig contains TLS/ACME configuration
type TLSConfig struct {
	Enabled      bool     `json:"enabled"`
	AutoRedirect bool     `json:"auto_redirect"` // HTTP -> HTTPS redirect
	ACMEProvider string   `json:"acme_provider"` // "letsencrypt", "zerossl"
	ACMEEmail    string   `json:"acme_email"`
	CacheDir     string   `json:"cache_dir"`
	Domains      []string `json:"domains"`
}

// Route represents a routing rule
type Route struct {
	// Matching criteria
	Host   string `json:"host"`   // Host pattern (e.g., "example.com", "*.example.com")
	Path   string `json:"path"`   // Path pattern (e.g., "/api/*")
	Method string `json:"method"` // HTTP method (e.g., "GET", "*")

	// Target configuration
	Target   string     `json:"target,omitempty"`   // Single backend URL
	Backends []*Backend `json:"backends,omitempty"` // Multiple backends for load balancing

	// Options
	WebSocket      bool              `json:"websocket,omitempty"`
	StripPrefix    string            `json:"strip_prefix,omitempty"`
	AddHeaders     map[string]string `json:"add_headers,omitempty"`
	LoadBalance    LoadBalanceConfig `json:"load_balance,omitempty"`
	MiddlewareList []MiddlewareConfig `json:"middleware,omitempty"`

	// Static file serving
	Static     bool   `json:"static,omitempty"`
	StaticRoot string `json:"static_root,omitempty"`
}

// Backend represents a backend server
type Backend struct {
	Name    string  `json:"name"`
	URL     *url.URL `json:"-"` // Parsed URL
	URLStr  string  `json:"url"` // String representation for JSON
	Weight  int     `json:"weight,omitempty"`
	Health  *Health `json:"-"` // Health status (runtime only)
}

// Health tracks backend health
type Health struct {
	Status      HealthStatus
	Connections int32
	FailCount   int32
	LastCheck   time.Time
	mu          sync.RWMutex
}

// LoadBalanceConfig configures load balancing
type LoadBalanceConfig struct {
	Strategy       LoadBalanceStrategy `json:"strategy"`
	HealthCheck    string              `json:"health_check,omitempty"`    // Health check path
	HealthInterval string              `json:"health_interval,omitempty"` // Check interval (e.g., "10s")
}

// MiddlewareConfig configures middleware
type MiddlewareConfig struct {
	Type string                 `json:"type"` // "ratelimit", "cors", "auth"
	Options map[string]interface{} `json:"options,omitempty"`
}

// Middleware is a function that wraps an http.Handler
type Middleware func(http.Handler) http.Handler

// GetHealthStatus safely gets health status
func (h *Health) GetStatus() HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.Status
}

// SetStatus safely sets health status
func (h *Health) SetStatus(status HealthStatus) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Status = status
	h.LastCheck = time.Now()
}

// IncrementConnections increments connection count
func (h *Health) IncrementConnections() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Connections++
}

// DecrementConnections decrements connection count
func (h *Health) DecrementConnections() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.Connections > 0 {
		h.Connections--
	}
}

// GetConnections gets current connection count
func (h *Health) GetConnections() int32 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.Connections
}
