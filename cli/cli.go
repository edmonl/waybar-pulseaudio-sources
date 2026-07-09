// Package cli parses command-line options and subcommands.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/edmonl/waybar-pulseaudio-sources/daemonutil/runtimepath"
)

const socketName = "waybar-pulseaudio-sources.sock"

// Command is a parsed top-level command invocation.
type Command struct {
	// SwitchSource reports whether the switch subcommand was selected.
	SwitchSource bool

	// Sock is the resolved Unix socket path used by the selected command.
	Sock string

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
		sock, err := parseSwitchOptions(name, args[1:])
		if err != nil {
			return Command{}, err
		}
		return Command{
			SwitchSource: true,
			Sock:         sock,
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
	var sock string

	command := Command{
		Text:    "{{or (.State | capitalize) (print .Volume `%`)}}",
		Class:   "{{.State}}",
		Tooltip: "{{.Desc}}",
	}
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.Usage = func() { usage(flags, name) }
	flags.StringVar(&sock, "sock", "", "path of Unix socket, disabled when empty")
	flags.StringVar(&command.Text, "text", command.Text, "Go template for Waybar text")
	flags.StringVar(&command.Class, "class", command.Class, "Go template for Waybar class")
	flags.StringVar(&command.Tooltip, "tooltip", command.Tooltip, "Go template for Waybar tooltip")
	if err := flags.Parse(args); err != nil {
		return Command{}, err
	}
	if flags.NArg() > 0 {
		return Command{}, fmt.Errorf("unknown argument: %v", flags.Arg(0))
	}

	resolvedSock, err := parseSocketPath(flags, sock)
	if err != nil {
		return Command{}, err
	}
	command.Sock = resolvedSock
	return command, nil
}

func parseSwitchOptions(name string, args []string) (string, error) {
	var sock string

	flags := flag.NewFlagSet(name+" switch", flag.ContinueOnError)
	flags.Usage = func() { switchUsage(flags, name) }
	flags.StringVar(&sock, "sock", "", "Unix socket path")
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if flags.NArg() > 0 {
		return "", fmt.Errorf("unknown argument: %v", flags.Arg(0))
	}

	resolvedSock, err := parseSocketPath(flags, sock)
	if err != nil {
		return "", err
	}
	if resolvedSock == "" {
		return "", fmt.Errorf("--sock must not be empty")
	}
	return resolvedSock, nil
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
	fmt.Fprintln(output, "Ask the running module process to switch the default PulseAudio input source.")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Usage:")
	fmt.Fprintf(output, "  %s switch [flags]\n", name)
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Flags:")
	flags.PrintDefaults()
}

func parseSocketPath(flags *flag.FlagSet, sock string) (string, error) {
	if sock == "" {
		socketDisabled := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "sock" {
				socketDisabled = true
			}
		})
		if socketDisabled {
			return "", nil
		}

		path, err := runtimepath.RuntimeDirJoin(socketName)
		if err != nil {
			return "", err
		}
		return path, nil
	}

	sock = strings.TrimSpace(sock)
	if sock == "" {
		return "", fmt.Errorf("--sock must not be blank")
	}

	return runtimepath.WorkingDirJoin(sock)
}
