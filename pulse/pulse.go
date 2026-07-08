// Package pulse provides a small CGo wrapper around libpulse for reading and
// switching PulseAudio sources.
package pulse

/*
#cgo pkg-config: libpulse
#include "pulse.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrNoInputSource reports that PulseAudio has no input source this module can
// display or switch to.
var ErrNoInputSource = errors.New("no input source available")

// Source is a PulseAudio source as seen by this program.
type Source struct {
	// Index is the PulseAudio runtime source index.
	Index uint32

	// Name is the PulseAudio source name.
	Name string

	// Description is the human-readable PulseAudio source description.
	Description string

	// Volume is the unclamped average channel volume as a percentage.
	Volume int

	// Muted reports whether the source is muted.
	Muted bool
}

// Client owns a connection to PulseAudio.
type Client struct {
	mu sync.Mutex

	ptr    *C.pulse_client_t
	ctx    context.Context
	active sync.WaitGroup

	stopContextShutdown func() bool
	contextShutdownDone chan struct{}
	closed              chan struct{}
}

// NewClient connects to PulseAudio, subscribes to source and server events,
// and permanently shuts down the client when ctx is cancelled.
func NewClient(ctx context.Context) (*Client, error) {
	ptr := C.pulse_client_new()
	if ptr == nil {
		return nil, errors.New("failed to create PulseAudio client")
	}

	client := &Client{
		ptr:                 ptr,
		ctx:                 ctx,
		contextShutdownDone: make(chan struct{}),
		closed:              make(chan struct{}),
	}
	client.stopContextShutdown = context.AfterFunc(ctx, func() {
		C.pulse_client_shutdown(ptr)
		close(client.contextShutdownDone)
	})

	if err := client.pulseError(C.pulse_client_start(ptr)); err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}

// Close shuts down the client, waits for active operations to return, and
// releases PulseAudio resources.
func (c *Client) Close() {
	c.mu.Lock()
	ptr := c.ptr
	c.ptr = nil
	if ptr == nil {
		c.mu.Unlock()
		<-c.closed
		return
	}
	c.mu.Unlock()
	defer close(c.closed)

	if c.stopContextShutdown() {
		C.pulse_client_shutdown(ptr)
		close(c.contextShutdownDone)
	} else {
		<-c.contextShutdownDone
	}

	c.active.Wait()
	C.pulse_client_free(ptr)
}

// DefaultSource returns the current default source.
func (c *Client) DefaultSource() (*Source, error) {
	ptr, err := c.beginOperation()
	if err != nil {
		return nil, err
	}
	defer c.endOperation()

	var pulseErr C.pulse_error_t
	source := C.pulse_get_default_source(ptr, &pulseErr)
	if err := c.pulseError(pulseErr); err != nil {
		return nil, err
	}
	defer C.pulse_source_free(source)

	return sourceFromC(source), nil
}

// CycleDefaultSource sets the next eligible source as the default source.
func (c *Client) CycleDefaultSource() error {
	ptr, err := c.beginOperation()
	if err != nil {
		return err
	}
	defer c.endOperation()

	return c.pulseError(C.pulse_cycle_default_source(ptr))
}

// WaitForChange blocks until PulseAudio reports a subscribed source or server event.
func (c *Client) WaitForChange() error {
	ptr, err := c.beginOperation()
	if err != nil {
		return err
	}
	defer c.endOperation()

	return c.pulseError(C.pulse_wait_for_change(ptr))
}

func (c *Client) pulseError(pulseErr C.pulse_error_t) error {
	var message string
	switch pulseErr.code {
	case C.PULSE_ERROR_NONE:
		return nil
	case C.PULSE_ERROR_CLIENT_SHUTDOWN:
		if err := c.ctx.Err(); err != nil {
			return err
		}
		message = "PulseAudio client closed"
	case C.PULSE_ERROR_CONTEXT_CONNECT:
		message = "failed to connect to PulseAudio"
	case C.PULSE_ERROR_CONTEXT_FAILED:
		message = "PulseAudio connection failed"
	case C.PULSE_ERROR_CONTEXT_NOT_READY:
		message = "PulseAudio context is not ready"
	case C.PULSE_ERROR_OPERATION_START:
		message = "failed to start PulseAudio operation"
	case C.PULSE_ERROR_OPERATION_CANCELLED:
		message = "PulseAudio operation was cancelled"
	case C.PULSE_ERROR_SUBSCRIBE:
		message = "failed to subscribe to PulseAudio events"
	case C.PULSE_ERROR_SNAPSHOT_ALLOC:
		message = "failed to allocate PulseAudio snapshot"
	case C.PULSE_ERROR_SERVER_INFO:
		message = "failed to get PulseAudio server info"
	case C.PULSE_ERROR_SOURCE_LIST:
		message = "failed to get PulseAudio source list"
	case C.PULSE_ERROR_NO_SOURCES:
		return ErrNoInputSource
	case C.PULSE_ERROR_DEFAULT_SOURCE:
		message = "failed to get PulseAudio default source"
	case C.PULSE_ERROR_SET_DEFAULT_SOURCE:
		message = "failed to set PulseAudio default source"
	default:
		message = fmt.Sprintf("unknown PulseAudio error code %v", pulseErr.code)
	}

	if pulseErr.pa_errno != 0 {
		return fmt.Errorf("%v: %v", message, C.GoString(C.pa_strerror(pulseErr.pa_errno)))
	}
	return errors.New(message)
}

func (c *Client) beginOperation() (*C.pulse_client_t, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ptr == nil {
		return nil, errors.New("PulseAudio client closed")
	}

	c.active.Add(1)
	return c.ptr, nil
}

func (c *Client) endOperation() {
	c.active.Done()
}

func sourceFromC(source *C.pulse_source_t) *Source {
	return &Source{
		Index:       uint32(source.index),
		Name:        C.GoString(source.name),
		Description: C.GoString(source.description),
		Volume:      int(source.volume_percent),
		Muted:       bool(source.mute),
	}
}
