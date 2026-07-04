package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.LogDir != "logs" {
		t.Errorf("expected log dir 'logs', got %q", cfg.LogDir)
	}
	if cfg.Global.MaxRestarts != 5 {
		t.Errorf("expected max restarts 5, got %d", cfg.Global.MaxRestarts)
	}
	if cfg.Global.RestartDelay != 1*time.Second {
		t.Errorf("expected restart delay 1s, got %v", cfg.Global.RestartDelay)
	}
	if cfg.Global.BackoffFactor != 2.0 {
		t.Errorf("expected backoff factor 2.0, got %f", cfg.Global.BackoffFactor)
	}
}

func TestLoadSimple(t *testing.T) {
	yaml := `
programs:
  - name: test-prog
    command: echo
    args: ["hello"]
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(cfg.Processes))
	}
	p := cfg.Processes[0]
	if p.Name != "test-prog" {
		t.Errorf("expected name 'test-prog', got %q", p.Name)
	}
	if p.Command != "echo" {
		t.Errorf("expected command 'echo', got %q", p.Command)
	}
	if len(p.Args) != 1 || p.Args[0] != "hello" {
		t.Errorf("expected args ['hello'], got %v", p.Args)
	}
}

func TestLoadWithEnv(t *testing.T) {
	yaml := `
programs:
  - name: env-test
    command: env
    env:
      FOO: bar
      BAZ: qux
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := cfg.Processes[0]
	if p.Env["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got %q", p.Env["FOO"])
	}
	if p.Env["BAZ"] != "qux" {
		t.Errorf("expected BAZ=qux, got %q", p.Env["BAZ"])
	}
}

