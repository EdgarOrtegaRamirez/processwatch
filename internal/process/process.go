// Package process manages individual supervised processes.
package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/EdgarOrtegaRamirez/processwatch/internal/config"
	"github.com/EdgarOrtegaRamirez/processwatch/internal/healthcheck"
	plog "github.com/EdgarOrtegaRamirez/processwatch/internal/log"
)

// State represents the current state of a process.
type State int

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateBackoff
	StateStopping
	StateErrored
)

func (s State) String() string {
	switch s {
	case StateStopped:
		return "STOPPED"
	case StateStarting:
		return "STARTING"
	case StateRunning:
		return "RUNNING"
	case StateBackoff:
		return "BACKOFF"
	case StateStopping:
		return "STOPPING"
	case StateErrored:
		return "ERRORED"
	default:
		return "UNKNOWN"
	}
}

// Info contains runtime information about a supervised process.
type Info struct {
	Name       string        `json:"name"`
	State      State         `json:"state"`
	PID        int           `json:"pid"`
	StartTime  time.Time     `json:"start_time"`
	Uptime     time.Duration `json:"uptime"`
	Restarts   int           `json:"restarts"`
	ExitCode   int           `json:"exit_code"`
	LastHealth string        `json:"last_health"`
}

// Process represents a supervised process.
type Process struct {
	config    config.ProcessConfig
	logger    *plog.Logger
	health    *healthcheck.Checker
	state     State
	pid       int
	startTime time.Time
	restarts  int
	exitCode  int
	mu        sync.RWMutex
	ctx       context.CancelFunc
	cmd       *exec.Cmd
	exitCh    chan struct{} // signals when process has exited
	healthOK  bool
}

// New creates a new Process from config.
func New(cfg config.ProcessConfig, logDir string) (*Process, error) {
	p := &Process{
		config: cfg,
		state:  StateStopped,
		exitCh: make(chan struct{}),
	}

	// Create logger
	var err error
	p.logger, err = plog.New(cfg.Name, logDir, cfg.StdoutLog, cfg.StderrLog, cfg.StdoutLogMax)
	if err != nil {
		return nil, fmt.Errorf("creating logger for %s: %w", cfg.Name, err)
	}

	// Create health checker
	if cfg.HealthCheck != nil {
		p.health = healthcheck.New(cfg.HealthCheck)
	}

	return p, nil
}

