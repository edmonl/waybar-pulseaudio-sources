package cli

import (
	"path/filepath"
	"testing"
)

func TestParseDefaultCommand(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/tmp/runtime-test")

	command, err := Parse("waybar-pulseaudio-sources", nil)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if command.SwitchSource {
		t.Fatal("SwitchSource = true, want false")
	}
	if command.Pidfile != "/tmp/runtime-test/waybar-pulseaudio-sources.pid" {
		t.Fatalf("Pidfile = %q", command.Pidfile)
	}
	if command.Text == "" || command.Class == "" || command.Tooltip == "" {
		t.Fatalf("default templates must not be empty: %#v", command)
	}
}

func TestParseSwitchCommand(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/tmp/runtime-test")

	command, err := Parse("waybar-pulseaudio-sources", []string{"switch"})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if !command.SwitchSource {
		t.Fatal("SwitchSource = false, want true")
	}
	if command.Pidfile != "/tmp/runtime-test/waybar-pulseaudio-sources.pid" {
		t.Fatalf("Pidfile = %q", command.Pidfile)
	}
}

func TestParseSwitchCommandRejectsEmptyPIDFile(t *testing.T) {
	_, err := Parse("waybar-pulseaudio-sources", []string{"switch", "--pidfile", ""})
	if err == nil {
		t.Fatal("Parse returned nil error, want empty pidfile error")
	}
}

func TestParseDefaultCommandAllowsEmptyPIDFile(t *testing.T) {
	command, err := Parse("waybar-pulseaudio-sources", []string{"--pidfile", ""})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if command.Pidfile != "" {
		t.Fatalf("Pidfile = %q, want empty", command.Pidfile)
	}
}

func TestParseResolvesRelativePIDFile(t *testing.T) {
	command, err := Parse("waybar-pulseaudio-sources", []string{"--pidfile", "module.pid"})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	want, err := filepath.Abs("module.pid")
	if err != nil {
		t.Fatal(err)
	}
	if command.Pidfile != want {
		t.Fatalf("Pidfile = %q, want %q", command.Pidfile, want)
	}
}
