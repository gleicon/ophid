package proxy

import (
	"net/http"
	"path/filepath"
)

// StaticHandler serves static files
type StaticHandler struct {
	route *Route
}

// ServeHTTP implements http.Handler
func (sh *StaticHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if sh.route.StaticRoot == "" {
		http.Error(w, "Static root not configured", http.StatusInternalServerError)
		return
	}

	// Clean and validate path
	urlPath := req.URL.Path
	if sh.route.StripPrefix != "" {
		urlPath = urlPath[len(sh.route.StripPrefix):]
	}

	// Construct file path
	filePath := filepath.Join(sh.route.StaticRoot, filepath.Clean(urlPath))

	// Serve file
	http.ServeFile(w, req, filePath)
}
