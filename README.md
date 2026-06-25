A Waybar custom module for showing and setting PulseAudio input sources.

The module prints Waybar-compatible JSON updates as PulseAudio state changes. It shows the default source volume and mute state, uses the source description as the tooltip, and can switch to the next available input source when clicked.

# Requirements

1. Waybar with custom module support.
2. PulseAudio or a PulseAudio-compatible server.
3. A Nerd Font or other font that includes the microphone icons used by the module.
4. Go 1.26 or newer, `pkg-config`, and the PulseAudio development headers when building from source.

On Debian or Ubuntu, the build dependencies are typically:

```sh
sudo apt install golang pkg-config libpulse-dev
```

On Fedora, they are typically:

```sh
sudo dnf install golang pkgconf-pkg-config pulseaudio-libs-devel
```

# Installation

Build and install from the project directory:

```sh
go install .
```

Make sure Go's install directory is on your `PATH`. By default, that is usually:

```sh
export PATH="$HOME/go/bin:$PATH"
```

You can also build a local binary:

```sh
go build -o waybar-pulseaudio-sources .
```

Then place the binary somewhere Waybar can execute it, such as `~/.local/bin`.

# Waybar Configuration

Add a custom module to your Waybar config:

```json
"custom/pulseaudio-sources": {
    "exec": "waybar-pulseaudio-sources",
    "on-click": "pidfile=${XDG_RUNTIME_DIR}/waybar-pulseaudio-sources.pid; test -r \"$pidfile\" && kill -SIGUSR1 \"$(cat \"$pidfile\")\"",
    "exec-on-event": false,
    "return-type": "json",
    "restart-interval": 300
}
```

Then include `custom/pulseaudio-sources` in one of Waybar's module lists, for example:

```json
"modules-right": [
    "custom/pulseaudio-sources"
]
```

Restart Waybar after changing the config.

# Behavior

The module displays the current default PulseAudio source:

1. `60% ` for an unmuted source.
2. `60% ` for a muted source.
3. `Unavailable ` when PulseAudio cannot be reached.
4. `Error ` when an operation fails.

The tooltip is the human-readable source name reported by PulseAudio.

Clicking the module sends `SIGUSR1` to the running process. On that signal, the module selects the next non-monitor source and sets it as the default PulseAudio source. Monitor sources ending in `.monitor` are ignored.

Source cycling follows PulseAudio source index order. These indexes are runtime identifiers, so the order can change after PulseAudio restarts.

Changing the default source mainly affects new recording streams. Applications that are already recording may keep using their current source until they are restarted or moved by PulseAudio policy.

# Pidfile

By default, the module writes its process ID to:

```text
$XDG_RUNTIME_DIR/waybar-pulseaudio-sources.pid
```

This is what the example `on-click` command uses to find the running process.

You can override the pidfile path:

```json
"exec": "waybar-pulseaudio-sources --pidfile /tmp/waybar-pulseaudio-sources.pid"
```

You can disable pidfile output by passing an explicit empty value:

```json
"exec": "waybar-pulseaudio-sources --pidfile ''"
```

If pidfile output is enabled, the pidfile directory must already exist. The module removes its pidfile when it exits cleanly.

# Troubleshooting

1. If the module does not appear, run `waybar-pulseaudio-sources` in a terminal and check for errors.
2. If Waybar logs mention `XDG_RUNTIME_DIR`, make sure Waybar starts with that environment variable set to an absolute path.
3. If the click action does nothing, check that the pidfile exists and contains a running process ID.
4. If the icons render as boxes, install and select a font that includes the microphone glyphs.
5. If source switching works but an application keeps using the old microphone, restart that application or move the stream in your audio control tool.

# Output

The program writes newline-delimited JSON to stdout for Waybar. A typical update looks like:

```json
{"text":"65% ","tooltip":"Microphone Name","class":"source","percentage":65}
```

The `percentage` value is the PulseAudio average channel volume percentage and may be greater than 100.
