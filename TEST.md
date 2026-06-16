# Manual Test Plan

Run commands from the project root.

# Build Checks

1. Format, test, vet, and build:

```sh
gofmt -w *.go
go test ./...
go vet ./...
go build -o /tmp/waybar-pulseaudio-sources .
```

# Startup Smoke Test

1. Start the program with a temporary pidfile:

```sh
timeout 3s /tmp/waybar-pulseaudio-sources --pidfile /tmp/waybar-pulseaudio-sources-test.pid
```

2. Confirm stdout contains one newline-delimited JSON object similar to:

```json
{"text":"65% ","tooltip":"Microphone Name","class":"source","percentage":65}
```

3. Confirm the pidfile was removed after exit:

```sh
test ! -e /tmp/waybar-pulseaudio-sources-test.pid
```

# Default Pidfile

1. Confirm `$XDG_RUNTIME_DIR` is set and writable:

```sh
test -n "$XDG_RUNTIME_DIR"
test -w "$XDG_RUNTIME_DIR"
```

2. Start the program without `--pidfile`:

```sh
/tmp/waybar-pulseaudio-sources
```

3. In another shell, confirm the default pidfile exists:

```sh
cat "$XDG_RUNTIME_DIR/waybar-pulseaudio-sources.pid"
```

4. Stop the process with `Ctrl-C`.

5. Confirm the default pidfile was removed:

```sh
test ! -e "$XDG_RUNTIME_DIR/waybar-pulseaudio-sources.pid"
```

# Waybar Integration

1. Configure Waybar:

```json
"custom/pulseaudio-sources": {
    "exec": "waybar-pulseaudio-sources",
    "on-click": "pidfile=${XDG_RUNTIME_DIR}/waybar-pulseaudio-sources.pid; test -r \"$pidfile\" && kill -SIGUSR1 \"$(cat \"$pidfile\")\"",
    "exec-on-event": false,
    "return-type": "json",
    "restart-interval": 300
}
```

2. Restart Waybar.

3. Confirm the module displays the default source volume and microphone icon.

4. Confirm the tooltip shows the default source's human-readable name.

# Source Switching

This test changes the default PulseAudio source.

1. List available input sources:

```sh
pactl list short sources
```

2. Ensure at least two non-monitor sources are available.

3. Start the program with a pidfile:

```sh
/tmp/waybar-pulseaudio-sources --pidfile /tmp/waybar-pulseaudio-sources-test.pid
```

4. In another shell, send `SIGUSR1`:

```sh
kill -SIGUSR1 "$(cat /tmp/waybar-pulseaudio-sources-test.pid)"
```

5. Confirm the default source changed:

```sh
pactl get-default-source
```

6. Confirm the program emits an updated JSON line.

# PulseAudio Event Updates

1. Start the program.

2. Change the default source using `pactl` or `pavucontrol`.

3. Confirm the program emits a new JSON line without restarting or polling.

4. Mute and unmute the default source.

5. Confirm the icon changes between `` and ``.

# Monitor Source Exclusion

1. List sources:

```sh
pactl list short sources
```

2. Confirm sources ending in `.monitor` are present only as monitor sources.

3. Cycle sources with `SIGUSR1`.

4. Confirm monitor sources are not selected as the default by this program.

# Unavailable Status

This test interrupts PulseAudio availability.

1. Start the program.

2. Stop or restart PulseAudio/PipeWire PulseAudio compatibility in the local user session.

3. Confirm the program emits unavailable status:

```json
{"text":"","tooltip":"PulseAudio unavailable: ...","class":"unavailable","percentage":0}
```

4. Restore PulseAudio availability.

5. Confirm the program reconnects and emits normal source status.

# Pidfile Error

1. Start with a pidfile path whose parent directory does not exist:

```sh
/tmp/waybar-pulseaudio-sources --pidfile /tmp/missing-dir/waybar-pulseaudio-sources.pid
```

2. Confirm the program exits with a fatal stderr message similar to:

```text
waybar-pulseaudio-sources: write pidfile: open /tmp/missing-dir/waybar-pulseaudio-sources.pid: no such file or directory
```

# Duplicate Output

1. Start the program and capture stdout:

```sh
/tmp/waybar-pulseaudio-sources --pidfile /tmp/waybar-pulseaudio-sources-test.pid | tee /tmp/waybar-pulseaudio-sources.log
```

2. Trigger PulseAudio events that do not change the rendered default source state.

3. Confirm duplicate JSON lines are not repeatedly written for the same rendered state.
