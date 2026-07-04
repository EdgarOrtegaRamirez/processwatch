// Package log provides log capture and rotation for supervised processes.
package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Logger captures output from a supervised process with optional file rotation.
type Logger struct {
	name     string
	dir      string
	stdout   io.WriteCloser
	stderr   io.WriteCloser
	fileOut  *os.File
	fileErr  *os.File
	mu       sync.Mutex
	closed   bool
	bytesOut int64
	bytesErr int64
	maxSize  int64 // max bytes before rotation (0 = no rotation)
}

// New creates a new Logger for the given process name.
// If logDir is empty, output goes to os.Stdout/Stderr.
// If stdoutLog/stderrLog are provided, they're written to files in logDir.
func New(name, logDir, stdoutLog, stderrLog string, maxSizeMB int64) (*Logger, error) {
	l := &Logger{
		name:    name,
		dir:     logDir,
		maxSize: maxSizeMB * 1024 * 1024,
	}

	if stdoutLog != "" && logDir != "" {
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating log dir: %w", err)
		}
		path := filepath.Join(logDir, stdoutLog)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("opening stdout log %s: %w", path, err)
		}
		l.fileOut = f
		l.stdout = f
	} else {
		l.stdout = &nopCloser{os.Stdout}
	}

	if stderrLog != "" && logDir != "" {
		if logDir != "" {
			if err := os.MkdirAll(logDir, 0o755); err != nil && l.fileOut == nil {
				return nil, fmt.Errorf("creating log dir: %w", err)
			}
		}
		path := filepath.Join(logDir, stderrLog)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("opening stderr log %s: %w", path, err)
		}
		l.fileErr = f
		l.stderr = f
	} else {
		l.stderr = &nopCloser{os.Stderr}
	}

	return l, nil
}

// StdoutWriter returns the writer for stdout.
func (l *Logger) StdoutWriter() io.WriteCloser {
	return l.stdout
}

// StderrWriter returns the writer for stderr.
func (l *Logger) StderrWriter() io.WriteCloser {
	return l.stderr
}

// WriteToStdout writes a line to the stdout log.
func (l *Logger) WriteToStdout(data []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n, err := l.stdout.Write(data)
	l.bytesOut += int64(n)
	l.checkRotate(l.fileOut, &l.bytesOut, l.name+".stdout")
	return n, err
}

// WriteToStderr writes a line to the stderr log.
func (l *Logger) WriteToStderr(data []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n, err := l.stderr.Write(data)
	l.bytesErr += int64(n)
	l.checkRotate(l.fileErr, &l.bytesErr, l.name+".stderr")
	return n, err
}

// Rotate forces rotation of log files if they exceed max size.
func (l *Logger) Rotate() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.checkRotate(l.fileOut, &l.bytesOut, l.name+".stdout")
	l.checkRotate(l.fileErr, &l.bytesErr, l.name+".stderr")
	return nil
}

func (l *Logger) checkRotate(f *os.File, bytes *int64, label string) {
	if f == nil || l.maxSize <= 0 || *bytes < l.maxSize {
		return
	}
	// Simple rotation: rename .log to .1.log, create new .log
	name := f.Name()
	rotated := name + ".1"

	// Remove old rotated file
	_ = os.Remove(rotated)

	// Close current, rename, reopen
	_ = f.Close()
	_ = os.Rename(name, rotated)

	newF, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] failed to rotate log %s: %v\n", label, name, err)
		return
	}

	// Update the appropriate file handle
	if label == l.name+".stdout" {
		l.fileOut = newF
		l.stdout = newF
	} else {
		l.fileErr = newF
		l.stderr = newF
	}
	*bytes = 0
}

// Close closes log files.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	var firstErr error
	if l.fileOut != nil {
		if err := l.fileOut.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if l.fileErr != nil && l.fileErr != l.fileOut {
		if err := l.fileErr.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// nopCloser wraps a Writer and adds a no-op Close method.
type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }
