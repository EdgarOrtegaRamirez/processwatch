package manager

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/EdgarOrtegaRamirez/processwatch/internal/config"
	"github.com/EdgarOrtegaRamirez/processwatch/internal/process"
)

func testConfig() *config.Config {
	return &config.Config{
		LogDir: "/tmp/processwatch-test",
		Global: config.GlobalConfig{
			MaxRestarts:   5,
			RestartDelay:  1 * time.Second,
			BackoffFactor: 2.0,
			MaxBackoff:    10 * time.Second,
			LogMaxSize:    50,
		},
		Processes: []config.ProcessConfig{
			{
				Name:           "echo-hello",
				Command:        "echo",
				Args:           []string{"hello"},
				MaxRestarts:    0,
				StopSignal:     "TERM",
				StopTimeoutSec: 2,
			},
			{
				Name:           "sleep-long",
				Command:        "sleep",
				Args:           []string{"100"},
				MaxRestarts:    0,
				StopSignal:     "TERM",
				StopTimeoutSec: 2,
			},
		},
	}
}

func TestNewManager(t *testing.T) {
	mgr, err := New(testConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Close()

	status := mgr.Status()
	if len(status) != 2 {
		t.Errorf("expected 2 processes, got %d", len(status))
	}
}

func TestManagerStatus(t *testing.T) {
	mgr, err := New(testConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Close()

	status := mgr.Status()
	if len(status) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(status))
	}

	// Check sorted by name
	if status[0].Name != "echo-hello" {
		t.Errorf("expected first process 'echo-hello', got %q", status[0].Name)
	}
	if status[1].Name != "sleep-long" {
		t.Errorf("expected second process 'sleep-long', got %q", status[1].Name)
	}

	// All should be stopped
	for _, info := range status {
		if info.State != 0 { // StateStopped = 0
			t.Errorf("process %s should be STOPPED, got %v", info.Name, info.State)
		}
	}
}

func TestManagerStatusJSON(t *testing.T) {
	mgr, err := New(testConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Close()

	data, err := mgr.StatusJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var infos []map[string]interface{}
	if err := json.Unmarshal(data, &infos); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(infos) != 2 {
		t.Errorf("expected 2 processes in JSON, got %d", len(infos))
	}
}

func TestManagerStatusText(t *testing.T) {
	mgr, err := New(testConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Close()

	text := mgr.StatusText()
	if text == "" {
		t.Error("expected non-empty status text")
	}
}

func TestManagerStatusCompact(t *testing.T) {
	mgr, err := New(testConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Close()

	compact := mgr.StatusCompact()
	if compact == "" {
		t.Error("expected non-empty compact status")
	}
}

func TestManagerStartStop(t *testing.T) {
	cfg := testConfig()
	cfg.Processes[0].Autorestart = false
	cfg.Processes[1].Autorestart = false

	mgr, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Close()

	// Start all
	if err := mgr.StartAll(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Check status
	status := mgr.Status()
	for _, info := range status {
		if info.Name == "sleep-long" && info.State != process.StateRunning {
			t.Errorf("sleep-long should be RUNNING, got %v", info.State)
		}
	}

	// Stop all
	mgr.StopAll()

	time.Sleep(200 * time.Millisecond)

	// Check stopped
	status = mgr.Status()
	for _, info := range status {
		if info.State != process.StateStopped {
			t.Errorf("process %s should be STOPPED, got %v", info.Name, info.State)
		}
	}
}

func TestManagerStartIndividual(t *testing.T) {
	cfg := testConfig()
	cfg.Processes[0].Autorestart = false
	cfg.Processes[1].Autorestart = false

	mgr, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Close()

	// Start all first
	if err := mgr.StartAll(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Start individual (should error if already running)
	err = mgr.Start("echo-hello")
	if err == nil {
		t.Log("started individual process (was stopped)")
	} else {
		t.Logf("expected error starting running process: %v", err)
	}

	mgr.StopAll()
	time.Sleep(200 * time.Millisecond)
}

func TestManagerStopNonexistent(t *testing.T) {
	mgr, err := New(testConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Close()

	err = mgr.Stop("nonexistent")
	if err == nil {
		t.Error("expected error stopping nonexistent process")
	}
}

func TestManagerRestartNonexistent(t *testing.T) {
	mgr, err := New(testConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Close()

	err = mgr.Restart("nonexistent")
	if err == nil {
		t.Error("expected error restarting nonexistent process")
	}
}

func TestManagerWithHealthCheck(t *testing.T) {
	cfg := &config.Config{
		LogDir: "/tmp/pw-test-health",
		Global: config.GlobalConfig{
			MaxRestarts:   5,
			RestartDelay:  1 * time.Second,
			BackoffFactor: 2.0,
			MaxBackoff:    10 * time.Second,
			LogMaxSize:    50,
		},
		Processes: []config.ProcessConfig{
			{
				Name:           "health-proc",
				Command:        "sleep",
				Args:           []string{"100"},
				MaxRestarts:    0,
				StopSignal:     "TERM",
				StopTimeoutSec: 2,
				HealthCheck: &config.HealthCheckConfig{
					Type:     "tcp",
					Port:     9999,
					Interval: 1 * time.Second,
					Timeout:  500 * time.Millisecond,
				},
			},
		},
	}

	mgr, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Close()

	// Should be able to create manager with health check
	status := mgr.Status()
	if len(status) != 1 {
		t.Errorf("expected 1 process, got %d", len(status))
	}
}

func TestManagerEmptyConfig(t *testing.T) {
	cfg := &config.Config{
		LogDir:    "/tmp/pw-test-empty",
		Processes: []config.ProcessConfig{},
	}

	mgr, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Close()

	status := mgr.Status()
	if len(status) != 0 {
		t.Errorf("expected 0 processes, got %d", len(status))
	}
}

func TestManagerClose(t *testing.T) {
	mgr, err := New(testConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close should not error
	mgr.Close()

	// Double close should be safe
	mgr.Close()
}

func TestManagerWithEnv(t *testing.T) {
	cfg := &config.Config{
		LogDir: "/tmp/pw-test-env",
		Global: config.GlobalConfig{
			MaxRestarts:   5,
			RestartDelay:  1 * time.Second,
			BackoffFactor: 2.0,
			MaxBackoff:    10 * time.Second,
			LogMaxSize:    50,
		},
		Processes: []config.ProcessConfig{
			{
				Name:           "env-proc",
				Command:        "env",
				MaxRestarts:    0,
				StopSignal:     "TERM",
				StopTimeoutSec: 2,
				Env: map[string]string{
					"TEST_VAR": "hello",
				},
			},
		},
	}

	mgr, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer mgr.Close()

	status := mgr.Status()
	if len(status) != 1 {
		t.Errorf("expected 1 process, got %d", len(status))
	}
}
