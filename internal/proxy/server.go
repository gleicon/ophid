package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// Server is the main HTTP/HTTPS reverse proxy server
type Server struct {
	config      *Config
	router      *Router
	tlsManager  *autocert.Manager
	httpServer  *http.Server
	httpsServer *http.Server
}

// NewServer creates a new proxy server
func NewServer(config *Config) (*Server, error) {
	// Create router and add routes
	router := NewRouter()
	for i := range config.Routes {
		// Parse backend URLs
		for j := range config.Routes[i].Backends {
			backend := config.Routes[i].Backends[j]
			if backend.URLStr != "" && backend.URL == nil {
				parsedURL, err := parseBackendURL(backend.URLStr)
				if err != nil {
					return nil, fmt.Errorf("invalid backend URL %s: %w", backend.URLStr, err)
				}
				backend.URL = parsedURL
			}
		}
		router.AddRoute(&config.Routes[i])
	}

	server := &Server{
		config: config,
		router: router,
	}

	// Setup TLS if enabled
	if config.TLS.Enabled {
		server.setupTLS()
	}

	return server, nil
}

// setupTLS configures TLS with Let's Encrypt
func (s *Server) setupTLS() {
	cacheDir := s.config.TLS.CacheDir
	if cacheDir == "" {
		cacheDir = ".ophid/certs"
	}

	s.tlsManager = &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Email:      s.config.TLS.ACMEEmail,
		HostPolicy: autocert.HostWhitelist(s.config.TLS.Domains...),
		Cache:      autocert.DirCache(cacheDir),
	}
}

// Start starts the proxy server
func (s *Server) Start() error {
	// Determine listen addresses
	httpAddr := ":80"
	httpsAddr := ":443"

	if len(s.config.General.Listen) > 0 {
		httpAddr = s.config.General.Listen[0]
	}
	if len(s.config.General.Listen) > 1 {
		httpsAddr = s.config.General.Listen[1]
	}

	// Start HTTP server
	if s.config.TLS.Enabled && s.config.TLS.AutoRedirect {
		// Redirect HTTP to HTTPS
		go s.startHTTPRedirect(httpAddr)
	} else {
		go s.startHTTP(httpAddr)
	}

	// Start HTTPS server if TLS is enabled
	if s.config.TLS.Enabled {
		return s.startHTTPS(httpsAddr)
	}

	// If no TLS, keep HTTP server running
	select {}
}

// startHTTP starts the HTTP server
func (s *Server) startHTTP(addr string) error {
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Starting HTTP server on %s", addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("HTTP server error: %v", err)
		return err
	}

	return nil
}

// startHTTPRedirect starts HTTP server that redirects to HTTPS
func (s *Server) startHTTPRedirect(addr string) error {
	// Create redirect handler that also handles ACME challenges
	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if it's an ACME challenge
		if s.tlsManager != nil {
			if h := s.tlsManager.HTTPHandler(nil); h != nil {
				h.ServeHTTP(w, r)
				return
			}
		}

		// Redirect to HTTPS
		target := "https://" + r.Host + r.RequestURI
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      redirectHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Starting HTTP redirect server on %s", addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("HTTP redirect server error: %v", err)
		return err
	}

	return nil
}

// startHTTPS starts the HTTPS server
func (s *Server) startHTTPS(addr string) error {
	tlsConfig := &tls.Config{
		GetCertificate: s.tlsManager.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1"}, // HTTP/2 support
		MinVersion:     tls.VersionTLS12,
	}

	s.httpsServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		TLSConfig:    tlsConfig,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Starting HTTPS server on %s", addr)
	if err := s.httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		log.Printf("HTTPS server error: %v", err)
		return err
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down proxy server...")

	errChan := make(chan error, 2)

	// Shutdown HTTP server
	go func() {
		if s.httpServer != nil {
			errChan <- s.httpServer.Shutdown(ctx)
		} else {
			errChan <- nil
		}
	}()

	// Shutdown HTTPS server
	go func() {
		if s.httpsServer != nil {
			errChan <- s.httpsServer.Shutdown(ctx)
		} else {
			errChan <- nil
		}
	}()

	// Wait for both shutdowns
	err1 := <-errChan
	err2 := <-errChan

	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}

	log.Println("Proxy server shutdown complete")
	return nil
}

// Reload reloads the configuration without downtime
func (s *Server) Reload(newConfig *Config) error {
	log.Println("Reloading proxy configuration...")

	// Create new router with new routes
	newRouter := NewRouter()
	for i := range newConfig.Routes {
		// Parse backend URLs
		for j := range newConfig.Routes[i].Backends {
			backend := newConfig.Routes[i].Backends[j]
			if backend.URLStr != "" && backend.URL == nil {
				parsedURL, err := parseBackendURL(backend.URLStr)
				if err != nil {
					return fmt.Errorf("invalid backend URL %s: %w", backend.URLStr, err)
				}
				backend.URL = parsedURL
			}
		}
		newRouter.AddRoute(&newConfig.Routes[i])
	}

	// Atomically swap routers
	s.router = newRouter
	s.config = newConfig

	log.Println("Configuration reloaded successfully")
	return nil
}

// parseBackendURL parses a backend URL string
func parseBackendURL(urlStr string) (*url.URL, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	// Ensure scheme is set
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "http"
	}

	return parsedURL, nil
}
