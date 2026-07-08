package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/edmonl/waybar-pulseaudio-sources/cli"
	"github.com/edmonl/waybar-pulseaudio-sources/pulse"
	"github.com/edmonl/waybar-pulseaudio-sources/waybar"
)

const reconnectDelay = 10 * time.Minute

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
		if err := switchSource(command.Pidfile); err != nil {
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

	if command.Pidfile != "" {
		removePIDFile, err := writePIDFile(command.Pidfile)
		if err != nil {
			log.Fatal(err)
		}
		defer removePIDFile()
	}

	output := newJSONWriter(os.Stdout)

	userSignal := make(chan os.Signal, 1)
	signal.Notify(userSignal, syscall.SIGUSR1)
	defer signal.Stop(userSignal)

	pendingCycle := false
	for {
		var err error
		pendingCycle, err = runPulse(ctx, output, userSignal, pendingCycle, formatter)
		if err != nil {
			return err
		}

		cycleRequested, err := waitForReconnect(ctx, userSignal)
		if err != nil {
			return err
		}

		pendingCycle = pendingCycle || cycleRequested
	}
}

func runPulse(ctx context.Context, output *jsonWriter, userSignal <-chan os.Signal, pendingCycle bool, formatter *waybar.Formatter) (bool, error) {
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
		case <-userSignal:
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

func waitForReconnect(ctx context.Context, userSignal <-chan os.Signal) (bool, error) {
	timer := time.NewTimer(reconnectDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-userSignal:
		return true, nil
	case <-timer.C:
		return false, nil
	}
}

func switchSource(pidfile string) error {
	content, err := os.ReadFile(pidfile)
	if err != nil {
		return fmt.Errorf("failed to read pidfile: %w", err)
	}

	pidText := strings.TrimSpace(string(content))
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 0 {
		return fmt.Errorf("invalid process ID %v in pidfile", pidText)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %v: %w", pid, err)
	}
	if err := process.Signal(syscall.SIGUSR1); err != nil {
		return fmt.Errorf("failed to signal process %v: %w", pid, err)
	}

	return nil
}

func writePIDFile(path string) (func(), error) {
	if err := checkReusablePIDFile(path); err != nil {
		return nil, err
	}
	pid := strconv.Itoa(os.Getpid())
	if err := os.WriteFile(path, []byte(pid+"\n"), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write pidfile %v: %w", path, err)
	}

	return func() {
		content, err := os.ReadFile(path)
		if err != nil {
			return
		}
		if strings.TrimSpace(string(content)) == pid {
			os.Remove(path)
		}
	}, nil
}

func checkReusablePIDFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to read pidfile %v: %w", path, err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil || pid <= 0 {
		return nil
	}

	if err := syscall.Kill(pid, 0); err == nil || errors.Is(err, syscall.EPERM) {
		return fmt.Errorf("pidfile %v is already used by process %v", path, pid)
	}

	return nil
}
