package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// HTTPProxy handles HTTP reverse proxying
type HTTPProxy struct {
	route        *Route
	loadBalancer *LoadBalancer
	transport    *http.Transport
}

// NewHTTPProxy creates a new HTTP proxy for a route
func NewHTTPProxy(route *Route) *HTTPProxy {
	// Create load balancer if multiple backends
	var lb *LoadBalancer
	if len(route.Backends) > 0 {
		strategy := StrategyRoundRobin
		if route.LoadBalance.Strategy != "" {
			strategy = route.LoadBalance.Strategy
		}
		lb = NewLoadBalancer(strategy, route.Backends)
	} else if route.Target != "" {
		// Single backend - create a simple load balancer with one backend
		targetURL, err := url.Parse(route.Target)
		if err != nil {
			log.Printf("Error parsing target URL %s: %v", route.Target, err)
			return nil
		}

		backend := &Backend{
			Name:   "default",
			URL:    targetURL,
			URLStr: route.Target,
			Weight: 1,
			Health: &Health{
				Status: HealthStatusHealthy,
			},
		}
		lb = NewLoadBalancer(StrategyRoundRobin, []*Backend{backend})
	}

	// Create transport with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}

	return &HTTPProxy{
		route:        route,
		loadBalancer: lb,
		transport:    transport,
	}
}

// ServeHTTP implements http.Handler
func (hp *HTTPProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Select backend
	backend := hp.loadBalancer.SelectBackend(req)
	if backend == nil {
		http.Error(w, "No healthy backends available", http.StatusServiceUnavailable)
		return
	}

	// Track connection
	backend.Health.IncrementConnections()
	defer backend.Health.DecrementConnections()

	// Create reverse proxy
	proxy := &httputil.ReverseProxy{
		Director:     hp.createDirector(backend, req),
		Transport:    hp.transport,
		ErrorHandler: hp.errorHandler,
	}

	// Proxy the request
	proxy.ServeHTTP(w, req)
}

// createDirector creates a director function for the reverse proxy
func (hp *HTTPProxy) createDirector(backend *Backend, originalReq *http.Request) func(*http.Request) {
	return func(r *http.Request) {
		// Set target URL
		r.URL.Scheme = backend.URL.Scheme
		r.URL.Host = backend.URL.Host
		r.Host = backend.URL.Host

		// Combine backend path with request path
		if backend.URL.Path != "" {
			r.URL.Path = singleJoiningSlash(backend.URL.Path, r.URL.Path)
		}

		// Strip prefix if configured
		if hp.route.StripPrefix != "" {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, hp.route.StripPrefix)
			// Ensure path starts with /
			if !strings.HasPrefix(r.URL.Path, "/") {
				r.URL.Path = "/" + r.URL.Path
			}
		}

		// Add custom headers
		for k, v := range hp.route.AddHeaders {
			r.Header.Set(k, v)
		}

		// Add standard proxy headers
		if clientIP, _, err := extractClientIP(originalReq); err == nil {
			r.Header.Set("X-Forwarded-For", clientIP)
		}

		r.Header.Set("X-Forwarded-Proto", originalReq.URL.Scheme)
		if originalReq.URL.Scheme == "" {
			if originalReq.TLS != nil {
				r.Header.Set("X-Forwarded-Proto", "https")
			} else {
				r.Header.Set("X-Forwarded-Proto", "http")
			}
		}

		r.Header.Set("X-Forwarded-Host", originalReq.Host)
	}
}

// errorHandler handles proxy errors
func (hp *HTTPProxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Proxy error for %s: %v", r.URL.String(), err)

	// TODO: Implement retry logic with another backend
	// TODO: Implement circuit breaker

	http.Error(w, fmt.Sprintf("Bad Gateway: %v", err), http.StatusBadGateway)
}

// singleJoiningSlash joins two URL paths with a single slash
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")

	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

// extractClientIP extracts the client IP from the request
func extractClientIP(r *http.Request) (string, string, error) {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			return ip, "", nil
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri, "", nil
	}

	// Use RemoteAddr
	ip := r.RemoteAddr
	if colonIdx := strings.LastIndex(ip, ":"); colonIdx != -1 {
		ip = ip[:colonIdx]
	}

	return ip, "", nil
}
