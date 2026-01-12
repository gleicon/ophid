package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// HealthChecker performs health checks on processes
// Adapted from guvnor health checker
type HealthChecker struct {
	manager *Manager
	client  *http.Client
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(manager *Manager) *HealthChecker {
	return &HealthChecker{
		manager: manager,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CheckProcess performs a health check on a process
func (h *HealthChecker) CheckProcess(proc *Process) error {
	if !proc.Config.HealthCheck.Enabled {
		return nil
	}

	switch proc.Config.HealthCheck.Type {
	case "http":
		return h.checkHTTP(proc)
	case "tcp":
		return h.checkTCP(proc)
	case "process":
		return h.checkProcess(proc)
	default:
		return fmt.Errorf("unknown health check type: %s", proc.Config.HealthCheck.Type)
	}
}

// checkHTTP performs an HTTP health check
func (h *HealthChecker) checkHTTP(proc *Process) error {
	ctx, cancel := context.WithTimeout(context.Background(), proc.Config.HealthCheck.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", proc.Config.HealthCheck.Endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unhealthy status code: %d", resp.StatusCode)
	}

	return nil
}

// checkTCP performs a TCP health check
func (h *HealthChecker) checkTCP(proc *Process) error {
	endpoint := proc.Config.HealthCheck.Endpoint
	if endpoint == "" {
		return fmt.Errorf("TCP health check endpoint not configured")
	}

	timeout := proc.Config.HealthCheck.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second // Default timeout
	}

	slog.Debug("performing TCP health check",
		"process", proc.Config.Name,
		"endpoint", endpoint,
		"timeout", timeout)

	// Attempt to establish TCP connection
	conn, err := net.DialTimeout("tcp", endpoint, timeout)
	if err != nil {
		return fmt.Errorf("TCP connection failed: %w", err)
	}

	// Connection successful, close it immediately
	conn.Close()

	slog.Debug("TCP health check passed",
		"process", proc.Config.Name,
		"endpoint", endpoint)

	return nil
}

// checkProcess checks if the process is still running
func (h *HealthChecker) checkProcess(proc *Process) error {
	if proc.Cmd == nil || proc.Cmd.Process == nil {
		return fmt.Errorf("process not running")
	}

	// Send signal 0 to check if process exists
	if err := proc.Cmd.Process.Signal(nil); err != nil {
		return fmt.Errorf("process check failed: %w", err)
	}

	return nil
}

// StartMonitoring starts continuous health monitoring
func (h *HealthChecker) StartMonitoring(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.checkAll(ctx)
		}
	}
}

// checkAll checks all processes
func (h *HealthChecker) checkAll(ctx context.Context) {
	processes := h.manager.List()

	for name, proc := range processes {
		if !proc.IsRunning() {
			continue
		}

		if err := h.CheckProcess(proc); err != nil {
			slog.Warn("health check failed",
				"process", name,
				"error", err)

			// Restart if auto-restart is enabled
			if proc.Config.AutoRestart {
				slog.Info("restarting process due to failed health check",
					"process", name)
				if err := h.manager.Restart(ctx, name); err != nil {
					slog.Error("failed to restart process",
						"process", name,
						"error", err)
				}
			}
		}
	}
}
