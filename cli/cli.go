// Package cli parses command-line options and subcommands.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const pidfileName = "waybar-pulseaudio-sources.pid"

// Command is a parsed top-level command invocation.
type Command struct {
	// SwitchSource reports whether the switch subcommand was selected.
	SwitchSource bool

	// Pidfile is the resolved pidfile path used by the selected command.
	Pidfile string

	// Text is the Go template for Waybar text.
	Text string

	// Class is the Go template for Waybar class.
	Class string

	// Tooltip is the Go template for Waybar tooltip.
	Tooltip string
}

// Parse parses top-level options and subcommands.
func Parse(name string, args []string) (Command, error) {
	if len(args) > 0 && args[0] == "switch" {
		pidfile, err := parseSwitchOptions(name, args[1:])
		if err != nil {
			return Command{}, err
		}
		return Command{
			SwitchSource: true,
			Pidfile:      pidfile,
		}, nil
	}

	command, err := parseOptions(name, args)
	if err != nil {
		return Command{}, err
	}
	return command, nil
}

// IsHelp reports whether err represents a help request.
func IsHelp(err error) bool {
	return errors.Is(err, flag.ErrHelp)
}

func parseOptions(name string, args []string) (Command, error) {
	var pidfile string

	command := Command{
		Text:    "{{or (.State | capitalize) (print .Volume `%`)}}",
		Class:   "{{.State}}",
		Tooltip: "{{.Desc}}",
	}
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.Usage = func() { usage(flags, name) }
	flags.StringVar(&pidfile, "pidfile", "", "write the process ID to this file; empty disables the pidfile")
	flags.StringVar(&command.Text, "text", command.Text, "Go template for Waybar text")
	flags.StringVar(&command.Class, "class", command.Class, "Go template for Waybar class")
	flags.StringVar(&command.Tooltip, "tooltip", command.Tooltip, "Go template for Waybar tooltip")
	if err := flags.Parse(args); err != nil {
		return Command{}, err
	}
	if flags.NArg() > 0 {
		return Command{}, fmt.Errorf("unknown argument: %v", flags.Arg(0))
	}

	resolvedPidfile, err := parsePidfile(flags, pidfile)
	if err != nil {
		return Command{}, err
	}
	command.Pidfile = resolvedPidfile
	return command, nil
}

func parseSwitchOptions(name string, args []string) (string, error) {
	var pidfile string

	flags := flag.NewFlagSet(name+" switch", flag.ContinueOnError)
	flags.Usage = func() { switchUsage(flags, name) }
	flags.StringVar(&pidfile, "pidfile", "", "read the process ID from this file")
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if flags.NArg() > 0 {
		return "", fmt.Errorf("unknown argument: %v", flags.Arg(0))
	}

	resolvedPidfile, err := parsePidfile(flags, pidfile)
	if err != nil {
		return "", err
	}
	if resolvedPidfile == "" {
		return "", fmt.Errorf("--pidfile must not be empty")
	}
	return resolvedPidfile, nil
}

func usage(flags *flag.FlagSet, name string) {
	output := flags.Output()
	fmt.Fprintln(output, "A long-running Waybar custom module for PulseAudio input sources.")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Usage:")
	fmt.Fprintf(output, "  %s [flags]\n", name)
	fmt.Fprintf(output, "  %s switch [flags]\n", name)
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
	fmt.Fprintln(output, "Subcommands:")
	fmt.Fprintln(output, "  switch  ask the running module process to cycle to the next source")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Flags:")
	flags.PrintDefaults()
}

func switchUsage(flags *flag.FlagSet, name string) {
	output := flags.Output()
	fmt.Fprintln(output, "Signal the running module process to switch the default PulseAudio input source.")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Usage:")
	fmt.Fprintf(output, "  %s switch [flags]\n", name)
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Flags:")
	flags.PrintDefaults()
}

func parsePidfile(flags *flag.FlagSet, pidfile string) (string, error) {
	if pidfile == "" {
		pidfileDisabled := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "pidfile" {
				pidfileDisabled = true
			}
		})
		if pidfileDisabled {
			return "", nil
		}

		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			return "", fmt.Errorf("XDG_RUNTIME_DIR is empty and --pidfile was not provided")
		}
		if !filepath.IsAbs(runtimeDir) {
			return "", fmt.Errorf("XDG_RUNTIME_DIR must be an absolute path")
		}

		return filepath.Join(runtimeDir, pidfileName), nil
	}

	pidfile = strings.TrimSpace(pidfile)
	if pidfile == "" {
		return "", fmt.Errorf("--pidfile must not be blank")
	}

	if !filepath.IsAbs(pidfile) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		pidfile = filepath.Join(cwd, pidfile)
	}

	return pidfile, nil
}
