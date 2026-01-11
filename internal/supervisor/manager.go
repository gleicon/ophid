package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Manager manages multiple processes
type Manager struct {
	processes map[string]*Process
	mu        sync.RWMutex
}

// NewManager creates a new process manager
func NewManager() *Manager {
	return &Manager{
		processes: make(map[string]*Process),
	}
}

// Start starts a process
func (m *Manager) Start(ctx context.Context, config ProcessConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running
	if proc, exists := m.processes[config.Name]; exists {
		if proc.IsRunning() {
			return fmt.Errorf("process %s is already running", config.Name)
		}
	}

	// Create process
	proc := &Process{
		Config:    config,
		StartTime: time.Now(),
		Status:    StatusStarting,
	}

	// Start process
	if err := m.startProcess(proc); err != nil {
		proc.SetStatus(StatusFailed)
		return fmt.Errorf("failed to start process: %w", err)
	}

	m.processes[config.Name] = proc

	// Monitor process
	go m.monitorProcess(ctx, proc)

	return nil
}

// Stop stops a process
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proc, exists := m.processes[name]
	if !exists {
		return fmt.Errorf("process %s not found", name)
	}

	if !proc.IsRunning() {
		return fmt.Errorf("process %s is not running", name)
	}

	// Kill process
	if proc.Cmd != nil && proc.Cmd.Process != nil {
		if err := proc.Cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	proc.SetStatus(StatusStopped)
	delete(m.processes, name)

	return nil
}

// Restart restarts a process
func (m *Manager) Restart(ctx context.Context, name string) error {
	if err := m.Stop(name); err != nil {
		return err
	}

	m.mu.RLock()
	proc, exists := m.processes[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("process %s not found after stop", name)
	}

	return m.Start(ctx, proc.Config)
}

// List returns all processes
func (m *Manager) List() map[string]*Process {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Process)
	for name, proc := range m.processes {
		result[name] = proc
	}
	return result
}

// Get returns a specific process
func (m *Manager) Get(name string) (*Process, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	proc, exists := m.processes[name]
	return proc, exists
}

// StopAll stops all processes
func (m *Manager) StopAll() error {
	m.mu.RLock()
	names := make([]string, 0, len(m.processes))
	for name := range m.processes {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		if err := m.Stop(name); err != nil {
			return err
		}
	}

	return nil
}

// startProcess starts the actual process
func (m *Manager) startProcess(proc *Process) error {
	cmd := exec.Command(proc.Config.Command, proc.Config.Args...)

	// Set working directory
	if proc.Config.WorkingDir != "" {
		cmd.Dir = proc.Config.WorkingDir
	}

	// Set environment
	if len(proc.Config.Environment) > 0 {
		env := os.Environ()
		for k, v := range proc.Config.Environment {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	// Inherit stdout/stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start process
	if err := cmd.Start(); err != nil {
		return err
	}

	proc.Cmd = cmd
	proc.SetStatus(StatusRunning)

	return nil
}

// monitorProcess monitors a process and handles auto-restart
func (m *Manager) monitorProcess(ctx context.Context, proc *Process) {
	// Wait for process to exit
	err := proc.Cmd.Wait()

	// Process exited
	proc.SetStatus(StatusStopped)

	// Check if should auto-restart
	if proc.Config.AutoRestart && proc.RestartCount < proc.Config.MaxRetries {
		proc.RestartCount++
		fmt.Printf("Process %s exited (error: %v), restarting (attempt %d/%d)...\n",
			proc.Config.Name, err, proc.RestartCount, proc.Config.MaxRetries)

		// Wait a bit before restarting
		time.Sleep(2 * time.Second)

		// Restart
		if err := m.startProcess(proc); err != nil {
			fmt.Printf("Failed to restart %s: %v\n", proc.Config.Name, err)
			proc.SetStatus(StatusFailed)
			return
		}

		// Continue monitoring
		go m.monitorProcess(ctx, proc)
	} else {
		if err != nil {
			proc.SetStatus(StatusFailed)
			fmt.Printf("Process %s failed: %v\n", proc.Config.Name, err)
		} else {
			fmt.Printf("Process %s stopped\n", proc.Config.Name)
		}
	}
}
