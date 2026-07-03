// Package config handles YAML configuration parsing for ProcessWatch.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level configuration.
type Config struct {
	Processes []ProcessConfig `yaml:"programs" json:"programs"`
	LogDir    string          `yaml:"log_dir" json:"log_dir"`
	Global    GlobalConfig    `yaml:"global" json:"global"`
}

// GlobalConfig contains global settings applied to all processes.
type GlobalConfig struct {
	MaxRestarts     int           `yaml:"max_restarts" json:"max_restarts"`
	RestartDelay    time.Duration `yaml:"restart_delay" json:"restart_delay"`
	BackoffFactor   float64       `yaml:"backoff_factor" json:"backoff_factor"`
	MaxBackoff      time.Duration `yaml:"max_backoff" json:"max_backoff"`
	LogMaxSize      int64         `yaml:"log_max_size_mb" json:"log_max_size_mb"`
	LogMaxBackups   int           `yaml:"log_max_backups" json:"log_max_backups"`
	HealthCheckWait time.Duration `yaml:"health_check_wait" json:"health_check_wait"`
}

// ProcessConfig defines a single process to supervise.
type ProcessConfig struct {
	Name            string            `yaml:"name" json:"name"`
	Command         string            `yaml:"command" json:"command"`
	Args            []string          `yaml:"args" json:"args"`
	Dir             string            `yaml:"dir" json:"dir"`
	Env             map[string]string `yaml:"env" json:"env"`
	Autorestart     bool              `yaml:"autorestart" json:"autorestart"`
	MaxRestarts     int               `yaml:"max_restarts" json:"max_restarts"`
	RestartDelay    time.Duration     `yaml:"restart_delay" json:"restart_delay"`
	BackoffFactor   float64           `yaml:"backoff_factor" json:"backoff_factor"`
	MaxBackoff      time.Duration     `yaml:"max_backoff" json:"max_backoff"`
	StdoutLog       string            `yaml:"stdout_log" json:"stdout_log"`
	StderrLog       string            `yaml:"stderr_log" json:"stderr_log"`
	StdoutLogMax    int64             `yaml:"stdout_log_max_mb" json:"stdout_log_max_mb"`
	StderrLogMax    int64             `yaml:"stderr_log_max_mb" json:"stderr_log_max_mb"`
	HealthCheck     *HealthCheckConfig `yaml:"healthcheck" json:"healthcheck"`
	StopSignal      string            `yaml:"stop_signal" json:"stop_signal"`
	StopTimeoutSec  int               `yaml:"stop_timeout" json:"stop_timeout"`
	Enabled         bool              `yaml:"enabled" json:"enabled"`
	ExitCodes       []int             `yaml:"exit_codes" json:"exit_codes"` // if set, only these are "clean" exits
}

// HealthCheckConfig defines health check parameters.
type HealthCheckConfig struct {
	Type     string        `yaml:"type" json:"type"` // "http", "tcp", "pid"
	Endpoint string        `yaml:"endpoint" json:"endpoint"`
	Port     int           `yaml:"port" json:"port"`
	Interval time.Duration `yaml:"interval" json:"interval"`
	Timeout  time.Duration `yaml:"timeout" json:"timeout"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		LogDir: "logs",
		Global: GlobalConfig{
			MaxRestarts:     5,
			RestartDelay:    1 * time.Second,
			BackoffFactor:   2.0,
			MaxBackoff:      30 * time.Second,
			LogMaxSize:      50, // 50 MB
			LogMaxBackups:   3,
			HealthCheckWait: 5 * time.Second,
		},
	}
}

// DefaultProcessConfig fills in defaults for a process config.
func DefaultProcessConfig(pc *ProcessConfig, global *GlobalConfig) {
	// If user explicitly disabled autorestart, respect that
	if !pc.Autorestart && pc.MaxRestarts == 0 {
		// User set autorestart: false with max_restarts: 0 - keep as-is
	} else if pc.MaxRestarts == 0 {
		pc.MaxRestarts = global.MaxRestarts
		pc.Autorestart = true
	}
	if pc.RestartDelay == 0 {
		pc.RestartDelay = global.RestartDelay
	}
	if pc.BackoffFactor == 0 {
		pc.BackoffFactor = global.BackoffFactor
	}
	if pc.MaxBackoff == 0 {
		pc.MaxBackoff = global.MaxBackoff
	}
	if pc.StopSignal == "" {
		pc.StopSignal = "TERM"
	}
	if pc.StopTimeoutSec == 0 {
		pc.StopTimeoutSec = 10
	}
	if len(pc.ExitCodes) == 0 {
		pc.ExitCodes = []int{0}
	}
	if pc.StdoutLogMax == 0 {
		pc.StdoutLogMax = global.LogMaxSize
	}
	if pc.StderrLogMax == 0 {
		pc.StderrLogMax = global.LogMaxSize
	}
}

// LoadFile reads and parses a YAML config file.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}
	return Load(data)
}

// Load parses YAML config bytes into a Config struct.
func Load(data []byte) (*Config, error) {
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.LogDir == "" {
		cfg.LogDir = "logs"
	}

	// Apply defaults to each process
	for i := range cfg.Processes {
		DefaultProcessConfig(&cfg.Processes[i], &cfg.Global)
		if cfg.Processes[i].Name == "" {
			return nil, fmt.Errorf("process at index %d has no name", i)
		}
		if cfg.Processes[i].Command == "" {
			return nil, fmt.Errorf("process %q has no command", cfg.Processes[i].Name)
		}
	}

	return cfg, nil
}

// SampleConfig returns a sample YAML configuration.
func SampleConfig() string {
	return `# ProcessWatch Configuration
# A lightweight process supervisor

# Global defaults (applied to all programs)
global:
  max_restarts: 5          # Max restarts before giving up
  restart_delay: 1s        # Delay between restart attempts
  backoff_factor: 2.0      # Exponential backoff multiplier
  max_backoff: 30s         # Maximum backoff delay
  log_max_size_mb: 50      # Max log file size in MB
  log_max_backups: 3       # Number of rotated log files to keep
  health_check_wait: 5s    # Time to wait after start before health check

# Directory for log files (relative or absolute)
log_dir: logs

# Programs to supervise
programs:
  - name: web-server
    command: python3
    args: ["-m", "http.server", "8080"]
    dir: ./public
    env:
      PORT: "8080"
      NODE_ENV: development
    autorestart: true
    max_restarts: 10
    stdout_log: web-server-stdout.log
    stderr_log: web-server-stderr.log
    healthcheck:
      type: http
      endpoint: http://localhost:8080/health
      interval: 10s
      timeout: 5s
    enabled: true

  - name: worker
    command: python3
    args: ["worker.py"]
    dir: .
    env:
      WORKERS: "4"
    autorestart: true
    max_restarts: 5
    stdout_log: worker-stdout.log
    stderr_log: worker-stderr.log
    stop_signal: INT
    stop_timeout: 15
    enabled: true

  - name: redis
    command: redis-server
    args: ["--port", "6379", "--daemonize", "no"]
    autorestart: true
    healthcheck:
      type: tcp
      port: 6379
      interval: 5s
      timeout: 2s
    enabled: false  # Disabled by default - enable when needed
`
}
