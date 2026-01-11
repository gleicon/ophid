package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
)

// WebSocketProxy handles WebSocket proxying
type WebSocketProxy struct {
	route *Route
}

// ServeHTTP implements http.Handler
func (wsp *WebSocketProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Get backend
	var backendURL *url.URL
	if wsp.route.Target != "" {
		var err error
		backendURL, err = url.Parse(wsp.route.Target)
		if err != nil {
			http.Error(w, "Invalid backend URL", http.StatusInternalServerError)
			return
		}
	} else if len(wsp.route.Backends) > 0 {
		backendURL = wsp.route.Backends[0].URL
	} else {
		http.Error(w, "No backend configured", http.StatusInternalServerError)
		return
	}

	// For now, return not implemented
	// Full WebSocket proxy implementation would require gorilla/websocket or similar
	log.Printf("WebSocket proxy not fully implemented for %s", backendURL)
	http.Error(w, fmt.Sprintf("WebSocket proxying to %s - not fully implemented yet", backendURL), http.StatusNotImplemented)

	// TODO: Implement full WebSocket proxying:
	// 1. Upgrade client connection
	// 2. Connect to backend WebSocket
	// 3. Bidirectional message forwarding
}
