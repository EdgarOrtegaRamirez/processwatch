package healthcheck

import (
	"context"
	"testing"
	"time"

	"github.com/EdgarOrtegaRamirez/processwatch/internal/config"
)

func TestNewChecker(t *testing.T) {
	cfg := &config.HealthCheckConfig{
		Type:     "http",
		Endpoint: "http://localhost:8080",
		Interval: 5 * time.Second,
		Timeout:  3 * time.Second,
	}

	checker := New(cfg)
	if checker == nil {
		t.Fatal("expected non-nil checker")
	}
	if checker.Interval() != 5*time.Second {
		t.Errorf("expected interval 5s, got %v", checker.Interval())
	}
}

func TestCheckNilConfig(t *testing.T) {
	checker := New(nil)
	ok, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("nil config should always be healthy")
	}
}

func TestCheckUnknownType(t *testing.T) {
	cfg := &config.HealthCheckConfig{
		Type: "unknown",
	}
	checker := New(cfg)
	_, err := checker.Check(context.Background())
	if err == nil {
		t.Error("expected error for unknown health check type")
	}
}

func TestCheckHTTP(t *testing.T) {
	cfg := &config.HealthCheckConfig{
		Type:     "http",
		Endpoint: "http://localhost:99999/nonexistent",
		Timeout:  1 * time.Second,
	}
	checker := New(cfg)
	ok, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected unhealthy for non-existent HTTP endpoint")
	}
}

func TestCheckHTTPNoEndpoint(t *testing.T) {
	cfg := &config.HealthCheckConfig{
		Type: "http",
	}
	checker := New(cfg)
	_, err := checker.Check(context.Background())
	if err == nil {
		t.Error("expected error for missing HTTP endpoint")
	}
}

func TestCheckTCP(t *testing.T) {
	cfg := &config.HealthCheckConfig{
		Type: "tcp",
		Port: 99999,
	}
	checker := New(cfg)
	ok, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected unhealthy for non-existent TCP port")
	}
}

func TestCheckTCPNoPort(t *testing.T) {
	cfg := &config.HealthCheckConfig{
		Type: "tcp",
	}
	checker := New(cfg)
	_, err := checker.Check(context.Background())
	if err == nil {
		t.Error("expected error for missing TCP port")
	}
}

func TestCheckPID(t *testing.T) {
	cfg := &config.HealthCheckConfig{
		Type: "pid",
	}
	checker := New(cfg)
	ok, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("PID check should always be healthy (managed by process)")
	}
}

func TestDefaultInterval(t *testing.T) {
	cfg := &config.HealthCheckConfig{
		Type: "http",
	}
	checker := New(cfg)
	if checker.Interval() != 10*time.Second {
		t.Errorf("expected default interval 10s, got %v", checker.Interval())
	}
}
