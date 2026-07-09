// Package sockdgram provides a small Unix datagram control socket.
package sockdgram

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

const endpointCheckTimeout = 250 * time.Millisecond

var errNotSocket = errors.New("not a socket")

type fileID struct {
	dev uint64
	ino uint64
}

// Socket is a Unix datagram socket with filesystem cleanup.
type Socket struct {
	path    string
	readBuf []byte

	// bind
	conn net.PacketConn
	id   fileID

	// close
	closeOnce sync.Once
}

// Listen binds path as a Unix datagram socket.
func Listen(path string, maxDatagramSize int) (*Socket, error) {
	if maxDatagramSize <= 0 {
		return nil, errors.New("max datagram size must be positive")
	}
	sock := &Socket{
		path:    path,
		readBuf: make([]byte, maxDatagramSize),
	}

	if err := bind(sock); err == nil {
		return sock, nil
	} else if !errors.Is(err, syscall.EADDRINUSE) {
		return nil, fmt.Errorf("failed to bind socket: %w", err)
	}

	if info, err := os.Lstat(path); err != nil {
		return nil, fmt.Errorf("failed to inspect existing socket: %w", err)
	} else if info.Mode()&os.ModeSocket == 0 {
		return nil, errNotSocket
	}

	if live, err := isLiveEndpoint(path); err != nil {
		return nil, fmt.Errorf("failed to check socket: %w", err)
	} else if live {
		return nil, errors.New("socket in use")
	}

	if err := os.Remove(path); err != nil {
		return nil, fmt.Errorf("failed to remove stale socket: %w", err)
	}

	if err := bind(sock); err != nil {
		return nil, fmt.Errorf("failed to bind socket: %w", err)
	}

	return sock, nil
}

func bind(sock *Socket) error {
	conn, err := net.ListenPacket("unixgram", sock.path)
	if err != nil {
		return err
	}

	id, err := socketFileID(sock.path)
	if err != nil {
		conn.Close()
		return err
	}

	sock.conn = conn
	sock.id = id
	return nil
}

func isLiveEndpoint(path string) (bool, error) {
	dialer := net.Dialer{Timeout: endpointCheckTimeout}

	conn, err := dialer.Dial("unixgram", path)
	if err == nil {
		conn.Close()
		return true, nil
	}

	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ENOENT) ||
		errors.Is(err, syscall.ECONNRESET) {
		err = nil
	}
	return false, err
}

// ReadString reads one datagram payload as a string.
func (s *Socket) ReadString() (string, error) {
	n, _, err := s.conn.ReadFrom(s.readBuf)
	if err != nil {
		return "", err
	}
	return string(s.readBuf[:n]), nil
}

// Close closes the socket and removes the socket path if it still belongs to this socket.
func (s *Socket) Close() error {
	var err error
	s.closeOnce.Do(func() {
		closeErr := s.conn.Close()
		err = removeIfSameSocket(s.path, s.id)
		if err == nil {
			err = closeErr
		}
	})
	return err
}

// SendString sends one string datagram to path with timeout as the connect and write deadline.
func SendString(path string, datagram string, timeout time.Duration) error {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.Dial("unixgram", path)
	if err != nil {
		return fmt.Errorf("failed to dial socket: %w", err)
	}
	defer conn.Close()

	if err = conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("failed to set socket write deadline: %w", err)
	}

	if _, err = conn.Write([]byte(datagram)); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}
	return nil
}

func socketFileID(path string) (fileID, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return fileID{}, fmt.Errorf("failed to inspect socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fileID{}, errNotSocket
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileID{}, errors.New("failed to inspect socket identity")
	}
	return fileID{dev: uint64(stat.Dev), ino: uint64(stat.Ino)}, nil
}

func removeIfSameSocket(path string, want fileID) error {
	got, err := socketFileID(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, errNotSocket) {
			return nil
		}
		return err
	}
	if got != want {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove socket : %w", err)
	}
	return nil
}
