# ProcessWatch

A lightweight, cross-platform process supervisor written in Go. Monitor, manage, and auto-restart background processes with YAML configuration.

## Features

- **YAML Configuration** - Define processes in a simple YAML file
- **Auto-Restart** - Exponential backoff on crash recovery
- **Log Capture** - Capture stdout/stderr to files with rotation
- **Health Checks** - HTTP endpoint and TCP port health monitoring
- **Status Dashboard** - Real-time status with colorized output
- **Signal Forwarding** - Configurable stop signals (TERM, INT, HUP, etc.)
- **Environment Variables** - Per-process env var injection
- **Cross-Platform** - Works on Linux, macOS, and Windows
- **Zero Dependencies** - Single binary, no runtime requirements

## Quick Start

```bash
# Install
go install github.com/EdgarOrtegaRamirez/processwatch/cmd/processwatch@latest

# Generate sample config
processwatch init

# Edit processwatch.yml, then start
processwatch start
```

## Commands

```
processwatch start [config.yml]     Start all processes
processwatch stop                   Stop all processes
processwatch restart <name>         Restart a specific process
processwatch status                 Show status of all processes
processwatch status --json          Show status as JSON
processwatch init                   Generate sample config
processwatch version                Show version
```

## Configuration

```yaml
# Global defaults
global:
  max_restarts: 5          # Max restarts before giving up
  restart_delay: 1s        # Delay between restart attempts
  backoff_factor: 2.0      # Exponential backoff multiplier
  max_backoff: 30s         # Maximum backoff delay
  log_max_size_mb: 50      # Max log file size in MB
  log_max_backups: 3       # Number of rotated log files to keep
  health_check_wait: 5s    # Time to wait before health check

# Directory for log files
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
    autorestart: true
    stop_signal: INT
    stop_timeout: 15
    enabled: true
```

## Process Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `name` | string | **required** | Process identifier |
| `command` | string | **required** | Command to execute |
| `args` | []string | `[]` | Command arguments |
| `dir` | string | `.` | Working directory |
| `env` | map | `{}` | Environment variables |
| `autorestart` | bool | `true` | Auto-restart on exit |
| `max_restarts` | int | `5` | Max restart attempts |
| `restart_delay` | duration | `1s` | Delay before restart |
| `backoff_factor` | float | `2.0` | Backoff multiplier |
| `max_backoff` | duration | `30s` | Maximum backoff |
| `stdout_log` | string | - | Stdout log filename |
| `stderr_log` | string | - | Stderr log filename |
| `stop_signal` | string | `TERM` | Signal for stopping |
| `stop_timeout` | int | `10` | Seconds to wait before force kill |
| `enabled` | bool | `true` | Whether to start this process |
| `exit_codes` | []int | `[0]` | Considered "clean" exit codes |

## Health Checks

ProcessWatch supports HTTP and TCP health checks:

```yaml
healthcheck:
  type: http           # http, tcp, or pid
  endpoint: http://localhost:8080/health
  interval: 10s
  timeout: 5s
```

For TCP checks:
```yaml
healthcheck:
  type: tcp
  port: 6379
  interval: 5s
```

## Status Output

```
NAME                 STATE        PID      UPTIME     RESTARTS HEALTH
--------------------------------------------------------------------------------
web-server           RUNNING      12345    2h30m      0        OK
worker               RUNNING      12346    2h30m      2        OK
redis                STOPPED      -        -          0        -
```

## Architecture

```
processwatch/
├── cmd/processwatch/      # CLI entry point
├── internal/
│   ├── config/            # YAML config parser
│   ├── process/           # Individual process management
│   ├── manager/           # Multi-process orchestration
│   ├── healthcheck/       # HTTP/TCP health checks
│   └── log/               # Log capture and rotation
└── tests/                 # Integration tests
```

## License

MIT
