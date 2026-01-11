package supervisor

import (
	"context"
	"testing"
	"time"
)

func TestManager_StartStop(t *testing.T) {
	mgr := NewManager()

	config := ProcessConfig{
		Name:    "test",
		Command: "sleep",
		Args:    []string{"10"},
	}

	ctx := context.Background()

	// Start process
	if err := mgr.Start(ctx, config); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Check process is running
	proc, exists := mgr.Get("test")
	if !exists {
		t.Fatal("Process not found after start")
	}

	if !proc.IsRunning() {
		t.Error("Process should be running")
	}

	// Stop process
	if err := mgr.Stop("test"); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Check process is stopped
	if proc.IsRunning() {
		t.Error("Process should be stopped")
	}
}

func TestManager_List(t *testing.T) {
	mgr := NewManager()

	config1 := ProcessConfig{Name: "test1", Command: "sleep", Args: []string{"10"}}
	config2 := ProcessConfig{Name: "test2", Command: "sleep", Args: []string{"10"}}

	ctx := context.Background()

	if err := mgr.Start(ctx, config1); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := mgr.Start(ctx, config2); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	processes := mgr.List()
	if len(processes) != 2 {
		t.Errorf("List() returned %d processes, want 2", len(processes))
	}

	// Cleanup
	mgr.StopAll()
}

func TestManager_AutoRestart(t *testing.T) {
	mgr := NewManager()

	// Use a command that exits immediately
	config := ProcessConfig{
		Name:        "test",
		Command:     "echo",
		Args:        []string{"hello"},
		AutoRestart: true,
		MaxRetries:  2,
	}

	ctx := context.Background()

	if err := mgr.Start(ctx, config); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait a bit for restarts to happen
	time.Sleep(3 * time.Second)

	proc, exists := mgr.Get("test")
	if !exists {
		t.Fatal("Process not found")
	}

	// Should have attempted restarts
	if proc.RestartCount == 0 {
		t.Error("Expected restart attempts, got 0")
	}
}

func TestProcess_Status(t *testing.T) {
	proc := &Process{
		Config: ProcessConfig{Name: "test"},
		Status: StatusStopped,
	}

	if proc.GetStatus() != StatusStopped {
		t.Errorf("GetStatus() = %v, want %v", proc.GetStatus(), StatusStopped)
	}

	proc.SetStatus(StatusRunning)

	if proc.GetStatus() != StatusRunning {
		t.Errorf("GetStatus() = %v, want %v", proc.GetStatus(), StatusRunning)
	}

	if !proc.IsRunning() {
		t.Error("IsRunning() should return true")
	}
}
