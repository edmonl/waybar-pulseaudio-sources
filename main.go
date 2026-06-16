package main

import (
	"context"
	"errors"
	"flag"
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

	"github.com/edmonl/waybar-pulseaudio-sources/pulse"
)

const (
	reconnectDelay = 10 * time.Minute
	pidfileName    = "waybar-pulseaudio-sources.pid"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("waybar-pulseaudio-sources: ")

	pidfile := ""
	flag.StringVar(&pidfile, "pidfile", defaultPIDFile(), "write the process ID to this file")
	flag.Parse()

	pidfile, err := normalizePIDFile(pidfile)
	if err != nil {
		log.Fatal(err)
	}

	removePIDFile, err := writePIDFile(pidfile)
	if err != nil {
		log.Fatal(err)
	}
	defer removePIDFile()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	output := newJSONWriter(os.Stdout)

	userSignal := make(chan os.Signal, 1)
	signal.Notify(userSignal, syscall.SIGUSR1)
	defer signal.Stop(userSignal)

	pendingCycle := false
	for {
		client, err := pulse.NewClient()
		if err != nil {
			if e := output.Emit(waybarUnavailable(err)); e != nil {
				return e
			}
			cycleRequested, err := waitForReconnect(ctx, userSignal)
			if err != nil {
				return err
			}
			pendingCycle = pendingCycle || cycleRequested
			continue
		}

		changes, waitErrors, stopWatching := watchPulse(client)
		if pendingCycle {
			pendingCycle = false
			if err := client.CycleDefaultSource(); err != nil {
				if err := output.Emit(waybarError(err)); err != nil {
					return err
				}
			}
		}
		if err := output.Emit(getWaybarOutput(client)); err != nil {
			return err
		}

		reconnect := false
		for !reconnect {
			select {
			case <-ctx.Done():
				stopWatching()
				client.Close()
				return ctx.Err()
			case <-changes:
				if err := output.EmitIfChanged(getWaybarOutput(client)); err != nil {
					return err
				}
			case err := <-waitErrors:
				if err := output.Emit(waybarUnavailable(err)); err != nil {
					return err
				}
				stopWatching()
				client.Close()
				cycleRequested, err := waitForReconnect(ctx, userSignal)
				if err != nil {
					return err
				}
				pendingCycle = pendingCycle || cycleRequested
				reconnect = true
			case <-userSignal:
				if err := client.CycleDefaultSource(); err != nil {
					if err := output.Emit(waybarError(err)); err != nil {
						return err
					}
					continue
				}
				if err := output.Emit(getWaybarOutput(client)); err != nil {
					return err
				}
			}
		}
	}
}

func getWaybarOutput(client *pulse.Client) any {
	source, err := client.DefaultSource()
	if err != nil {
		return waybarError(err)
	}
	if source == nil {
		return waybarDefaultSourceNotFound()
	}

	return waybarState(source)
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
			client.Wakeup()
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

func defaultPIDFile() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return ""
	}

	return filepath.Join(runtimeDir, pidfileName)
}

func normalizePIDFile(path string) (string, error) {
	path = strings.TrimSpace(os.ExpandEnv(path))
	if path == "" {
		return "", errors.New("XDG_RUNTIME_DIR is empty and --pidfile was not provided")
	}
	return path, nil
}

func writePIDFile(path string) (func(), error) {
	pid := strconv.Itoa(os.Getpid())
	if err := os.WriteFile(path, []byte(pid+"\n"), 0o644); err != nil {
		return nil, fmt.Errorf("write pidfile: %w", err)
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
