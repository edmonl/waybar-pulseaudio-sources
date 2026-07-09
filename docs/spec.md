# Scope

The project provides a long-running Waybar custom module for PulseAudio input sources.

The program reports the current default PulseAudio source, updates Waybar when relevant PulseAudio state changes, and lets the user cycle to the next available input source from Waybar.

# Requirements

The program requires a PulseAudio-compatible server and a Waybar configuration that runs it as a continuous custom module.

The default control socket path is resolved from the runtime directory environment. `XDG_RUNTIME_DIR` is preferred, then `TMPDIR`, then `/tmp`. Environment-provided runtime directories must be absolute and must already exist.

# Runtime Behavior

The program writes newline-delimited JSON updates to stdout. Each JSON line is flushed after it is written.

The program emits an update at startup and whenever relevant PulseAudio source or server state changes.

# Waybar Integration

Example Waybar configuration:

```json
"custom/pulseaudio-sources": {
    "exec": "waybar-pulseaudio-sources",
    "on-click": "waybar-pulseaudio-sources switch",
    "exec-on-event": false,
    "return-type": "json",
    "restart-interval": 300
}
```

Waybar starts the process through `exec`. Click handling runs `waybar-pulseaudio-sources switch`, which sends a switch request to the running module process.

The long-running process listens for control requests on a Unix socket. The default socket path is `waybar-pulseaudio-sources.sock` inside the resolved runtime directory. The `--sock` flag may override this path. An explicit empty `--sock` value disables the control socket for the long-running command. Explicit socket paths are trimmed, and relative paths are resolved against the current working directory. Blank explicit socket values after trimming are invalid.

The switch command exits successfully after sending the request. If the control socket cannot be reached, the switch command exits with an error.

The `switch` subcommand accepts `--sock` to match a long-running process started with a non-default socket. An explicit empty `switch --sock` value is invalid because the switch command needs a socket path.

The socket parent directory must already exist. Startup fails if the socket path exists and is not a socket, or if another live endpoint already owns the socket path. If the socket path is stale, the program may replace it. The program removes its socket on clean exit when the path still belongs to the same module instance.

`restart-interval` lets Waybar restart the long-running process if it exits or crashes. It is intended for continuous custom modules and must not be used together with `interval`. The example uses 300 seconds. Fatal startup errors are logged to stderr and exit; Waybar may retry them on this interval.

# Display Behavior

The program avoids writing duplicate JSON lines when multiple PulseAudio events produce the same rendered state.
The Waybar `text`, `class`, and `tooltip` fields are rendered with Go `text/template` templates. The `--text`, `--class`, and `--tooltip` flags override those templates.

Each template receives data with these fields:

1. `Index`: PulseAudio runtime source index, or `-1` when no source is available.
2. `Name`: PulseAudio source name.
3. `Desc`: human-readable PulseAudio source description, or error detail when no source is available.
4. `Muted`: whether the source is muted.
5. `Volume`: unclamped average channel volume percentage.
6. `State`: `""` for a healthy unmuted source, `muted`, `unavailable`, or `error`.
7. `Available`: whether source data is available.

For available source data, `Available` is `true`, `Index`, `Name`, `Desc`, `Muted`, and `Volume` describe the default source, and `State` is `""` for an unmuted source or `muted` for a muted source.

For PulseAudio availability failures and cases where no eligible input source is available, `Available` is `false`, `Index` is `-1`, `Name` is `""`, `Desc` contains the status detail, `Muted` is `false`, `Volume` is `0`, and `State` is `unavailable`.

For other operation failures, `Available` is `false`, `Index` is `-1`, `Name` is `""`, `Desc` contains the operation error detail, `Muted` is `false`, `Volume` is `0`, and `State` is `error`.

Empty template values and malformed templates are fatal startup errors. Template execution errors emit formatter-error JSON with the error detail in the tooltip.

Templates may use `capitalize` to uppercase the first character of a string.

Normal source-status JSON output includes `text` and `percentage`. The `class` field is present only when the rendered class is non-empty. The `tooltip` field is present only when the rendered tooltip is non-empty.

The `percentage` value is the unclamped PulseAudio average channel volume percentage. It may exceed 100. Unavailable, operation-error, and template-execution-error JSON output omits `percentage`.

If PulseAudio is unavailable, the program emits unavailable status and retries connection after a long delay. The delay should avoid tight reconnect loops because PulseAudio is usually not restored immediately.

A switch request always represents a source cycling request. If PulseAudio is unavailable when the request is received, the program records the pending switch request, retries connection immediately, and applies the cycle after reconnecting.

# Source Switching

On a switch request, the program selects the next eligible input source and sets it as the default source. Reconnection is a prerequisite for this operation when PulseAudio is unavailable.

Sources whose PulseAudio name ends with `.monitor` are excluded from display and are never selected by source switching. When the current default source is a monitor source, source switching may use it as the ordering anchor before selecting the next non-monitor source.

Source switching uses ascending PulseAudio source index order. PulseAudio source indexes are runtime identifiers and are not expected to be stable across PulseAudio server restarts.
If only one eligible source is available, source switching selects that same source again.

Changing the default source primarily affects new recording streams. Existing recording streams may continue using their current source unless the application or PulseAudio policy explicitly moves them.

# Design Notes

The process is intended to be restarted by Waybar if it exits or crashes.

Reconnect behavior favors avoiding tight retry loops during PulseAudio outages while still responding immediately to a user source-cycling request.
