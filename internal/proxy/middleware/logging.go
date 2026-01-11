package middleware

import (
	"log"
	"net/http"
	"time"
)

// Logger implements access logging middleware
type Logger struct {
	logger *log.Logger
}

// NewLogger creates a new logging middleware
func NewLogger(logger *log.Logger) *Logger {
	if logger == nil {
		logger = log.Default()
	}

	return &Logger{
		logger: logger,
	}
}

// Middleware returns the logging middleware
func (l *Logger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			status:         200,
		}

		// Call next handler
		next.ServeHTTP(wrapped, r)

		// Log request
		duration := time.Since(start)
		l.logger.Printf("%s %s %s %d %s %s",
			extractIP(r),
			r.Method,
			r.RequestURI,
			wrapped.status,
			duration,
			r.UserAgent(),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// Write ensures status is set
func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = 200
	}
	return rw.ResponseWriter.Write(b)
}
