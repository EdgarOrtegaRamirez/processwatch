package process

import (
	"testing"
	"time"

	"github.com/EdgarOrtegaRamirez/processwatch/internal/config"
)

func TestProcessStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateStopped, "STOPPED"},
		{StateStarting, "STARTING"},
		{StateRunning, "RUNNING"},
		{StateBackoff, "BACKOFF"},
		{StateStopping, "STOPPING"},
		{StateErrored, "ERRORED"},
		{State(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestNewProcess(t *testing.T) {
	cfg := config.ProcessConfig{
		Name:           "test-echo",
		Command:        "echo",
		Args:           []string{"hello"},
		MaxRestarts:    3,
		RestartDelay:   1 * time.Second,
		BackoffFactor:  2.0,
		MaxBackoff:     10 * time.Second,
		StopSignal:     "TERM",
		StopTimeoutSec: 5,
	}

	p, err := New(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	if p.Name() != "test-echo" {
		t.Errorf("expected name 'test-echo', got %q", p.Name())
	}
	if p.State() != StateStopped {
		t.Errorf("expected state STOPPED, got %v", p.State())
	}
}

func TestProcessStartAndStop(t *testing.T) {
	cfg := config.ProcessConfig{
		Name:           "test-proc",
		Command:        "sleep",
		Args:           []string{"100"},
		MaxRestarts:    0, // Disable auto-restart for this test
		StopSignal:     "TERM",
		StopTimeoutSec: 2,
	}

	p, err := New(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	// Start the process
	if err := p.Start(); err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	// Wait a moment for process to start
	time.Sleep(100 * time.Millisecond)

	// Check state
	if p.State() != StateRunning {
		t.Errorf("expected state RUNNING, got %v", p.State())
	}

	info := p.Info()
	if info.PID <= 0 {
		t.Errorf("expected positive PID, got %d", info.PID)
	}
	if info.Name != "test-proc" {
		t.Errorf("expected name 'test-proc', got %q", info.Name)
	}

	// Stop the process
	if err := p.Stop(); err != nil {
		t.Fatalf("failed to stop process: %v", err)
	}

	if p.State() != StateStopped {
		t.Errorf("expected state STOPPED, got %v", p.State())
	}

	info = p.Info()
	if info.PID != 0 {
		t.Errorf("expected PID 0 after stop, got %d", info.PID)
	}
}

func TestProcessInfo(t *testing.T) {
	cfg := config.ProcessConfig{
		Name:           "info-test",
		Command:        "echo",
		Args:           []string{"test"},
		StopSignal:     "TERM",
		StopTimeoutSec: 2,
	}

	p, err := New(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	info := p.Info()
	if info.Name != "info-test" {
		t.Errorf("expected name 'info-test', got %q", info.Name)
	}
	if info.State != StateStopped {
		t.Errorf("expected state STOPPED, got %v", info.State)
	}
	if info.Restarts != 0 {
		t.Errorf("expected 0 restarts, got %d", info.Restarts)
	}
	if info.Uptime != 0 {
		t.Errorf("expected 0 uptime, got %v", info.Uptime)
	}
}

func TestProcessName(t *testing.T) {
	cfg := config.ProcessConfig{
		Name:    "named-process",
		Command: "echo",
	}

	p, err := New(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	if p.Name() != "named-process" {
		t.Errorf("expected name 'named-process', got %q", p.Name())
	}
}

func TestProcessClose(t *testing.T) {
	cfg := config.ProcessConfig{
		Name:    "close-test",
		Command: "echo",
	}

	p, err := New(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close should not error
	if err := p.Close(); err != nil {
		t.Errorf("unexpected error closing: %v", err)
	}

	// Double close should be safe
	if err := p.Close(); err != nil {
		t.Errorf("unexpected error on double close: %v", err)
	}
}

func TestProcessInfoWhileRunning(t *testing.T) {
	cfg := config.ProcessConfig{
		Name:           "running-info",
		Command:        "sleep",
		Args:           []string{"100"},
		MaxRestarts:    0,
		StopSignal:     "TERM",
		StopTimeoutSec: 2,
	}

	p, err := New(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer p.Close()

	if err := p.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	defer p.Stop()

	time.Sleep(100 * time.Millisecond)

	info := p.Info()
	if info.State != StateRunning {
		t.Errorf("expected RUNNING, got %v", info.State)
	}
	if info.Uptime < 50*time.Millisecond {
		t.Errorf("expected some uptime, got %v", info.Uptime)
	}
}

func TestParseExitCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"nil error", nil, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseExitCode(tt.err)
			if got != tt.expected {
				t.Errorf("ParseExitCode() = %q, want %q", got, tt.expected)
			}
		})
	}
}
