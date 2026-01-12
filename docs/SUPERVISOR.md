# OPHID Supervisor

**Component:** Process Supervision and Management
**Status:** Implemented
**Priority:** High

## Overview

The Supervisor provides process management for tools and services, enabling OPHID to run and monitor long-lived processes with automatic restart, health checks, and structured logging.

**Inspired by:** systemd, supervisor, pm2, guvnor

## Goals

1. **Automatic restart** on process failure
2. **Health monitoring** with configurable checks
3. **Graceful shutdown** and reload
4. **Simple configuration** (no complex unit files)

## Architecture

### High-Level Design

```
┌───────────────────────────────────────────────┐
│  Supervisor                                   │
├───────────────────────────────────────────────┤
│                                               │
│  ┌──────────────┐  ┌──────────────────────┐  │
│  │ Process Mgr  │  │ Health Checker       │  │
│  │ - Start/Stop │  │ - HTTP checks        │  │
│  │ - Restart    │  │ - TCP checks         │  │
│  │ - Monitor    │  │ - Script checks      │  │
│  └──────┬───────┘  └─────────┬────────────┘  │
│         │                    │                │
│  ┌──────▼───────┐  ┌─────────▼────────────┐  │
│  │ Restart Mgr  │  │ Resource Controller  │  │
│  │ - Backoff    │  │ - CPU limits         │  │
│  │ - Policies   │  │ - Memory limits      │  │
│  │ - Cooldown   │  │ - cgroups (Linux)    │  │
│  └──────────────┘  └──────────────────────┘  │
│                                               │
│  ┌───────────────────────────────────────┐   │
│  │ Log Manager                           │   │
│  │ - stdout/stderr capture               │   │
│  │ - Log rotation                        │   │
│  │ - Structured logging                  │   │
│  └───────────────────────────────────────┘   │
└───────────────────────────────────────────────┘
```

### Data Flow

```
User Command (start/stop/restart)
     ↓
Process Manager
     ↓
┌────▼────────────────────────────────┐
│ Fork & Execute Process              │
│  - Set working directory            │
│  - Set environment variables        │
│  - Apply resource limits            │
│  - Redirect stdout/stderr to logger │
└────┬────────────────────────────────┘
     ↓
┌────▼────────────────────────────────┐
│ Monitor Loop                        │
│  - Wait for process exit            │
│  - Run health checks periodically   │
│  - Track resource usage             │
│  - Detect crashes                   │
└────┬────────────────────────────────┘
     ↓
Process Exit/Crash
     ↓
┌────▼────────────────────────────────┐
│ Restart Policy Decision             │
│  - Check restart policy             │
│  - Apply backoff strategy           │
│  - Check max restarts limit         │
└────┬────────────────────────────────┘
     ↓
Restart or Stop
```

## Process Management

### Process Lifecycle

