// Package healthcheck provides health checking for supervised processes.
package healthcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/EdgarOrtegaRamirez/processwatch/internal/config"
)

// Checker performs health checks on a process.
type Checker struct {
	config *config.HealthCheckConfig
	client *http.Client
}

// New creates a new health Checker from config.
func New(cfg *config.HealthCheckConfig) *Checker {
	if cfg == nil {
		return &Checker{config: nil}
	}
	timeout := 5 * time.Second
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}
	return &Checker{
		config: cfg,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Check performs a single health check. Returns true if the process is healthy.
func (c *Checker) Check(ctx context.Context) (bool, error) {
	if c.config == nil {
		return true, nil
	}

	switch c.config.Type {
	case "http":
		return c.checkHTTP(ctx)
	case "tcp":
		return c.checkTCP(ctx)
	case "pid":
		return true, nil // PID check is handled by the process manager
	default:
		return true, fmt.Errorf("unknown health check type: %s", c.config.Type)
	}
}

func (c *Checker) checkHTTP(ctx context.Context) (bool, error) {
	url := c.config.Endpoint
	if url == "" {
		return false, fmt.Errorf("no endpoint configured for HTTP health check")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("creating health check request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, nil // Connection failed = unhealthy
	}
	resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 400, nil
}

func (c *Checker) checkTCP(ctx context.Context) (bool, error) {
	if c.config.Port <= 0 {
		return false, fmt.Errorf("no port configured for TCP health check")
	}

	host := "localhost"
	if c.config.Endpoint != "" {
		host = c.config.Endpoint
	}

	addr := fmt.Sprintf("%s:%d", host, c.config.Port)

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false, nil // Connection failed = unhealthy
	}
	conn.Close()

	return true, nil
}

// Interval returns the check interval (default 10s).
func (c *Checker) Interval() time.Duration {
	if c.config != nil && c.config.Interval > 0 {
		return c.config.Interval
	}
	return 10 * time.Second
}
