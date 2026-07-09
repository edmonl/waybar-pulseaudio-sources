package cli

import (
	"path/filepath"
	"testing"
)

func TestParseDefaultCommand(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

	command, err := Parse("waybar-pulseaudio-sources", nil)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if command.SwitchSource {
		t.Fatal("SwitchSource = true, want false")
	}
	if command.Sock != filepath.Join(runtimeDir, "waybar-pulseaudio-sources.sock") {
		t.Fatalf("Sock = %q", command.Sock)
	}
	if command.Text == "" || command.Class == "" || command.Tooltip == "" {
		t.Fatalf("default templates must not be empty: %#v", command)
	}
}

func TestParseSwitchCommand(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

	command, err := Parse("waybar-pulseaudio-sources", []string{"switch"})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if !command.SwitchSource {
		t.Fatal("SwitchSource = false, want true")
	}
	if command.Sock != filepath.Join(runtimeDir, "waybar-pulseaudio-sources.sock") {
		t.Fatalf("Sock = %q", command.Sock)
	}
}

func TestParseSwitchCommandRejectsEmptySocket(t *testing.T) {
	_, err := Parse("waybar-pulseaudio-sources", []string{"switch", "--sock", ""})
	if err == nil {
		t.Fatal("Parse returned nil error, want empty socket error")
	}
}

func TestParseDefaultCommandAllowsEmptySocket(t *testing.T) {
	command, err := Parse("waybar-pulseaudio-sources", []string{"--sock", ""})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if command.Sock != "" {
		t.Fatalf("Sock = %q, want empty", command.Sock)
	}
}

func TestParseResolvesRelativeSocket(t *testing.T) {
	command, err := Parse("waybar-pulseaudio-sources", []string{"--sock", "module.sock"})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	want, err := filepath.Abs("module.sock")
	if err != nil {
		t.Fatal(err)
	}
	if command.Sock != want {
		t.Fatalf("Sock = %q, want %q", command.Sock, want)
	}
}