```go
// internal/supervisor/process.go
package supervisor

import (
    "os/exec"
    "syscall"
)

type Process struct {
    // Identity
    Name    string
    PID     int
    PPID    int

    // Configuration
    Command     string
    Args        []string
    WorkingDir  string
    User        string
    Group       string
    Env         map[string]string

    // State
    Status      ProcessStatus
    StartTime   time.Time
    Uptime      time.Duration
    Restarts    int
    LastRestart time.Time

    // Policies
    RestartPolicy string // "always", "on-failure", "never"
    MaxRestarts   int
    RestartDelay  time.Duration

    // Resources
    Resources *ResourceLimits

    // Health
    HealthCheck *HealthCheck

    // Logging
    StdoutLog string
    StderrLog string
}

type ProcessStatus string

const (
    StatusStarting ProcessStatus = "starting"
    StatusRunning  ProcessStatus = "running"
    StatusStopping ProcessStatus = "stopping"
    StatusStopped  ProcessStatus = "stopped"
    StatusFailed   ProcessStatus = "failed"
    StatusUnhealthy ProcessStatus = "unhealthy"
)

func (p *Process) Start() error {
    // 1. Validate configuration
    if err := p.validate(); err != nil {
        return err
    }

    // 2. Set up command
    cmd := exec.Command(p.Command, p.Args...)
    cmd.Dir = p.WorkingDir
    cmd.Env = p.buildEnv()

    // 3. Set up logging
    stdout, _ := p.setupStdoutLog()
    stderr, _ := p.setupStderrLog()
    cmd.Stdout = stdout
    cmd.Stderr = stderr

    // 4. Apply resource limits (platform-specific)
    if err := p.applyResourceLimits(cmd); err != nil {
        return err
    }

    // 5. Start process
    if err := cmd.Start(); err != nil {
        return err
    }

    p.PID = cmd.Process.Pid
    p.Status = StatusStarting
    p.StartTime = time.Now()

    // 6. Wait for process in goroutine
    go p.wait(cmd)

    return nil
}

func (p *Process) Stop() error {
    if p.Status == StatusStopped {
        return nil
    }

    p.Status = StatusStopping

    // 1. Send SIGTERM for graceful shutdown
    if err := p.signal(syscall.SIGTERM); err != nil {
        return err
    }

    // 2. Wait for graceful shutdown (timeout: 30s)
    timeout := time.After(30 * time.Second)
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-timeout:
            // Force kill with SIGKILL
            p.signal(syscall.SIGKILL)
            return nil
        case <-ticker.C:
            if p.Status == StatusStopped {
                return nil
            }
        }
    }
}

func (p *Process) Restart() error {
    if err := p.Stop(); err != nil {
        return err
    }

    // Apply restart delay
    time.Sleep(p.RestartDelay)

    return p.Start()
}

func (p *Process) wait(cmd *exec.Cmd) {
    err := cmd.Wait()

    p.Status = StatusStopped

    if err != nil {
        // Process crashed
        p.handleCrash(err)
    }
}

func (p *Process) handleCrash(err error) {
    p.Status = StatusFailed
    p.Restarts++
    p.LastRestart = time.Now()

    // Check restart policy
    if !p.shouldRestart() {
        return
    }

    // Apply exponential backoff
    delay := p.calculateBackoff()
    time.Sleep(delay)

    // Restart
    p.Start()
}

func (p *Process) shouldRestart() bool {
    switch p.RestartPolicy {
    case "always":
        return p.Restarts < p.MaxRestarts
    case "on-failure":
        return p.Restarts < p.MaxRestarts
    case "never":
        return false
    default:
        return false
    }
}

func (p *Process) calculateBackoff() time.Duration {
    // Exponential backoff: 1s, 2s, 4s, 8s, 16s, max 60s
    delay := time.Duration(1<<uint(p.Restarts)) * time.Second
    if delay > 60*time.Second {
        delay = 60 * time.Second
    }
    return delay
}
```

### Supervisor Manager

```go
// internal/supervisor/supervisor.go
package supervisor

type Supervisor struct {
    processes map[string]*Process
    mu        sync.RWMutex
    configDir string
}

func NewSupervisor(configDir string) *Supervisor {
    return &Supervisor{
        processes: make(map[string]*Process),
        configDir: configDir,
    }
}

func (s *Supervisor) Start(name string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    proc, exists := s.processes[name]
    if !exists {
        return fmt.Errorf("process %s not found", name)
    }

    return proc.Start()
}

func (s *Supervisor) Stop(name string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    proc, exists := s.processes[name]
    if !exists {
        return fmt.Errorf("process %s not found", name)
    }

    return proc.Stop()
}

func (s *Supervisor) Restart(name string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    proc, exists := s.processes[name]
    if !exists {
        return fmt.Errorf("process %s not found", name)
    }

    return proc.Restart()
}

func (s *Supervisor) List() []*Process {
    s.mu.RLock()
    defer s.mu.RUnlock()

    procs := make([]*Process, 0, len(s.processes))
    for _, proc := range s.processes {
        procs = append(procs, proc)
    }

    return procs
}

func (s *Supervisor) LoadConfig(path string) error {
    // Load process configuration from TOML file
    // Add to s.processes
    return nil
}

func (s *Supervisor) SaveConfig(proc *Process) error {
    // Save process configuration to TOML file
    return nil
}
```

## Health Checks

