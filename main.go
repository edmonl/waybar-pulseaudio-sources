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
	"github.com/edmonl/waybar-pulseaudio-sources/waybar"
)

const (
	reconnectDelay = 10 * time.Minute
	pidfileName    = "waybar-pulseaudio-sources.pid"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("waybar-pulseaudio-sources: ")

	options := parseOptions()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, options); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

type options struct {
	pidfile string
	text    string
	class   string
	tooltip string
}

func run(ctx context.Context, options options) error {
	formatter, err := waybar.NewFormatter(options.text, options.class, options.tooltip)
	if err != nil {
		return err
	}

	if options.pidfile != "" {
		removePIDFile, err := writePIDFile(options.pidfile)
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
				if e := output.EmitIfChanged(formatter.Error(err)); e != nil {
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

func parseOptions() options {
	var pidfile string

	opts := options{
		text:    "{{or (.State | capitalize) (print .Volume `%`)}}",
		class:   "{{.State}}",
		tooltip: "{{.Desc}}",
	}
	flag.Usage = usage
	flag.StringVar(&pidfile, "pidfile", "", "write the process ID to this file; empty disables the pidfile")
	flag.StringVar(&opts.text, "text", opts.text, "Go template for Waybar text")
	flag.StringVar(&opts.class, "class", opts.class, "Go template for Waybar class")
	flag.StringVar(&opts.tooltip, "tooltip", opts.tooltip, "Go template for Waybar tooltip")
	flag.Parse()

	opts.pidfile = parsePidfile(pidfile)
	return opts
}

func usage() {
	output := flag.CommandLine.Output()
	fmt.Fprintln(output, "A long-running Waybar custom module for PulseAudio input sources.")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Template fields for -text, -class, and -tooltip:")
	fmt.Fprintln(output, "  Index   PulseAudio runtime source index, or -1 when no source is available")
	fmt.Fprintln(output, "  Name    PulseAudio source name")
	fmt.Fprintln(output, "  Desc    PulseAudio source description, or error detail when no source is available")
	fmt.Fprintln(output, "  Muted   whether the source is muted")
	fmt.Fprintln(output, "  Volume  unclamped average channel volume percentage")
	fmt.Fprintln(output, "  State   empty for a healthy unmuted source, or muted, unavailable, error")
	fmt.Fprintln(output, "  Available  whether source data is available")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Template functions:")
	fmt.Fprintln(output, "  capitalize  uppercase the first character of a string")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Flags:")
	flag.PrintDefaults()
}

func parsePidfile(pidfile string) string {
	if pidfile == "" {
		pidfileDisabled := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "pidfile" {
				pidfileDisabled = true
			}
		})
		if pidfileDisabled {
			return ""
		}

		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			log.Fatal("XDG_RUNTIME_DIR is empty and --pidfile was not provided")
		}
		if !filepath.IsAbs(runtimeDir) {
			log.Fatal("XDG_RUNTIME_DIR must be an absolute path")
		}

		return filepath.Join(runtimeDir, pidfileName)
	}

	pidfile = strings.TrimSpace(pidfile)
	if pidfile == "" {
		log.Fatal("--pidfile must not be blank")
	}

	if !filepath.IsAbs(pidfile) {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		pidfile = filepath.Join(cwd, pidfile)
	}

	return pidfile
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
