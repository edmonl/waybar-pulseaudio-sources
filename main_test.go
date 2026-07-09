package main

import (
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSwitchSourceRejectsMissingSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.sock")

	err := switchSource(path)
	if err == nil {
		t.Fatal("switchSource returned nil, want missing socket error")
	}
	if !strings.Contains(err.Error(), "send switch command") {
		t.Fatalf("switchSource error = %q, want send switch command", err)
	}
}

func TestSwitchSourceSendsDatagram(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	conn, err := net.ListenPacket("unixgram", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := switchSource(path); err != nil {
		t.Fatalf("switchSource(%q) = %v, want nil", path, err)
	}

	buffer := make([]byte, 1024)
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := conn.ReadFrom(buffer)
	if err != nil {
		t.Fatal(err)
	}
	if string(buffer[:n]) != "switch" {
		t.Fatalf("datagram = %q, want switch", string(buffer[:n]))
	}
}