### Implemented Health Check Types

1. **HTTP/HTTPS:** GET request to endpoint (returns 2xx status)
2. **TCP:** TCP connection test to host:port
3. **Process:** Check if process is still running (signal 0)

### TCP Health Check Implementation

Uses `net.DialTimeout` to establish TCP connections with configurable timeouts. Useful for services without HTTP endpoints (databases, Redis, message queues).

**Features:**
- Connection-only testing (no application-level protocol)
- Configurable timeout (defaults to 5 seconds)
- Immediate connection closure to prevent resource leaks
- Structured logging at debug level

### Implementation

```go
// internal/supervisor/health.go
package supervisor

type HealthCheck struct {
    Type     string        // "http", "tcp", "script", "process"
    Target   string        // URL, host:port, or script path
    Interval time.Duration // How often to check
    Timeout  time.Duration // Check timeout
    Retries  int           // Failed checks before unhealthy
}

type HealthChecker struct {
    checks map[string]*HealthCheck
    mu     sync.RWMutex
}

func (hc *HealthChecker) Check(proc *Process) error {
    check := proc.HealthCheck
    if check == nil {
        return nil // No health check configured
    }

    switch check.Type {
    case "http":
        return hc.checkHTTP(check)
    case "tcp":
        return hc.checkTCP(check)
    case "script":
        return hc.checkScript(check)
    case "process":
        return hc.checkProcess(proc)
    default:
        return fmt.Errorf("unknown health check type: %s", check.Type)
    }
}

func (hc *HealthChecker) checkHTTP(check *HealthCheck) error {
    client := &http.Client{
        Timeout: check.Timeout,
    }

    resp, err := client.Get(check.Target)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("unhealthy: HTTP %d", resp.StatusCode)
    }

    return nil
}

func (hc *HealthChecker) checkTCP(check *HealthCheck) error {
    conn, err := net.DialTimeout("tcp", check.Target, check.Timeout)
    if err != nil {
        return err
    }
    defer conn.Close()

    return nil
}

func (hc *HealthChecker) checkScript(check *HealthCheck) error {
    ctx, cancel := context.WithTimeout(context.Background(), check.Timeout)
    defer cancel()

    cmd := exec.CommandContext(ctx, check.Target)
    err := cmd.Run()

    if err != nil {
        return fmt.Errorf("script failed: %v", err)
    }

    return nil
}

func (hc *HealthChecker) checkProcess(proc *Process) error {
    if proc.PID == 0 {
        return fmt.Errorf("process not running")
    }

    // Check if PID exists
    process, err := os.FindProcess(proc.PID)
    if err != nil {
        return err
    }

    // Send signal 0 to check if process exists
    err = process.Signal(syscall.Signal(0))
    if err != nil {
        return fmt.Errorf("process not found")
    }

    return nil
}

func (hc *HealthChecker) StartMonitoring(proc *Process) {
    if proc.HealthCheck == nil {
        return
    }

    ticker := time.NewTicker(proc.HealthCheck.Interval)
    defer ticker.Stop()

    failCount := 0

    for range ticker.C {
        if proc.Status == StatusStopped {
            return
        }

        err := hc.Check(proc)
        if err != nil {
            failCount++
            if failCount >= proc.HealthCheck.Retries {
                proc.Status = StatusUnhealthy
                // Optionally restart process
                proc.handleUnhealthy()
            }
        } else {
            failCount = 0
            if proc.Status == StatusStarting {
                proc.Status = StatusRunning
            }
        }
    }
}
```

## Resource Limits

### Resource Controller

