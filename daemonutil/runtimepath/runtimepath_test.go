package runtimepath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeDirJoinUsesXDGRuntimeDir(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("TMPDIR", "/tmp/ignored")

	path, err := RuntimeDirJoin("module.sock")
	if err != nil {
		t.Fatalf("RuntimeDirJoin returned error: %v", err)
	}
	if path != filepath.Join(runtimeDir, "module.sock") {
		t.Fatalf("RuntimeDirJoin returned %q", path)
	}
}

func TestRuntimeDirJoinFallsBackToTMPDIR(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMPDIR", tmpDir)

	path, err := RuntimeDirJoin("module.sock")
	if err != nil {
		t.Fatalf("RuntimeDirJoin returned error: %v", err)
	}
	if path != filepath.Join(tmpDir, "module.sock") {
		t.Fatalf("RuntimeDirJoin returned %q", path)
	}
}

func TestRuntimeDirJoinFallsBackToSlashTmp(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMPDIR", "")

	path, err := RuntimeDirJoin("module.sock")
	if err != nil {
		t.Fatalf("RuntimeDirJoin returned error: %v", err)
	}
	if path != "/tmp/module.sock" {
		t.Fatalf("RuntimeDirJoin returned %q", path)
	}
}

func TestRuntimeDirJoinRejectsRelativeXDGRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "relative")

	_, err := RuntimeDirJoin("module.sock")
	if err == nil {
		t.Fatal("RuntimeDirJoin returned nil error, want relative runtime directory error")
	}
	if !strings.Contains(err.Error(), "XDG_RUNTIME_DIR") {
		t.Fatalf("RuntimeDirJoin error = %q, want XDG_RUNTIME_DIR", err)
	}
}

func TestRuntimeDirJoinRejectsRelativeTMPDIR(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMPDIR", "relative")

	_, err := RuntimeDirJoin("module.sock")
	if err == nil {
		t.Fatal("RuntimeDirJoin returned nil error, want relative runtime directory error")
	}
	if !strings.Contains(err.Error(), "TMPDIR") {
		t.Fatalf("RuntimeDirJoin error = %q, want TMPDIR", err)
	}
}

func TestRuntimeDirJoinRejectsRuntimeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime-file")
	if err := os.WriteFile(path, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", path)

	_, err := RuntimeDirJoin("module.sock")
	if err == nil {
		t.Fatal("RuntimeDirJoin returned nil error, want runtime file error")
	}
	if strings.Contains(err.Error(), "%!") {
		t.Fatalf("RuntimeDirJoin error has bad formatting: %q", err)
	}
	if !strings.Contains(err.Error(), "must be a directory") {
		t.Fatalf("RuntimeDirJoin error = %q, want directory error", err)
	}
}

func TestWorkingDirJoinResolvesRelativePath(t *testing.T) {
	path, err := WorkingDirJoin("module.sock")
	if err != nil {
		t.Fatalf("WorkingDirJoin returned error: %v", err)
	}

	want, err := filepath.Abs("module.sock")
	if err != nil {
		t.Fatal(err)
	}
	if path != want {
		t.Fatalf("WorkingDirJoin returned %q, want %q", path, want)
	}
}
