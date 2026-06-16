# Overview

This project is a Go program that uses CGo bindings to `libpulse`.

It runs as a long-lived Waybar custom module process and writes newline-delimited JSON updates to stdout. Each JSON line is flushed after it is written.

The program subscribes to PulseAudio source and server events, then emits updates when relevant PulseAudio state changes.

# Waybar Integration

Example Waybar configuration:

```json
"custom/pulseaudio-sources": {
    "exec": "waybar-pulseaudio-sources",
    "on-click": "pidfile=${XDG_RUNTIME_DIR}/waybar-pulseaudio-sources.pid; test -r \"$pidfile\" && kill -SIGUSR1 \"$(cat \"$pidfile\")\"",
    "exec-on-event": false,
    "return-type": "json",
    "restart-interval": 300
}
```

Waybar starts the process through `exec`. Click handling sends `SIGUSR1` to the running process.

The click command reads the pidfile only when it is present and readable. If the process is not running or the pidfile is absent, the click command is a no-op.

The program writes its PID to a pidfile on startup and removes the pidfile on exit. The default pidfile is `$XDG_RUNTIME_DIR/waybar-pulseaudio-sources.pid`. The `--pidfile` flag may override this path. The pidfile parent directory must already exist. Overwriting an existing pidfile is allowed. Failure to create or write the pidfile is a fatal startup error.

`restart-interval` lets Waybar restart the long-running process if it exits or crashes. It is intended for continuous custom modules and must not be used together with `interval`. The example uses 300 seconds. Fatal startup errors, such as pidfile write failure, are logged to stderr and exit; Waybar may retry them on this interval.

# Display Behavior

The program avoids writing duplicate JSON lines when multiple PulseAudio events produce the same rendered state.
The module text shows the default PulseAudio source volume and mute state:

1. Unmuted: `60% `
2. Muted: `60% `

The tooltip shows the default source's human-readable name.

Source-status JSON output includes `text`, `tooltip`, `class`, and `percentage`.

The `percentage` value is the unclamped PulseAudio average channel volume percentage. It may exceed 100.

If PulseAudio is unavailable, the program emits unavailable status and retries connection after a long delay. The delay should avoid tight reconnect loops because PulseAudio is usually not restored immediately.

`SIGUSR1` always represents a source cycling request. If PulseAudio is unavailable when the signal is received, the program records the pending cycling request, retries connection immediately, and applies the cycle after reconnecting.

```json
{"text":"Unavailable ","tooltip":"...","class":"unavailable"}
```

Operation failures that are not availability failures use error status:

```json
{"text":"Error ","tooltip":"...","class":"error"}
```

# Source Switching

On `SIGUSR1`, the program selects the next available PulseAudio source and sets it as the default source. Reconnection is a prerequisite for this operation when PulseAudio is unavailable.

Sources whose PulseAudio name ends with `.monitor` are excluded from display and source switching.

Source switching uses ascending PulseAudio source index order. PulseAudio source indexes are runtime identifiers and are not expected to be stable across PulseAudio server restarts.

Changing the default source primarily affects new recording streams. Existing recording streams may continue using their current source unless the application or PulseAudio policy explicitly moves them.

# Scope

The program only handles PulseAudio sources.
