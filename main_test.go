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
