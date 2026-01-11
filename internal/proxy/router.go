package proxy

import (
	"net/http"
	"strings"
	"sync"
)

// Router handles request routing to backends
type Router struct {
	routes []*Route
	mu     sync.RWMutex
}

// NewRouter creates a new router
func NewRouter() *Router {
	return &Router{
		routes: make([]*Route, 0),
	}
}

// AddRoute adds a route to the router
func (r *Router) AddRoute(route *Route) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes = append(r.routes, route)
}

// RemoveRoute removes a route by host
func (r *Router) RemoveRoute(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, route := range r.routes {
		if route.Host == host {
			r.routes = append(r.routes[:i], r.routes[i+1:]...)
			return
		}
	}
}

// GetRoutes returns all routes
func (r *Router) GetRoutes() []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make([]*Route, len(r.routes))
	copy(routes, r.routes)
	return routes
}

// ServeHTTP implements http.Handler
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Find matching route
	route := r.match(req)
	if route == nil {
		http.NotFound(w, req)
		return
	}

	// Build handler
	handler := r.buildHandler(route)

	// Apply middleware
	for i := len(route.MiddlewareList) - 1; i >= 0; i-- {
		// TODO: Build middleware from config
		// handler = middleware(handler)
	}

	// Execute handler
	handler.ServeHTTP(w, req)
}

// match finds the first matching route for a request
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

// matchRoute checks if a route matches the request
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
	if route.Method != "" && route.Method != "*" && route.Method != req.Method {
		return false
	}

	return true
}

// buildHandler builds the appropriate handler for a route
func (r *Router) buildHandler(route *Route) http.Handler {
	if route.Static {
		return &StaticHandler{route: route}
	}

	if route.WebSocket {
		return &WebSocketProxy{route: route}
	}

	return NewHTTPProxy(route)
}

// matchHost checks if a host pattern matches a request host
func matchHost(pattern, host string) bool {
	// Remove port from host if present
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		host = host[:colonIdx]
	}

	// Exact match
	if pattern == "*" || pattern == host {
		return true
	}

	// Wildcard subdomain: *.example.com
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(host, suffix)
	}

	return false
}

// matchPath checks if a path pattern matches a request path
func matchPath(pattern, path string) bool {
	// Exact match
	if pattern == path {
		return true
	}

	// Prefix wildcard: /api/*
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix)
	}

	// Suffix wildcard: /*.jpg
	if strings.HasPrefix(pattern, "/*") {
		suffix := strings.TrimPrefix(pattern, "/*")
		return strings.HasSuffix(path, suffix)
	}

	return false
}
