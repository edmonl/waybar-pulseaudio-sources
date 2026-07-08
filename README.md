A Waybar custom module for showing and setting PulseAudio input sources.

The module prints Waybar-compatible JSON updates as PulseAudio state changes. It shows the default source volume, uses the source description as the tooltip, and can switch to the next available input source.

# Quick Start

 Edit Waybar config (e.g. `~/.config/waybar/config.jsonc`) to add the custom module:

```json
"custom/pulseaudio-sources": {
	"exec": "exec waybar-pulseaudio-sources --text '{{if .Available}}{{.Volume}}%{{else}}{{capitalize .State}}{{end}}{{if .Muted}}{{else}}{{end}}'",
	"on-click": "waybar-pulseaudio-sources switch",
	"exec-on-event": false,
	"return-type": "json",
	"restart-interval": 300
}
```
The above example shows customized text using unicode emojis. Make sure you have the proper font if you copy it literally.
If the executable binary is not in PATH, make sure to specify the path e.g. `~/.local/bin/waybar-pulseaudio-sources`.

Then include `custom/pulseaudio-sources` in one of Waybar's module lists. for example:

```json
"modules-right": [
    "custom/pulseaudio-sources"
]
```

Restart Waybar after changing the config.

# Build

Go, `pkg-config`, `build-essentials`, and the PulseAudio development headers re required to build from source.

On Debian or Ubuntu, the build dependencies are typically:

```sh
sudo apt install golang pkg-config libpulse-dev
```

On Fedora, they are typically:

```sh
sudo dnf install golang pkgconf-pkg-config pulseaudio-libs-devel
```

Build and install from the project directory:

```sh
go install
```

Make sure Go's install directory is on your `PATH`.

You can also build a local binary:

```sh
go build
```

Then place the binary somewhere Waybar can execute it, such as `~/.local/bin`.


# Display templates

The source text can be customized with `--text`, which accepts a Go `text/template`.
The default text template is:

```gotemplate
{{if .State}}{{capitalize .State}} {{else}}{{.Volume}}%{{end}}
```

The template may use these fields:

1. `Index`: PulseAudio runtime source index, or `-1` when no source is available.
2. `Name`: PulseAudio source name.
3. `Desc`: human-readable PulseAudio source description, or error detail when no source is available.
4. `Muted`: whether the source is muted.
5. `Volume`: unclamped average channel volume percentage.
6. `State`: `""` for a healthy unmuted source, `muted`, `unavailable`, or `error`.
7. `Available`: whether source data is available.

The template may also use `capitalize`, which uppercases the first character of a string.

Empty template values and malformed templates cause startup to fail. Template execution errors produce error output with the error detail in the tooltip.

The Waybar class and tooltip can also be customized:

```json
"exec": "waybar-pulseaudio-sources --class '{{.State}}' --tooltip '{{.Desc}}'"
```

The default class is omitted for a healthy unmuted source and is `muted`, `unavailable`, or `error` otherwise.
The default tooltip is `Desc`, which is the human-readable source name reported by PulseAudio or the error detail for unavailable/error states.

Clicking the module runs `waybar-pulseaudio-sources switch`, which asks the running process to select the next non-monitor source and set it as the default PulseAudio source. Monitor sources ending in `.monitor` are ignored.

Source switching follows PulseAudio source index order. These indexes are runtime identifiers, so the order can change after PulseAudio restarts.

Changing the default source mainly affects new recording streams. Applications that are already recording may keep using their current source until they are restarted or moved by PulseAudio policy.

# Pidfile

By default, the module writes its process ID to:

```text
$XDG_RUNTIME_DIR/waybar-pulseaudio-sources.pid
```

This is what the `switch` subcommand uses to find the running process.

You can override the pidfile path:

```json
"exec": "waybar-pulseaudio-sources --pidfile /tmp/waybar-pulseaudio-sources.pid"
```

You can disable pidfile output by passing an explicit empty value:

```json
"exec": "waybar-pulseaudio-sources --pidfile ''"
```

If pidfile output is enabled, the pidfile directory must already exist. The module removes its pidfile when it exits cleanly.

# Waybar custom module contract

The program writes newline-delimited JSON to stdout for Waybar. A typical update looks like:

```json
{"text":"65%","tooltip":"Microphone Name","percentage":65}
```

The `percentage` value is the PulseAudio average channel volume percentage and may be greater than 100.

# Troubleshooting

1. If the module does not appear, run `waybar-pulseaudio-sources` in a terminal and check for errors.
2. If Waybar logs mention `XDG_RUNTIME_DIR`, make sure Waybar starts with that environment variable set to an absolute path.
3. Check that the pidfile exists and contains a running process ID when click action is used.
5. If source switching works but an application keeps using the old microphone, restart that application or move the stream in your audio control tool.

