# Scope

The project provides a long-running Waybar custom module for PulseAudio input sources.

The program reports the current default PulseAudio source, updates Waybar when relevant PulseAudio state changes, and lets the user cycle to the next available input source from Waybar.

# Requirements

The program requires a PulseAudio-compatible server and a Waybar configuration that runs it as a continuous custom module.

When pidfile output is enabled, `$XDG_RUNTIME_DIR` must be set to an absolute path unless `--pidfile` provides an explicit path.

# Runtime Behavior

The program writes newline-delimited JSON updates to stdout. Each JSON line is flushed after it is written.

The program emits an update at startup and whenever relevant PulseAudio source or server state changes.

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

The program writes its PID to a pidfile on startup and removes the pidfile on exit when pidfile output is enabled. The default pidfile is `$XDG_RUNTIME_DIR/waybar-pulseaudio-sources.pid`; `$XDG_RUNTIME_DIR` must be set to an absolute path. The `--pidfile` flag may override this path; an explicit empty value disables pidfile output. Explicit pidfile paths are trimmed, and relative paths are resolved against the current working directory. Blank explicit pidfile values after trimming are invalid. When pidfile output is enabled, failure to determine the default path or create/write the pidfile is a fatal startup error. The pidfile parent directory must already exist. Overwriting an existing pidfile is allowed.

`restart-interval` lets Waybar restart the long-running process if it exits or crashes. It is intended for continuous custom modules and must not be used together with `interval`. The example uses 300 seconds. Fatal startup errors, such as pidfile write failure, are logged to stderr and exit; Waybar may retry them on this interval.

# Display Behavior

The program avoids writing duplicate JSON lines when multiple PulseAudio events produce the same rendered state.
The module text is rendered from the default PulseAudio source with a Go `text/template`.
The default template is `{{.Volume}}%`.
The `--format` flag overrides this template. Templates may use these fields:

1. `Name`: PulseAudio source name.
2. `Desc`: human-readable PulseAudio source description.
3. `Muted`: whether the source is muted.
4. `Volume`: unclamped average channel volume percentage.

Empty format values and invalid templates are fatal startup errors.
The template applies only to the Waybar `text` field, not the tooltip.

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

# Design Notes

The process is intended to be restarted by Waybar if it exits or crashes.

Reconnect behavior favors avoiding tight retry loops during PulseAudio outages while still responding immediately to a user source-cycling request.
