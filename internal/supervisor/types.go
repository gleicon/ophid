package supervisor

import (
	"os/exec"
	"sync"
	"time"
)

// ProcessConfig defines how to run a process
type ProcessConfig struct {
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	AutoRestart bool              `json:"auto_restart"`
	MaxRetries  int               `json:"max_retries"`
	HealthCheck HealthCheckConfig `json:"health_check"`
}

// HealthCheckConfig defines health check parameters
type HealthCheckConfig struct {
	Enabled  bool          `json:"enabled"`
	Type     string        `json:"type"` // "http", "tcp", "process"
	Endpoint string        `json:"endpoint,omitempty"`
	Interval time.Duration `json:"interval"`
	Timeout  time.Duration `json:"timeout"`
	Retries  int           `json:"retries"`
}

// Process represents a running process
type Process struct {
	Config      ProcessConfig
	Cmd         *exec.Cmd
	StartTime   time.Time
	RestartCount int
	Status      ProcessStatus
	mu          sync.RWMutex
}

// ProcessStatus represents process state
type ProcessStatus string

const (
	StatusStopped  ProcessStatus = "stopped"
	StatusStarting ProcessStatus = "starting"
	StatusRunning  ProcessStatus = "running"
	StatusFailed   ProcessStatus = "failed"
)

// IsRunning returns true if the process is running
func (p *Process) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Status == StatusRunning
}

// GetStatus returns the current status
func (p *Process) GetStatus() ProcessStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Status
}

// SetStatus sets the process status
func (p *Process) SetStatus(status ProcessStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = status
}