// Start starts the process.
func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == StateRunning || p.state == StateStarting {
		return fmt.Errorf("process %s is already running", p.config.Name)
	}

	p.state = StateStarting

	// Build command
	args := append([]string{}, p.config.Args...)
	p.cmd = exec.CommandContext(p.context(), p.config.Command, args...)
	p.cmd.Dir = p.config.Dir
	if p.cmd.Dir == "" {
		p.cmd.Dir, _ = os.Getwd()
	}

	// Set environment
	p.cmd.Env = os.Environ()
	for k, v := range p.config.Env {
		p.cmd.Env = append(p.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Connect output to logger
	p.cmd.Stdout = p.logger.StdoutWriter()
	p.cmd.Stderr = p.logger.StderrWriter()

	// Start the process
	if err := p.cmd.Start(); err != nil {
		p.state = StateErrored
		p.exitCode = -1
		return fmt.Errorf("starting process %s: %w", p.config.Name, err)
	}

	p.pid = p.cmd.Process.Pid
	p.startTime = time.Now()
	p.state = StateRunning
	p.healthOK = true
	p.exitCh = make(chan struct{})

	fmt.Printf("  [%s] Started (PID %d)\n", p.config.Name, p.pid)

	// Monitor in background - single goroutine calls Wait()
	go p.monitor()

	// Start health checking
	if p.health != nil {
		go p.healthLoop()
	}

	return nil
}

// Stop sends a signal to stop the process gracefully.
func (p *Process) Stop() error {
	p.mu.Lock()
	if p.state == StateStopped || p.state == StateStopping {
		p.mu.Unlock()
		return nil
	}

	p.state = StateStopping
	fmt.Printf("  [%s] Stopping...\n", p.config.Name)

	cmd := p.cmd
	sig := p.getStopSignal()
	timeout := time.Duration(p.config.StopTimeoutSec) * time.Second
	p.mu.Unlock()

	// Send stop signal outside of lock
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Signal(sig)

		// Wait for process to exit (with timeout)
		select {
		case <-p.exitCh:
			// Process exited
		case <-time.After(timeout):
			// Force kill
			fmt.Printf("  [%s] Force killing (timeout)\n", p.config.Name)
			_ = cmd.Process.Kill()
			<-p.exitCh
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = StateStopped
	p.pid = 0
	p.restarts = 0
	fmt.Printf("  [%s] Stopped\n", p.config.Name)
	return nil
}

// Restart stops and starts the process.
func (p *Process) Restart() error {
	if err := p.Stop(); err != nil {
		return err
	}
	time.Sleep(p.config.RestartDelay)
	return p.Start()
}

// Info returns the current process info.
func (p *Process) Info() Info {
	p.mu.RLock()
	defer p.mu.RUnlock()

	info := Info{
		Name:      p.config.Name,
		State:     p.state,
		PID:       p.pid,
		StartTime: p.startTime,
		Restarts:  p.restarts,
		ExitCode:  p.exitCode,
	}
	if p.state == StateRunning {
		info.Uptime = time.Since(p.startTime)
	}
	if p.healthOK {
		info.LastHealth = "OK"
	} else {
		info.LastHealth = "FAIL"
	}
	return info
}

// State returns the current state.
func (p *Process) State() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// Name returns the process name.
func (p *Process) Name() string {
	return p.config.Name
}

// Close releases resources.
func (p *Process) Close() error {
	if p.ctx != nil {
		p.ctx()
	}
	if p.logger != nil {
		return p.logger.Close()
	}
	return nil
}

func (p *Process) context() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	p.ctx = cancel
	return ctx
}

func (p *Process) monitor() {
	if p.cmd == nil {
		return
	}

	// Wait for process to exit - this is the ONLY goroutine that calls Wait()
	err := p.cmd.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == StateStopping {
		// Intentional stop - just signal exit
		p.pid = 0
		close(p.exitCh)
		return
	}

	p.exitCode = exitCode(err)
	p.pid = 0

	if !p.config.Autorestart {
		p.state = StateStopped
		fmt.Printf("  [%s] Exited (code %d) - autorestart disabled\n", p.config.Name, p.exitCode)
		return
	}

	// Check if exit code is in the "clean" list
	if !isCleanExit(p.exitCode, p.config.ExitCodes) {
		p.restarts++
		if p.restarts > p.config.MaxRestarts {
			p.state = StateErrored
			fmt.Printf("  [%s] Exceeded max restarts (%d) - giving up\n", p.config.Name, p.config.MaxRestarts)
			return
		}
		p.state = StateBackoff
		delay := p.backoffDelay()
		fmt.Printf("  [%s] Exited (code %d) - restarting in %v (attempt %d/%d)\n",
			p.config.Name, p.exitCode, delay, p.restarts, p.config.MaxRestarts)
		p.mu.Unlock()
		time.Sleep(delay)
		p.mu.Lock()

		if p.state == StateStopping || p.state == StateStopped {
			return
		}
		p.startUnlocked()
	} else {
		// Clean exit - just restart
		p.restarts++
		if p.restarts > p.config.MaxRestarts {
			p.state = StateStopped
			fmt.Printf("  [%s] Clean exit - max restarts reached\n", p.config.Name)
			return
		}
		delay := p.config.RestartDelay
		fmt.Printf("  [%s] Exited cleanly - restarting in %v\n", p.config.Name, delay)
		p.mu.Unlock()
		time.Sleep(delay)
		p.mu.Lock()
		if p.state == StateStopping || p.state == StateStopped {
			return
		}
		p.startUnlocked()
	}
}

func (p *Process) startUnlocked() {
	p.state = StateStarting

	args := append([]string{}, p.config.Args...)
	p.cmd = exec.CommandContext(p.context(), p.config.Command, args...)
	p.cmd.Dir = p.config.Dir
	if p.cmd.Dir == "" {
		p.cmd.Dir, _ = os.Getwd()
	}
	p.cmd.Env = os.Environ()
	for k, v := range p.config.Env {
		p.cmd.Env = append(p.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	p.cmd.Stdout = p.logger.StdoutWriter()
	p.cmd.Stderr = p.logger.StderrWriter()

	if err := p.cmd.Start(); err != nil {
		p.state = StateErrored
		p.exitCode = -1
		fmt.Printf("  [%s] Failed to restart: %v\n", p.config.Name, err)
		return
	}

	p.pid = p.cmd.Process.Pid
	p.startTime = time.Now()
	p.state = StateRunning
	fmt.Printf("  [%s] Restarted (PID %d)\n", p.config.Name, p.pid)
}

func (p *Process) healthLoop() {
	ticker := time.NewTicker(p.health.Interval())
	defer ticker.Stop()

	// Initial wait
	time.Sleep(5 * time.Second)

	for {
		select {
		case <-p.exitCh:
			return
		case <-ticker.C:
			p.mu.RLock()
			state := p.state
			p.mu.RUnlock()
			if state != StateRunning {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			ok, _ := p.health.Check(ctx)
			cancel()

			p.mu.Lock()
			p.healthOK = ok
			p.mu.Unlock()

			if !ok {
				fmt.Printf("  [%s] Health check FAILED\n", p.config.Name)
			}
		}
	}
}

func (p *Process) backoffDelay() time.Duration {
	delay := p.config.RestartDelay
	factor := p.config.BackoffFactor
	if factor < 1.0 {
		factor = 2.0
	}
	for i := 1; i < p.restarts; i++ {
		delay = time.Duration(float64(delay) * factor)
		if delay > p.config.MaxBackoff {
			delay = p.config.MaxBackoff
			break
		}
	}
	return delay
}

func (p *Process) getStopSignal() os.Signal {
	switch strings.ToUpper(p.config.StopSignal) {
	case "INT":
		return syscall.SIGINT
	case "HUP":
		return syscall.SIGHUP
	case "QUIT":
		return syscall.SIGQUIT
	case "USR1":
		return syscall.SIGUSR1
	case "USR2":
		return syscall.SIGUSR2
	default:
		return syscall.SIGTERM
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	return -1
}

var codePattern = regexp.MustCompile(`\d+`)

func isCleanExit(code int, allowed []int) bool {
	for _, c := range allowed {
		if c == code {
			return true
		}
	}
	return false
}

// ParseExitCode extracts exit code from error message for display.
func ParseExitCode(err error) string {
	if err == nil {
		return "0"
	}
	msg := err.Error()
	matches := codePattern.FindString(msg)
	if matches != "" {
		return matches
	}
	return "?"
}
