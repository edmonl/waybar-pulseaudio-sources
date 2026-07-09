## Overview

[![CI](https://github.com/edmonl/waybar-pulseaudio-sources/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/edmonl/waybar-pulseaudio-sources/actions/workflows/ci.yml)

A Waybar custom module for showing and changing the default PulseAudio input source, such as a microphone.
![Waybar module showing microphone volume and icon](screenshot.png)

By default, the module shows the default source volume in Waybar, uses the source description as the tooltip, and can switch to the next non-monitor input source. It works with PulseAudio-compatible servers, including PipeWire's PulseAudio server.

This project exists because Waybar's built-in PulseAudio support is much stronger for output sinks than for input sources. This provides a focused, customizable input-source module for microphone status and source switching.


## Quick Start

Add the custom module to your Waybar config, for example `~/.config/waybar/config.jsonc`:

```json
"custom/pulseaudio-sources": {
    "exec": "waybar-pulseaudio-sources",
    "on-click": "waybar-pulseaudio-sources switch",
    "exec-on-event": false,
    "return-type": "json",
    "restart-interval": 300
}
```

If the executable is not on `PATH`, use its full path, for example `~/.local/bin/waybar-pulseaudio-sources`.
This module is intended to keep running. It keeps watching PulseAudio state changes.
Thus `exec-on-event` must be `false` so Waybar does not restart the module for click events. `restart-interval` lets Waybar start it again if it exits.

Then include `custom/pulseaudio-sources` in one of Waybar's module lists:

```json
"modules-right": [
    "custom/pulseaudio-sources"
]
```

Optionally customize the appearance in Waybar CSS, for example `~/.config/waybar/style.css`:

```css
#custom-pulseaudio-sources {
    background-color: #f1c40f;
    color: #000000;
    padding: 0 10px;
}
```

Restart Waybar.

## Installation

### System Dependencies

Besides Go, building requires `pkg-config`, a C build toolchain for `cgo`, and the PulseAudio development headers.

On Debian or Ubuntu:

```sh
sudo apt install build-essential pkg-config libpulse-dev
```

On Fedora:

```sh
sudo dnf install gcc pkgconf-pkg-config pulseaudio-libs-devel
```

### Install With Go

Install the latest version from the module path:

```sh
go install github.com/edmonl/waybar-pulseaudio-sources@latest
```

Make sure Go's install directory is on your `PATH` if you do not use the full binary path in your Waybar config.

### Build From Local Checkout

From the project directory:

```sh
go install
```

You can also build a local binary:

```sh
go build
```

Then place the binary somewhere Waybar can execute it, such as `~/.local/bin`.

## Display Templates

The Waybar `text`, `class`, and `tooltip` fields are rendered with Go `text/template` templates. Override them with `--text`, `--class`, and `--tooltip`:

```json
"custom/pulseaudio-sources": {
    "exec": "waybar-pulseaudio-sources --text '{{if .Available}}{{.Volume}}% {{if .Muted}}{{else}}{{end}}{{else}}{{capitalize .State}}{{end}}'",
    "on-click": "waybar-pulseaudio-sources switch",
    "exec-on-event": false,
    "return-type": "json",
    "restart-interval": 300
}
```

The icon glyphs in this example require a font that can render them properly.

Templates receive these fields:

1. `Index`: PulseAudio runtime source index, or `-1` when no source is available.
2. `Name`: PulseAudio source name.
3. `Desc`: human-readable PulseAudio source description, or error detail when no source is available.
4. `Muted`: whether the source is muted.
5. `Volume`: unclamped average channel volume percentage.
6. `State`: empty for a healthy unmuted source, or `muted`, `unavailable`, or `error`.
7. `Available`: whether source data is available.

Templates may also use `capitalize`, which uppercases the first character of a string.

Empty template values and malformed templates cause startup to fail. Template execution errors produce error output with the error detail in the tooltip.

The default templates are:

```sh
--text '{{or (.State | capitalize) (print .Volume `%`)}}'
--class '{{.State}}'
--tooltip '{{.Desc}}'
```

With the defaults, the module:

1. Shows the volume percentage for an available, unmuted source.
2. Shows `Muted`, `Unavailable`, or `Error` when those states apply.
3. Omits the Waybar class for a healthy unmuted source and uses `muted`, `unavailable`, or `error` otherwise.
4. Uses the PulseAudio source description, or the error detail, as the tooltip.

## Source Switching

Clicking the module, as configured in [Quick Start](#quick-start), runs `waybar-pulseaudio-sources switch`. This sends a switch request to the running module. The running module then switches the system default to the next source.

Sources whose names end with `.monitor` are not displayed or selected. Source switching follows ascending PulseAudio source index order. These indexes are runtime identifiers, so the order can change after PulseAudio restarts.

Changing the default source mainly affects new recording streams. Existing applications may keep using their current input source unless the application or PulseAudio policy moves them.

## Control Socket

By default, the module listens for switch requests at:

```text
$XDG_RUNTIME_DIR/waybar-pulseaudio-sources.sock
```

If `XDG_RUNTIME_DIR` is empty, the default path falls back to `$TMPDIR/waybar-pulseaudio-sources.sock`, then `/tmp/waybar-pulseaudio-sources.sock`. Environment-provided runtime directories must be absolute and must already exist.

The `switch` subcommand uses the same default path. You may override the socket path when starting the module:

```json
"exec": "waybar-pulseaudio-sources --sock /tmp/waybar-pulseaudio-sources.sock"
```

Use the same socket for the click action:

```json
"on-click": "waybar-pulseaudio-sources switch --sock /tmp/waybar-pulseaudio-sources.sock"
```

Relative socket paths are resolved against the current working directory, which is useful for debugging. The socket parent directory must already exist. Startup fails if another live module already owns the socket path. A stale socket path may be replaced.

You can disable the socket by passing an explicit empty value:

```json
"exec": "waybar-pulseaudio-sources --sock ''"
```

Disabling the socket makes `waybar-pulseaudio-sources switch` not work, because the switch command needs a socket to reach the running module.

## Waybar Output

The module writes newline-delimited JSON to stdout for Waybar. A typical update looks like:

```json
{"text":"65%","tooltip":"Microphone Name","percentage":65}
```

The `percentage` value is the unclamped PulseAudio average channel volume percentage and may be greater than 100. Unavailable and error output has no `percentage`.

## Troubleshooting

1. Run `waybar-pulseaudio-sources` in a terminal and check stderr for startup or PulseAudio errors.
2. If click switching does not work, confirm the running module and `switch` command use the same socket path.
3. If switching works but an application keeps using the old input source, check that application's input-source configuration or restart the application.

## Contributing

Feel free to open GitHub issues for suggestions.
