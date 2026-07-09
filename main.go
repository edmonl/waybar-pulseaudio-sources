package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/edmonl/waybar-pulseaudio-sources/cli"
	"github.com/edmonl/waybar-pulseaudio-sources/daemonutil/sockdgram"
	"github.com/edmonl/waybar-pulseaudio-sources/pulse"
	"github.com/edmonl/waybar-pulseaudio-sources/waybar"
)

const (
	switchCommand          = "switch"
	controlMaxDatagramSize = 8
	controlSendTimeout     = 250 * time.Millisecond
	reconnectDelay         = 10 * time.Minute
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("waybar-pulseaudio-sources: ")

	command, err := cli.Parse(filepath.Base(os.Args[0]), os.Args[1:])
	if err != nil {
		if cli.IsHelp(err) {
			return
		}
		log.Fatal(err)
	}
	if command.SwitchSource {
		if err := switchSource(command.Sock); err != nil {
			log.Fatal(err)
		}
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, command); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

func run(ctx context.Context, command cli.Command) error {
	formatter, err := waybar.NewFormatter(command.Text, command.Class, command.Tooltip)
	if err != nil {
		return err
	}

	var controlRequests <-chan struct{}
	if command.Sock != "" {
		controlSocket, err := sockdgram.Listen(command.Sock, controlMaxDatagramSize)
		if err != nil {
			return err
		}
		defer controlSocket.Close()
		controlRequests = readControlRequests(controlSocket)
	}

	output := newJSONWriter(os.Stdout)

	pendingCycle := false
	for {
		var err error
		pendingCycle, err = runPulse(ctx, output, controlRequests, pendingCycle, formatter)
		if err != nil {
			return err
		}

		cycleRequested, err := waitForReconnect(ctx, controlRequests)
		if err != nil {
			return err
		}

		pendingCycle = pendingCycle || cycleRequested
	}
}

func runPulse(ctx context.Context, output *jsonWriter, controlRequests <-chan struct{}, pendingCycle bool, formatter *waybar.Formatter) (bool, error) {
	client, err := pulse.NewClient(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return pendingCycle, err
		}
		return pendingCycle, output.Emit(formatter.Unavailable(err))
	}
	defer client.Close()

	changes, waitErrors, stopWatching := watchPulse(client)
	defer stopWatching()

	output.Reset()
	for {
		if pendingCycle {
			if err := client.CycleDefaultSource(); err != nil {
				if errors.Is(err, context.Canceled) {
					return pendingCycle, err
				}
				var state waybar.Output
				if errors.Is(err, pulse.ErrNoInputSource) {
					state = formatter.Unavailable(err)
				} else {
					state = formatter.Error(err)
				}
				if e := output.EmitIfChanged(state); e != nil {
					return pendingCycle, e
				}
			} else {
				state, err := getWaybarOutput(client, formatter)
				if err != nil {
					return pendingCycle, err
				}
				if err := output.Emit(state); err != nil {
					return pendingCycle, err
				}
			}

			pendingCycle = false
		} else {
			state, err := getWaybarOutput(client, formatter)
			if err != nil {
				return pendingCycle, err
			}
			if err := output.EmitIfChanged(state); err != nil {
				return pendingCycle, err
			}
		}

		select {
		case <-ctx.Done():
			return pendingCycle, ctx.Err()
		case err := <-waitErrors:
			if errors.Is(err, context.Canceled) {
				return pendingCycle, err
			}
			return pendingCycle, output.Emit(formatter.Unavailable(err))
		case <-controlRequests:
			pendingCycle = true
		case <-changes:
		}
	}
}

func getWaybarOutput(client *pulse.Client, formatter *waybar.Formatter) (waybar.Output, error) {
	source, err := client.DefaultSource()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return waybar.Output{}, err
		}
		if errors.Is(err, pulse.ErrNoInputSource) {
			return formatter.Unavailable(err), nil
		}
		return formatter.Error(err), nil
	}

	return formatter.State(source), nil
}

func watchPulse(client *pulse.Client) (<-chan struct{}, <-chan error, func()) {
	changes := make(chan struct{}, 1)
	waitErrors := make(chan error, 1)
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			err := client.WaitForChange()
			select {
			case <-stop:
				return
			default:
			}

			if err != nil {
				select {
				case waitErrors <- err:
				default:
				}
				return
			}

			select {
			case changes <- struct{}{}:
			default:
			}
		}
	}()

	var once sync.Once
	stopWatching := func() {
		once.Do(func() {
			close(stop)
			client.Close()
			<-done
		})
	}

	return changes, waitErrors, stopWatching
}

func waitForReconnect(ctx context.Context, controlRequests <-chan struct{}) (bool, error) {
	timer := time.NewTimer(reconnectDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-controlRequests:
		return true, nil
	case <-timer.C:
		return false, nil
	}
}

func readControlRequests(socket *sockdgram.Socket) <-chan struct{} {
	requests := make(chan struct{}, 1)
	go func() {
		// Do not close requests here. It would probably be harmless in the
		// current shutdown path because run is already returning, but it is
		// unnecessary and fragile: receivers use one-value receives, so a closed
		// channel would look like an endless stream of requests if the lifecycle
		// changes.
		for {
			packet, err := socket.ReadString()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				log.Printf("failed to read command from socket: %v", err)
				continue
			}
			if packet != switchCommand {
				continue
			}
			select {
			case requests <- struct{}{}:
			default:
			}
		}
	}()
	return requests
}

func switchSource(sock string) error {
	if err := sockdgram.SendString(sock, switchCommand, controlSendTimeout); err != nil {
		return fmt.Errorf("failed to send switch command: %w", err)
	}
	return nil
}