```go
// internal/supervisor/resources.go
package supervisor

type ResourceLimits struct {
    MaxCPU    string // "50%" or "0.5" cores
    MaxMemory string // "512M", "1G"
    MaxFDs    int    // File descriptors
}

type ResourceController struct {
    // Platform-specific implementation
    cgroups bool // Use cgroups on Linux
}

func (rc *ResourceController) Apply(cmd *exec.Cmd, limits *ResourceLimits) error {
    if runtime.GOOS == "linux" && rc.cgroups {
        return rc.applyCgroups(cmd, limits)
    } else {
        return rc.applyRlimit(cmd, limits)
    }
}

// Linux: cgroups v2
func (rc *ResourceController) applyCgroups(cmd *exec.Cmd, limits *ResourceLimits) error {
    // Create cgroup for process
    cgroupPath := fmt.Sprintf("/sys/fs/cgroup/ophid/%d", os.Getpid())

    // Set memory limit
    if limits.MaxMemory != "" {
        memBytes := parseMemory(limits.MaxMemory)
        ioutil.WriteFile(
            filepath.Join(cgroupPath, "memory.max"),
            []byte(fmt.Sprintf("%d", memBytes)),
            0644,
        )
    }

    // Set CPU limit
    if limits.MaxCPU != "" {
        cpuQuota := parseCPU(limits.MaxCPU)
        ioutil.WriteFile(
            filepath.Join(cgroupPath, "cpu.max"),
            []byte(fmt.Sprintf("%d 100000", cpuQuota)),
            0644,
        )
    }

    return nil
}

// Fallback: setrlimit
func (rc *ResourceController) applyRlimit(cmd *exec.Cmd, limits *ResourceLimits) error {
    if cmd.SysProcAttr == nil {
        cmd.SysProcAttr = &syscall.SysProcAttr{}
    }

    // Set file descriptor limit
    if limits.MaxFDs > 0 {
        cmd.SysProcAttr.Rlimit = []syscall.Rlimit{
            {
                Cur: uint64(limits.MaxFDs),
                Max: uint64(limits.MaxFDs),
            },
        }
    }

    // Note: Memory and CPU limits harder with setrlimit
    // May need platform-specific code

    return nil
}

func parseMemory(s string) int64 {
    // Parse "512M", "1G", etc.
    // Return bytes
}

func parseCPU(s string) int64 {
    // Parse "50%", "0.5", etc.
    // Return CPU quota
}
```

## Log Management

### Log Capture

```go
// internal/supervisor/logger.go
package supervisor

type Logger struct {
    stdoutPath string
    stderrPath string
    rotate     *LogRotation
}

type LogRotation struct {
    MaxSize  int64 // Bytes
    MaxFiles int   // Number of old files to keep
}

func (l *Logger) SetupStdout(proc *Process) (io.Writer, error) {
    // Open log file
    f, err := os.OpenFile(
        proc.StdoutLog,
        os.O_CREATE|os.O_WRONLY|os.O_APPEND,
        0644,
    )
    if err != nil {
        return nil, err
    }

    // Wrap with rotation
    rotator := &RotatingWriter{
        file:     f,
        path:     proc.StdoutLog,
        maxSize:  l.rotate.MaxSize,
        maxFiles: l.rotate.MaxFiles,
    }

    // Also write to structured logger
    multi := io.MultiWriter(rotator, l.structuredLogger(proc))

    return multi, nil
}

type RotatingWriter struct {
    file     *os.File
    path     string
    maxSize  int64
    maxFiles int
    current  int64
}

func (rw *RotatingWriter) Write(p []byte) (n int, err error) {
    // Check if rotation needed
    if rw.current+int64(len(p)) > rw.maxSize {
        rw.rotate()
    }

    n, err = rw.file.Write(p)
    rw.current += int64(n)

    return n, err
}

func (rw *RotatingWriter) rotate() error {
    // Close current file
    rw.file.Close()

    // Rename old files
    for i := rw.maxFiles - 1; i >= 0; i-- {
        oldPath := fmt.Sprintf("%s.%d", rw.path, i)
        newPath := fmt.Sprintf("%s.%d", rw.path, i+1)
        os.Rename(oldPath, newPath)
    }

    // Move current to .0
    os.Rename(rw.path, rw.path+".0")

    // Open new file
    f, err := os.OpenFile(rw.path, os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }

    rw.file = f
    rw.current = 0

    return nil
}
```

## Configuration

### Process Configuration File

