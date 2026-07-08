package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSwitchSourceRejectsMissingPIDFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.pid")

	err := switchSource(path)
	if err == nil {
		t.Fatal("switchSource returned nil, want missing pidfile error")
	}
	if !strings.Contains(err.Error(), "read pidfile") {
		t.Fatalf("switchSource error = %q, want read pidfile", err)
	}
}

func TestSwitchSourceRejectsInvalidPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.pid")
	if err := os.WriteFile(path, []byte("not-a-pid\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := switchSource(path)
	if err == nil {
		t.Fatal("switchSource returned nil, want invalid PID error")
	}
	if !strings.Contains(err.Error(), "invalid process ID") {
		t.Fatalf("switchSource error = %q, want invalid process ID", err)
	}
}

func TestSwitchSourceSignalsPID(t *testing.T) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)
	defer signal.Stop(signals)

	path := filepath.Join(t.TempDir(), "self.pid")
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := switchSource(path); err != nil {
		t.Fatalf("switchSource(%q) = %v, want nil", path, err)
	}

	select {
	case signal := <-signals:
		if signal != syscall.SIGUSR1 {
			t.Fatalf("got signal %v, want %v", signal, syscall.SIGUSR1)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for SIGUSR1")
	}
}

func TestSwitchSourceRejectsStalePID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.pid")
	if err := os.WriteFile(path, []byte("4194303\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := switchSource(path)
	if err == nil {
		t.Fatal("switchSource returned nil, want stale PID error")
	}
	if !strings.Contains(err.Error(), "signal process") {
		t.Fatalf("switchSource error = %q, want signal process", err)
	}
}

func TestWritePIDFileRejectsLivePID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.pid")
	oldContent := strconv.Itoa(os.Getpid()) + "\n"
	if err := os.WriteFile(path, []byte(oldContent), 0o644); err != nil {
		t.Fatal(err)
	}

	removePIDFile, err := writePIDFile(path)
	if err == nil {
		removePIDFile()
		t.Fatal("writePIDFile returned nil, want live PID error")
	}
	if !strings.Contains(err.Error(), "already used by process") {
		t.Fatalf("writePIDFile error = %q, want live PID error", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != oldContent {
		t.Fatalf("pidfile content = %q, want %q", string(content), oldContent)
	}
}

func TestWritePIDFileOverwritesStalePID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.pid")
	if err := os.WriteFile(path, []byte("99999999\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	removePIDFile, err := writePIDFile(path)
	if err != nil {
		t.Fatalf("writePIDFile(%q) = %v, want nil", path, err)
	}
	defer removePIDFile()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(content)) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("pidfile content = %q, want current PID", string(content))
	}
}
