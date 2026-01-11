package middleware

import (
	"net/http"
	"strings"
)

// CORS implements CORS middleware
type CORS struct {
	allowOrigins []string
	allowMethods []string
	allowHeaders []string
	allowAll     bool
}

// NewCORS creates a new CORS middleware
func NewCORS(allowOrigins, allowMethods, allowHeaders []string) *CORS {
	cors := &CORS{
		allowOrigins: allowOrigins,
		allowMethods: allowMethods,
		allowHeaders: allowHeaders,
	}

	// Check if we should allow all origins
	for _, origin := range allowOrigins {
		if origin == "*" {
			cors.allowAll = true
			break
		}
	}

	// Default methods if not specified
	if len(cors.allowMethods) == 0 {
		cors.allowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
	}

	// Default headers if not specified
	if len(cors.allowHeaders) == 0 {
		cors.allowHeaders = []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "Authorization"}
	}

	return cors
}

// Middleware returns the CORS middleware
func (c *CORS) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Set CORS headers if origin is allowed
		if c.allowAll || c.isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(c.allowMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(c.allowHeaders, ", "))
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "3600")
		}

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

// isAllowedOrigin checks if an origin is allowed
func (c *CORS) isAllowedOrigin(origin string) bool {
	for _, allowed := range c.allowOrigins {
		if allowed == origin {
			return true
		}
	}
	return false
}
