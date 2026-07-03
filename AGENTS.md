# AGENTS.md - ProcessWatch

## What This Project Is
ProcessWatch is a lightweight, cross-platform process supervisor written in Go. It manages background processes with YAML configuration, auto-restart, log capture, and health checks.

## Architecture
- `cmd/processwatch/main.go` - CLI entry point with command parsing
- `internal/config/config.go` - YAML configuration parsing and validation
- `internal/process/process.go` - Individual process lifecycle management
- `internal/manager/manager.go` - Multi-process orchestration and signal handling
- `internal/healthcheck/healthcheck.go` - HTTP/TCP health check implementations
- `internal/log/logger.go` - Log capture and rotation

## Key Design Decisions
- Single binary with zero runtime dependencies
- Goroutine-based process monitoring (one per process)
- Mutex-based state synchronization for concurrent access
- Exponential backoff for crash recovery
- Single Wait() call per process to avoid race conditions

## Development Commands
```bash
# Build
go build -o processwatch ./cmd/processwatch

# Run tests
go test ./... -v

# Run specific package tests
go test ./internal/config/ -v
go test ./internal/process/ -v
go test ./internal/manager/ -v

# Lint
go vet ./...
```

## Adding New Features
1. Add to appropriate internal package
2. Add tests in `*_test.go` files
3. Update CLI commands in `cmd/processwatch/main.go`
4. Update README.md documentation