```toml
# ~/.ophid/supervise/myapp.toml
[process]
name = "myapp"
command = "ophid run myapp"
args = ["--port", "3000"]
directory = "/opt/apps/myapp"
user = "www-data"
group = "www-data"

[restart]
policy = "always"        # always, on-failure, never
max_restarts = 10
restart_delay = "5s"
backoff = "exponential"  # exponential, linear, constant

[resources]
max_memory = "512M"
max_cpu = "50%"
max_fds = 1024

[health]
type = "http"
target = "http://localhost:3000/health"
interval = "30s"
timeout = "5s"
retries = 3

[logging]
stdout = "/var/log/ophid/myapp.log"
stderr = "/var/log/ophid/myapp.err"
rotate_size = "100M"
rotate_count = 5

[env]
NODE_ENV = "production"
PORT = "3000"
```

### Global Configuration

```toml
# ~/.ophid/config.toml
[supervisor]
enabled = true
config_dir = "~/.ophid/supervise"
log_dir = "/var/log/ophid"
pid_file = "/var/run/ophid-supervisor.pid"

[supervisor.defaults]
restart_policy = "always"
max_restarts = 5
restart_delay = "5s"
health_interval = "30s"
log_rotate_size = "100M"
log_rotate_count = 5
```

## CLI Commands

```bash
# Start supervisor daemon
ophid supervise daemon start
ophid supervise daemon stop
ophid supervise daemon status

# Manage processes
ophid supervise start <name>
ophid supervise stop <name>
ophid supervise restart <name>
ophid supervise reload <name>  # Reload config

# List processes
ophid supervise list
ophid supervise status
ophid supervise ps

# View logs
ophid supervise logs <name>
ophid supervise logs <name> --follow
ophid supervise logs <name> --tail 100

# Configuration
ophid supervise add <name> --cmd "..." --restart always
ophid supervise remove <name>
ophid supervise config <name>
ophid supervise config <name> --edit
```

## Status Display

```bash
$ ophid supervise status

Supervisor: running (PID 1234, uptime 5d 3h)

PROCESS    STATUS      PID    UPTIME   RESTARTS  MEMORY   CPU    HEALTH
myapp      running     5678   2d 5h    0         245M     12%    OK
api        running     5679   2d 5h    1         189M     8%     OK
worker     running     5680   1d 2h    0         512M     25%    OK
nginx      running     5681   5d 3h    0         89M      3%     OK
db-backup  stopped     -      -        -         -        -      -

$ ophid supervise logs myapp --tail 10
[2025-01-10 14:30:01] INFO: Request GET /api/users
[2025-01-10 14:30:02] INFO: Response 200 (15ms)
[2025-01-10 14:30:05] INFO: Request POST /api/users
[2025-01-10 14:30:06] INFO: Response 201 (42ms)
...
```

## Integration with Other Components

### With Security Scanner

```bash
# Supervise with security checks
ophid supervise start myapp --scan

# Periodic security scans
ophid supervise start myapp --scan-interval 24h
```

### With Proxy

```bash
# Auto-register with proxy
ophid supervise start myapp --register-proxy
# Creates proxy route: myapp.local -> localhost:3000
```

## Platform-Specific Considerations

### Linux
- **cgroups v2** for resource limits
- **systemd integration** (optional)
- **journald** for logging (optional)

### macOS
- **launchd integration** (optional)
- Resource limits via setrlimit

### Windows
- **Windows Services** integration
- Resource limits via Job Objects

## Open Questions

1. **Daemon Mode:**
   - Should supervisor run as daemon or on-demand?
   - Integration with systemd/launchd?

2. **Resource Limits:**
   - Require cgroups or fallback to rlimit?
   - How to handle Windows?

3. **Zero-Downtime:**
   - Support rolling restarts?
   - Old process keeps running while new starts?

4. **Process Groups:**
   - Start/stop multiple related processes together?
   - Dependencies between processes?

5. **Remote Management:**
   - RPC interface for remote control?
   - Web UI for monitoring?

## Next Steps

1. PASS Design complete
2. Pending Implement process manager
3. Pending Implement health checker
4. Pending Implement resource controller
5. Pending Implement log manager
6. Pending Add CLI commands
7. Pending Write tests
8. Pending Documentation

**Related:** [PLATFORM_VISION.md](../PLATFORM_VISION.md)
