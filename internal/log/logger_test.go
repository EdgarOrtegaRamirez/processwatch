package log

import (
	"os"
	"testing"
)

func TestNewLoggerToFile(t *testing.T) {
	dir := t.TempDir()
	l, err := New("test", dir, "stdout.log", "stderr.log", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	// Write to stdout
	n, err := l.WriteToStdout([]byte("hello stdout\n"))
	if err != nil {
		t.Fatalf("unexpected error writing stdout: %v", err)
	}
	if n != 13 {
		t.Errorf("expected 13 bytes written, got %d", n)
	}

	// Write to stderr
	n, err = l.WriteToStderr([]byte("hello stderr\n"))
	if err != nil {
		t.Fatalf("unexpected error writing stderr: %v", err)
	}
	if n != 13 {
		t.Errorf("expected 13 bytes written, got %d", n)
	}

	// Verify files exist
	if _, err := os.Stat(dir + "/stdout.log"); os.IsNotExist(err) {
		t.Error("stdout.log should exist")
	}
	if _, err := os.Stat(dir + "/stderr.log"); os.IsNotExist(err) {
		t.Error("stderr.log should exist")
	}
}

func TestNewLoggerToStdout(t *testing.T) {
	l, err := New("test", "", "", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	// Should not error when writing to stdout
	n, err := l.WriteToStdout([]byte("hello\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 6 {
		t.Errorf("expected 6 bytes, got %d", n)
	}
}

func TestNewLoggerNoLogDir(t *testing.T) {
	l, err := New("test", "", "stdout.log", "stderr.log", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	// Without log dir, should write to os.Stdout/Stderr
	n, err := l.WriteToStdout([]byte("test\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes, got %d", n)
	}
}

func TestLoggerClose(t *testing.T) {
	dir := t.TempDir()
	l, err := New("test", dir, "out.log", "err.log", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := l.Close(); err != nil {
		t.Errorf("unexpected error closing: %v", err)
	}

	// Double close should be safe
	if err := l.Close(); err != nil {
		t.Errorf("unexpected error on double close: %v", err)
	}
}

func TestLoggerStdoutWriter(t *testing.T) {
	l, err := New("test", "", "", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	w := l.StdoutWriter()
	if w == nil {
		t.Fatal("expected non-nil stdout writer")
	}

	n, err := w.Write([]byte("hello\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 6 {
		t.Errorf("expected 6 bytes, got %d", n)
	}
}

func TestLoggerStderrWriter(t *testing.T) {
	l, err := New("test", "", "", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	w := l.StderrWriter()
	if w == nil {
		t.Fatal("expected non-nil stderr writer")
	}

	n, err := w.Write([]byte("error\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 6 {
		t.Errorf("expected 6 bytes, got %d", n)
	}
}

func TestLoggerRotation(t *testing.T) {
	dir := t.TempDir()
	l, err := New("test", dir, "out.log", "err.log", 0) // 0 = no rotation
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	// Write some data
	l.WriteToStdout([]byte("test data\n"))

	// Rotate should not error even with 0 max size
	if err := l.Rotate(); err != nil {
		t.Errorf("unexpected error rotating: %v", err)
	}
}

func TestLoggerLargeMaxSize(t *testing.T) {
	dir := t.TempDir()
	l, err := New("test", dir, "out.log", "err.log", 1000) // 1GB max
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	// Write data - should not trigger rotation
	for i := 0; i < 100; i++ {
		l.WriteToStdout([]byte("test line\n"))
	}

	l.Rotate()
	// File should still be the same
}

func TestNewLoggerCreateDir(t *testing.T) {
	dir := t.TempDir() + "/nested/deep/logs"
	l, err := New("test", dir, "out.log", "err.log", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	// Verify directory was created
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("log directory should have been created")
	}
}
