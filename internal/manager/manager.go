// Package manager orchestrates multiple supervised processes.
package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/EdgarOrtegaRamirez/processwatch/internal/config"
	"github.com/EdgarOrtegaRamirez/processwatch/internal/process"
)

// Manager supervises multiple processes.
type Manager struct {
	config     *config.Config
	processes  map[string]*process.Process
	mu         sync.RWMutex
	running    bool
	signalCh   chan os.Signal
	logDir     string
}

// New creates a new Manager from config.
func New(cfg *config.Config) (*Manager, error) {
	m := &Manager{
		config:    cfg,
		processes: make(map[string]*process.Process),
		logDir:    cfg.LogDir,
		signalCh:  make(chan os.Signal, 1),
	}

	// Create process instances
	for _, pc := range cfg.Processes {
		p, err := process.New(pc, m.logDir)
		if err != nil {
			// Clean up already created processes
			for _, pp := range m.processes {
			_ = pp.Close()
			}
			return nil, fmt.Errorf("creating process %s: %w", pc.Name, err)
		}
		m.processes[pc.Name] = p
	}

	return m, nil
}

// StartAll starts all enabled processes.
func (m *Manager) StartAll() error {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	// Set up signal handling
	signal.Notify(m.signalCh, syscall.SIGINT, syscall.SIGTERM)
	go m.handleSignals()

	fmt.Println("Starting processes...")

	started := 0
	for _, p := range m.processes {
		info := p.Info()
		if info.State == process.StateStopped || info.State == process.StateErrored {
			if err := p.Start(); err != nil {
				fmt.Printf("  [%s] Error: %v\n", p.Name(), err)
				continue
			}
			started++
		}
	}

	fmt.Printf("\nStarted %d process(es)\n", started)
	return nil
}

// Start starts a specific process by name.
func (m *Manager) Start(name string) error {
	p := m.findProcess(name)
	if p == nil {
		return fmt.Errorf("process %q not found", name)
	}
	return p.Start()
}

// Stop stops a specific process by name.
func (m *Manager) Stop(name string) error {
	p := m.findProcess(name)
	if p == nil {
		return fmt.Errorf("process %q not found", name)
	}
	return p.Stop()
}

// StopAll stops all running processes.
func (m *Manager) StopAll() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()

	fmt.Println("\nStopping all processes...")

	// Sort by name for consistent order
	names := m.processNames()
	for _, name := range names {
		p := m.processes[name]
		if p.State() != process.StateStopped {
			p.Stop()
		}
	}
	fmt.Println("All processes stopped.")
}

// Restart restarts a specific process by name.
func (m *Manager) Restart(name string) error {
	p := m.findProcess(name)
	if p == nil {
		return fmt.Errorf("process %q not found", name)
	}
	return p.Restart()
}

// Status returns the status of all processes.
func (m *Manager) Status() []process.Info {
	var infos []process.Info
	for _, p := range m.processes {
		infos = append(infos, p.Info())
	}

	// Sort by name
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos
}

// StatusJSON returns status as JSON.
func (m *Manager) StatusJSON() ([]byte, error) {
	infos := m.Status()
	return json.MarshalIndent(infos, "", "  ")
}

// StatusText returns a formatted text status.
func (m *Manager) StatusText() string {
	infos := m.Status()
	if len(infos) == 0 {
		return "No processes configured."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-20s %-12s %-8s %-10s %-8s %s\n",
		"NAME", "STATE", "PID", "UPTIME", "RESTARTS", "HEALTH"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")

	for _, info := range infos {
		stateColor := stateColor(info.State)
		resetColor := "\033[0m"

		uptime := "-"
		if info.Uptime > 0 {
			uptime = formatDuration(info.Uptime)
		}

		pid := "-"
		if info.PID > 0 {
			pid = fmt.Sprintf("%d", info.PID)
		}

		health := info.LastHealth
		if health == "" {
			health = "-"
		}

		sb.WriteString(fmt.Sprintf("%s%-20s%s %-12s %-8s %-10s %-8d %s\n",
			stateColor, info.Name, resetColor,
			info.State, pid, uptime, info.Restarts, health))
	}

	return sb.String()
}

// StatusCompact returns a compact single-line status.
func (m *Manager) StatusCompact() string {
	infos := m.Status()
	var parts []string
	for _, info := range infos {
		parts = append(parts, fmt.Sprintf("%s=%s", info.Name, info.State))
	}
	return strings.Join(parts, ", ")
}

// WaitForExit blocks until all processes have stopped.
func (m *Manager) WaitForExit() {
	for {
		m.mu.RLock()
		running := m.running
		m.mu.RUnlock()

		if !running {
			return
		}

		allStopped := true
		for _, p := range m.processes {
			if p.State() != process.StateStopped && p.State() != process.StateErrored {
				allStopped = false
				break
			}
		}
		if allStopped {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// Close releases all resources.
func (m *Manager) Close() {
	for _, p := range m.processes {
		_ = p.Close()
	}
}

func (m *Manager) handleSignals() {
	sig := <-m.signalCh
	fmt.Printf("\nReceived signal: %v\n", sig)
	m.StopAll()
	os.Exit(0)
}

func (m *Manager) findProcess(name string) *process.Process {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.processes[name]
}

func (m *Manager) processNames() []string {
	var names []string
	for name := range m.processes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func stateColor(state process.State) string {
	switch state {
	case process.StateRunning:
		return "\033[32m" // green
	case process.StateStopped:
		return "\033[90m" // gray
	case process.StateStarting:
		return "\033[33m" // yellow
	case process.StateBackoff:
		return "\033[35m" // magenta
	case process.StateErrored:
		return "\033[31m" // red
	case process.StateStopping:
		return "\033[33m" // yellow
	default:
		return "\033[0m"
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", h, m)
}