func TestLoadMultipleProcesses(t *testing.T) {
	yaml := `
programs:
  - name: proc1
    command: echo
    args: ["one"]
  - name: proc2
    command: echo
    args: ["two"]
  - name: proc3
    command: sleep
    args: ["10"]
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Processes) != 3 {
		t.Fatalf("expected 3 processes, got %d", len(cfg.Processes))
	}
	names := map[string]bool{}
	for _, p := range cfg.Processes {
		names[p.Name] = true
	}
	for _, expected := range []string{"proc1", "proc2", "proc3"} {
		if !names[expected] {
			t.Errorf("missing process %q", expected)
		}
	}
}

func TestLoadMissingName(t *testing.T) {
	yaml := `
programs:
  - command: echo
`
	_, err := Load([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoadMissingCommand(t *testing.T) {
	yaml := `
programs:
  - name: test
`
	_, err := Load([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	yaml := `{{{not valid yaml`
	_, err := Load([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestDefaultProcessConfig(t *testing.T) {
	global := &GlobalConfig{
		MaxRestarts:   10,
		RestartDelay:  2 * time.Second,
		BackoffFactor: 3.0,
		MaxBackoff:    60 * time.Second,
	}

	pc := &ProcessConfig{
		Name:         "test",
		Command:      "echo",
		Autorestart:  true,
	}

	DefaultProcessConfig(pc, global)

	if pc.MaxRestarts != 10 {
		t.Errorf("expected max restarts 10, got %d", pc.MaxRestarts)
	}
	if pc.RestartDelay != 2*time.Second {
		t.Errorf("expected restart delay 2s, got %v", pc.RestartDelay)
	}
	if pc.BackoffFactor != 3.0 {
		t.Errorf("expected backoff factor 3.0, got %f", pc.BackoffFactor)
	}
	if pc.MaxBackoff != 60*time.Second {
		t.Errorf("expected max backoff 60s, got %v", pc.MaxBackoff)
	}
	if pc.StopSignal != "TERM" {
		t.Errorf("expected stop signal TERM, got %q", pc.StopSignal)
	}
	if pc.StopTimeoutSec != 10 {
		t.Errorf("expected stop timeout 10, got %d", pc.StopTimeoutSec)
	}
}

func TestDefaultProcessConfigDisabled(t *testing.T) {
	global := &GlobalConfig{
		MaxRestarts: 10,
	}

	pc := &ProcessConfig{
		Name:         "test",
		Command:      "echo",
		Autorestart:  false,
	}

	DefaultProcessConfig(pc, global)

	if pc.MaxRestarts != 0 {
		t.Errorf("expected max restarts 0, got %d", pc.MaxRestarts)
	}
	if pc.Autorestart != false {
		t.Error("expected autorestart false")
	}
}

func TestLoadWithHealthCheck(t *testing.T) {
	yaml := `
programs:
  - name: web
    command: python3
    args: ["-m", "http.server", "8080"]
    healthcheck:
      type: http
      endpoint: http://localhost:8080
      interval: 5s
      timeout: 3s
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hc := cfg.Processes[0].HealthCheck
	if hc == nil {
		t.Fatal("expected health check config")
	}
	if hc.Type != "http" {
		t.Errorf("expected type 'http', got %q", hc.Type)
	}
	if hc.Endpoint != "http://localhost:8080" {
		t.Errorf("expected endpoint 'http://localhost:8080', got %q", hc.Endpoint)
	}
	if hc.Interval != 5*time.Second {
		t.Errorf("expected interval 5s, got %v", hc.Interval)
	}
	if hc.Timeout != 3*time.Second {
		t.Errorf("expected timeout 3s, got %v", hc.Timeout)
	}
}

func TestLoadWithGlobalConfig(t *testing.T) {
	yaml := `
global:
  max_restarts: 20
  restart_delay: 5s
  backoff_factor: 1.5
  max_backoff: 60s
  log_max_size_mb: 100
  log_max_backups: 5
programs:
  - name: test
    command: echo
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Global.MaxRestarts != 20 {
		t.Errorf("expected max restarts 20, got %d", cfg.Global.MaxRestarts)
	}
	if cfg.Global.RestartDelay != 5*time.Second {
		t.Errorf("expected restart delay 5s, got %v", cfg.Global.RestartDelay)
	}
	if cfg.Global.LogMaxSize != 100 {
		t.Errorf("expected log max size 100, got %d", cfg.Global.LogMaxSize)
	}
	if cfg.Global.LogMaxBackups != 5 {
		t.Errorf("expected log max backups 5, got %d", cfg.Global.LogMaxBackups)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/config.yml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadFileValid(t *testing.T) {
	content := `
programs:
  - name: test-file
    command: echo
    args: ["from file"]
`
	tmpFile, err := os.CreateTemp("", "processwatch-test-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	_ = tmpFile.Close()

	cfg, err := LoadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(cfg.Processes))
	}
	if cfg.Processes[0].Name != "test-file" {
		t.Errorf("expected name 'test-file', got %q", cfg.Processes[0].Name)
	}
}

func TestSampleConfig(t *testing.T) {
	sample := SampleConfig()
	if sample == "" {
		t.Fatal("sample config should not be empty")
	}
	cfg, err := Load([]byte(sample))
	if err != nil {
		t.Fatalf("sample config should be valid YAML: %v", err)
	}
	if len(cfg.Processes) == 0 {
		t.Fatal("sample config should have at least one process")
	}
}

func TestLoadWithDirAndDisabled(t *testing.T) {
	yaml := `
programs:
  - name: prod-app
    command: node
    args: ["server.js"]
    dir: /app
    enabled: false
    autorestart: false
    max_restarts: 0
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := cfg.Processes[0]
	if p.Dir != "/app" {
		t.Errorf("expected dir '/app', got %q", p.Dir)
	}
	if p.Autorestart != false {
		t.Error("expected autorestart false")
	}
	if p.MaxRestarts != 0 {
		t.Errorf("expected max_restarts 0, got %d", p.MaxRestarts)
	}
}

func TestLoadWithExitCodes(t *testing.T) {
	yaml := `
programs:
  - name: custom-exit
    command: exit
    args: ["1"]
    exit_codes: [0, 1, 2]
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := cfg.Processes[0]
	if len(p.ExitCodes) != 3 {
		t.Fatalf("expected 3 exit codes, got %d", len(p.ExitCodes))
	}
	if p.ExitCodes[0] != 0 || p.ExitCodes[1] != 1 || p.ExitCodes[2] != 2 {
		t.Errorf("unexpected exit codes: %v", p.ExitCodes)
	}
}

func TestLoadWithStopSignal(t *testing.T) {
	yaml := `
programs:
  - name: custom-signal
    command: sleep
    args: ["100"]
    stop_signal: INT
    stop_timeout: 5
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := cfg.Processes[0]
	if p.StopSignal != "INT" {
		t.Errorf("expected stop signal 'INT', got %q", p.StopSignal)
	}
	if p.StopTimeoutSec != 5 {
		t.Errorf("expected stop timeout 5, got %d", p.StopTimeoutSec)
	}
}

func TestLoadWithLogConfig(t *testing.T) {
	yaml := `
programs:
  - name: logged
    command: echo
    stdout_log: stdout.log
    stderr_log: stderr.log
    stdout_log_max_mb: 100
    stderr_log_max_mb: 200
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := cfg.Processes[0]
	if p.StdoutLog != "stdout.log" {
		t.Errorf("expected stdout_log 'stdout.log', got %q", p.StdoutLog)
	}
	if p.StderrLog != "stderr.log" {
		t.Errorf("expected stderr_log 'stderr.log', got %q", p.StderrLog)
	}
	if p.StdoutLogMax != 100 {
		t.Errorf("expected stdout_log_max_mb 100, got %d", p.StdoutLogMax)
	}
	if p.StderrLogMax != 200 {
		t.Errorf("expected stderr_log_max_mb 200, got %d", p.StderrLogMax)
	}
}

func TestLoadEmptyPrograms(t *testing.T) {
	yaml := `
programs: []
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Processes) != 0 {
		t.Errorf("expected 0 processes, got %d", len(cfg.Processes))
	}
}

func TestLoadNoPrograms(t *testing.T) {
	yaml := `
log_dir: /tmp/logs
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Processes) != 0 {
		t.Errorf("expected 0 processes, got %d", len(cfg.Processes))
	}
	if cfg.LogDir != "/tmp/logs" {
		t.Errorf("expected log_dir '/tmp/logs', got %q", cfg.LogDir)
	}
}

func TestLoadHealthCheckTCP(t *testing.T) {
	yaml := `
programs:
  - name: redis
    command: redis-server
    healthcheck:
      type: tcp
      port: 6379
      endpoint: 127.0.0.1
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hc := cfg.Processes[0].HealthCheck
	if hc == nil {
		t.Fatal("expected health check config")
	}
	if hc.Type != "tcp" {
		t.Errorf("expected type 'tcp', got %q", hc.Type)
	}
	if hc.Port != 6379 {
		t.Errorf("expected port 6379, got %d", hc.Port)
	}
	if hc.Endpoint != "127.0.0.1" {
		t.Errorf("expected endpoint '127.0.0.1', got %q", hc.Endpoint)
	}
}

func TestLoadDuplicateNamesAllowed(t *testing.T) {
	yaml := `
programs:
  - name: dup
    command: echo
    args: ["1"]
  - name: dup
    command: echo
    args: ["2"]
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should allow duplicate names (manager will handle conflicts)
	if len(cfg.Processes) != 2 {
		t.Errorf("expected 2 processes, got %d", len(cfg.Processes))
	}
}

func TestLoadWithLogDir(t *testing.T) {
	yaml := `
log_dir: /var/log/myapp
programs:
  - name: test
    command: echo
`
	cfg, err := Load([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogDir != "/var/log/myapp" {
		t.Errorf("expected log_dir '/var/log/myapp', got %q", cfg.LogDir)
	}
}
