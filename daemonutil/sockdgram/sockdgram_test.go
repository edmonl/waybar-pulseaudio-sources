package sockdgram

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testMaxDatagramSize = 1024
	testTimeout         = 250 * time.Millisecond
)

func TestListenBindsSocketAndReceivesDatagram(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	socket, err := Listen(path, testMaxDatagramSize)
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	defer socket.Close()

	if err := SendString(path, "payload-one", testTimeout); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	datagram, err := readStringWithTimeout(socket, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if datagram != "payload-one" {
		t.Fatalf("datagram = %q, want payload-one", datagram)
	}
}

func TestListenDoesNotReservePayloadValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	socket, err := Listen(path, testMaxDatagramSize)
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	defer socket.Close()

	if err := SendString(path, "payload-two", testTimeout); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	datagram, err := readStringWithTimeout(socket, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if datagram != "payload-two" {
		t.Fatalf("datagram = %q, want payload-two", datagram)
	}
}

func TestListenRejectsInvalidMaxDatagramSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")

	socket, err := Listen(path, 0)
	if err == nil {
		socket.Close()
		t.Fatal("Listen returned nil error, want buffer size error")
	}
	if !strings.Contains(err.Error(), "max datagram size") {
		t.Fatalf("Listen error = %q, want max datagram size", err)
	}
}

func TestListenPreservesNonSocketPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	if err := os.WriteFile(path, []byte("not a socket\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	socket, err := Listen(path, testMaxDatagramSize)
	if err == nil {
		socket.Close()
		t.Fatal("Listen returned nil error, want non-socket path error")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "not a socket\n" {
		t.Fatalf("content = %q, want original content", string(content))
	}
}

func TestListenFailsForLiveSocketWithoutRemovingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	live, err := net.ListenPacket("unixgram", path)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()

	socket, err := Listen(path, testMaxDatagramSize)
	if err == nil {
		socket.Close()
		t.Fatal("Listen returned nil error, want live socket error")
	}
	if !strings.Contains(err.Error(), "socket in use") {
		t.Fatalf("Listen error = %q, want socket in use", err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("%v is not a socket after failed Listen", path)
	}
}

func TestListenReplacesStaleSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	stale, err := net.ListenPacket("unixgram", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := stale.Close(); err != nil {
		t.Fatal(err)
	}

	socket, err := Listen(path, testMaxDatagramSize)
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	defer socket.Close()

	if err := SendString(path, "payload-three", testTimeout); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	datagram, err := readStringWithTimeout(socket, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if datagram != "payload-three" {
		t.Fatalf("datagram = %q, want payload-three", datagram)
	}
}

func readStringWithTimeout(socket *Socket, timeout time.Duration) (string, error) {
	type result struct {
		datagram string
		err      error
	}
	results := make(chan result, 1)
	go func() {
		datagram, err := socket.ReadString()
		results <- result{datagram: datagram, err: err}
	}()

	select {
	case result := <-results:
		return result.datagram, result.err
	case <-time.After(timeout):
		return "", errors.New("timed out waiting for datagram")
	}
}

func TestCloseRemovesOwnSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	socket, err := Listen(path, testMaxDatagramSize)
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}

	if err := socket.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("socket path still exists after Close: %v", err)
	}
}

func TestCloseDoesNotRemoveReplacedSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	socket, err := Listen(path, testMaxDatagramSize)
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	replacement, err := net.ListenPacket("unixgram", path)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close()

	if err := socket.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("%v is not a socket after Close", path)
	}
}

func TestCloseDoesNotRemoveReplacedNonSocketPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	socket, err := Listen(path, testMaxDatagramSize)
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := socket.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "replacement\n" {
		t.Fatalf("replacement content = %q, want original replacement", string(content))
	}
}

func TestSendMissingSocketReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.sock")

	err := SendString(path, "payload", testTimeout)
	if err == nil {
		t.Fatal("Send returned nil error, want missing socket error")
	}
}
