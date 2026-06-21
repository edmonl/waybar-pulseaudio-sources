// Package pulse provides a small CGo wrapper around libpulse for reading and
// switching PulseAudio sources.
package pulse

/*
#cgo pkg-config: libpulse
#include <stdlib.h>
#include "pulse.h"
*/
import "C"

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"unsafe"
)

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
	ptr *C.pulse_client_t
}

// NewClient connects to PulseAudio and subscribes to source and server events.
func NewClient() (*Client, error) {
	ptr := C.pulse_client_new()
	if ptr == nil {
		return nil, errors.New("failed to allocate PulseAudio client")
	}

	client := &Client{ptr: ptr}
	if err := pulseError(C.pulse_client_connect(ptr)); err != nil {
		client.Close()
		return nil, err
	}
	if err := pulseError(C.pulse_client_subscribe(ptr)); err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}

// Close disconnects from PulseAudio and releases client resources.
func (c *Client) Close() {
	if c.ptr == nil {
		return
	}

	C.pulse_client_free(c.ptr)
	c.ptr = nil
}

// DefaultSource returns the current default source.
//
// A nil source with a nil error means PulseAudio did not report a usable
// default source.
func (c *Client) DefaultSource() (*Source, error) {
	ptr := c.mustPtr()

	var pulseErr C.pulse_error_t
	source := C.pulse_get_default_source(ptr, &pulseErr)
	if err := pulseError(pulseErr); err != nil {
		return nil, err
	}
	if source == nil {
		return nil, nil
	}
	defer C.pulse_source_free(source)

	return sourceFromC(source), nil
}

// CycleDefaultSource sets the next eligible source as the default source.
func (c *Client) CycleDefaultSource() error {
	var pulseErr C.pulse_error_t
	snapshot := C.pulse_get_sources(c.mustPtr(), &pulseErr)
	if err := pulseError(pulseErr); err != nil {
		return err
	}
	defer C.pulse_snapshot_free(snapshot)

	count := int(snapshot.count)
	if count == 0 {
		return errors.New("no PulseAudio sources available")
	}

	var defaultSourceName string
	if snapshot.default_source_name != nil {
		defaultSourceName = C.GoString(snapshot.default_source_name)
	}

	sources := make([]*Source, 0, count)
	for _, source := range unsafe.Slice(snapshot.sources, count) {
		sources = append(sources, sourceFromC(&source))
	}

	if len(sources) == 0 {
		return errors.New("no PulseAudio sources available")
	}

	slices.SortStableFunc(sources, func(a, b *Source) int {
		return cmp.Compare(a.Index, b.Index)
	})

	next := 0
	for i, source := range sources {
		if source.Name == defaultSourceName {
			next = (i + 1) % len(sources)
			break
		}
	}

	cName := C.CString(sources[next].Name)
	defer C.free(unsafe.Pointer(cName))

	if err := pulseError(C.pulse_set_default_source(c.mustPtr(), cName)); err != nil {
		return err
	}

	return nil
}

// WaitForChange blocks until PulseAudio reports a subscribed source or server event.
func (c *Client) WaitForChange() error {
	return pulseError(C.pulse_wait_for_change(c.mustPtr()))
}

// Wakeup unblocks a goroutine waiting in WaitForChange.
func (c *Client) Wakeup() {
	C.pulse_wakeup(c.mustPtr())
}

func pulseError(pulseErr C.pulse_error_t) error {
	if pulseErr.code == C.PULSE_ERROR_NONE {
		return nil
	}

	var message string
	switch pulseErr.code {
	case C.PULSE_ERROR_MAINLOOP_NEW:
		message = "failed to create PulseAudio mainloop"
	case C.PULSE_ERROR_CONTEXT_NEW:
		message = "failed to create PulseAudio context"
	case C.PULSE_ERROR_CONTEXT_CONNECT:
		message = "failed to connect to PulseAudio"
	case C.PULSE_ERROR_MAINLOOP_START:
		message = "failed to start PulseAudio mainloop"
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

func (c *Client) mustPtr() *C.pulse_client_t {
	if c.ptr == nil {
		panic("pulse: closed Client")
	}

	return c.ptr
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
