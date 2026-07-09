# Manual Test Plan

Run commands from the project root.

Before each test, record any current state the test may affect, such as the default PulseAudio source, source mute state, running test process, socket path, or temporary log file. After each test, confirm affected state has been restored or cleaned up.

Tests that start `/tmp/waybar-pulseaudio-sources` without `timeout` run until stopped. If a test hangs or is interrupted, stop the process with `Ctrl-C` when it is in the foreground.

Use this cleanup after tests that start the program and after interrupted tests:

```sh
rm -f /tmp/waybar-pulseaudio-sources-test.sock /tmp/waybar-pulseaudio-sources.log
```

# Build Checks

This check writes `/tmp/waybar-pulseaudio-sources`, which later tests use. Remove it after manual testing if it is no longer needed.

1. Check formatting, run tests, vet, and build:

```sh
test -z "$(gofmt -l -s .)"
go test ./...
go vet ./...
go build -o /tmp/waybar-pulseaudio-sources .
```

2. If the formatting check prints files, run `gofmt -s -w .` on those files and inspect the resulting diff before continuing.

# Startup Smoke Test

1. Start the program with a temporary socket:

```sh
timeout 3s /tmp/waybar-pulseaudio-sources --sock /tmp/waybar-pulseaudio-sources-test.sock
```

2. Confirm stdout contains one newline-delimited JSON object similar to:

```json
{"text":"65%","tooltip":"Microphone Name","percentage":65}
```

3. Confirm the socket was removed after exit:

```sh
test ! -e /tmp/waybar-pulseaudio-sources-test.sock
```

# Default Control Socket

1. Confirm the default runtime directory can be resolved. If `XDG_RUNTIME_DIR` is set, it must be absolute and writable:

```sh
test -z "$XDG_RUNTIME_DIR" || test "${XDG_RUNTIME_DIR#/}" != "$XDG_RUNTIME_DIR"
test -z "$XDG_RUNTIME_DIR" || test -w "$XDG_RUNTIME_DIR"
```

2. Start the program without `--sock`:

```sh
/tmp/waybar-pulseaudio-sources
```

3. In another shell, switch sources:

```sh
/tmp/waybar-pulseaudio-sources switch
```

4. Stop the process with `Ctrl-C`.

# Disabled Control Socket

1. Start the program with the control socket disabled:

```sh
timeout 3s /tmp/waybar-pulseaudio-sources --sock ''
```

2. Confirm the program starts and emits status without requiring a control socket.

3. Confirm the switch command rejects an empty socket path:

```sh
/tmp/waybar-pulseaudio-sources switch --sock ''
```

# Text, Class, And Tooltip Flags

1. Start the program with a custom text template:

```sh
timeout 3s /tmp/waybar-pulseaudio-sources --sock /tmp/waybar-pulseaudio-sources-test.sock --text '{{.Desc}} {{.Volume}}%'
```

2. Confirm stdout contains a JSON object whose `text` field contains the default source description and volume.

3. Start the program with custom class and tooltip templates:

```sh
timeout 3s /tmp/waybar-pulseaudio-sources --sock /tmp/waybar-pulseaudio-sources-test.sock --class 'mic-{{if .State}}{{.State}}{{else}}active{{end}}' --tooltip '{{.Index}} {{.Name}} {{.State}}'
```

4. Confirm stdout contains a JSON object whose `class` field starts with `mic-` and whose `tooltip` field contains the source index and PulseAudio source name.

5. Start the program with a malformed text template:

```sh
/tmp/waybar-pulseaudio-sources --sock '' --text '{{'
```

6. Confirm startup fails with a fatal template error.

7. Start the program with a text template that fails during execution:

```sh
timeout 3s /tmp/waybar-pulseaudio-sources --sock '' --text '{{.Description}}'
```

8. Confirm stdout contains a JSON object whose `text` field is `Error`, whose `class` field is `error`, and whose `tooltip` field contains the template error detail.

9. Start the program with an empty text template:

```sh
/tmp/waybar-pulseaudio-sources --sock '' --text ''
```

10. Confirm startup fails with a fatal template error.

# Runtime Directory Resolution

1. Start without `--sock` and with `XDG_RUNTIME_DIR` set to a relative path:

```sh
env XDG_RUNTIME_DIR=relative /tmp/waybar-pulseaudio-sources
```

2. Confirm startup fails because the runtime directory is not absolute.

3. Start without `--sock`, with `XDG_RUNTIME_DIR` unset, and with `TMPDIR` set to a relative path:

```sh
env -u XDG_RUNTIME_DIR TMPDIR=relative /tmp/waybar-pulseaudio-sources
```

4. Confirm startup fails because the fallback runtime directory is not absolute.

# Explicit Socket Path

1. Start the program with a relative socket path:

```sh
(cd /tmp && timeout 3s /tmp/waybar-pulseaudio-sources --sock waybar-pulseaudio-sources-test.sock)
```

2. Confirm the socket was created relative to `/tmp` and removed after exit:

```sh
test ! -e /tmp/waybar-pulseaudio-sources-test.sock
```

3. Start the program with a blank non-empty socket value:

```sh
/tmp/waybar-pulseaudio-sources --sock '   '
```

4. Confirm startup fails because the explicit socket value is blank after trimming.

# Source Switching

This test changes the default PulseAudio source.

1. Record the current default source:

```sh
original_source=$(pactl get-default-source)
printf '%s\n' "$original_source"
```

2. List available input sources:

```sh
pactl list short sources
```

3. Ensure at least two non-monitor sources are available.

4. Start the program with a socket:

```sh
/tmp/waybar-pulseaudio-sources --sock /tmp/waybar-pulseaudio-sources-test.sock
```

5. In another shell, switch sources:

```sh
/tmp/waybar-pulseaudio-sources switch --sock /tmp/waybar-pulseaudio-sources-test.sock
```

6. Confirm the default source changed:

```sh
pactl get-default-source
```

7. Confirm the program emits an updated JSON line.

8. Restore the original default source, stop the test process with `Ctrl-C`, and remove the socket:

```sh
pactl set-default-source "$original_source"
rm -f /tmp/waybar-pulseaudio-sources-test.sock
```

# PulseAudio Event Updates

This test may change the default PulseAudio source and source mute state.

1. Record the current default source:

```sh
original_source=$(pactl get-default-source)
printf '%s\n' "$original_source"
```

2. Start the program with a socket and a template that exposes source state data:

```sh
/tmp/waybar-pulseaudio-sources --sock /tmp/waybar-pulseaudio-sources-test.sock --text '{{.Muted}}|{{.State}}|{{.Available}}' --class '{{.State}}'
```

3. Change the default source using `pactl` or `pavucontrol`.

4. Confirm the program emits a new JSON line without restarting or polling.

5. Record the current default source and its mute state, then mute and unmute it:

```sh
event_source=$(pactl get-default-source)
event_source_mute=$(pactl get-source-mute "$event_source")
printf '%s\n' "$event_source"
printf '%s\n' "$event_source_mute"
```

6. Confirm the rendered text changes from `false||true` when unmuted to `true|muted|true` when muted. Confirm the `class` field is omitted when unmuted and changes to `muted` when muted.

7. Restore the muted source's original mute state and the original default source, stop the test process with `Ctrl-C`, and remove the socket:

```sh
case "$event_source_mute" in
  *yes) pactl set-source-mute "$event_source" 1 ;;
  *no) pactl set-source-mute "$event_source" 0 ;;
esac
pactl set-default-source "$original_source"
rm -f /tmp/waybar-pulseaudio-sources-test.sock
```

# Monitor Source Exclusion

This test may change the default PulseAudio source.

1. Record the current default source:

```sh
original_source=$(pactl get-default-source)
printf '%s\n' "$original_source"
```

2. List sources:

```sh
pactl list short sources
```

3. Confirm sources ending in `.monitor` are present only as monitor sources.

4. Start the program with a socket:

```sh
/tmp/waybar-pulseaudio-sources --sock /tmp/waybar-pulseaudio-sources-test.sock
```

5. Cycle sources:

```sh
/tmp/waybar-pulseaudio-sources switch --sock /tmp/waybar-pulseaudio-sources-test.sock
```

6. Confirm monitor sources are not selected as the default by this program.

7. Restore the original default source, stop the test process with `Ctrl-C`, and remove the socket:

```sh
pactl set-default-source "$original_source"
rm -f /tmp/waybar-pulseaudio-sources-test.sock
```

# Unavailable Status

This test interrupts PulseAudio availability.
It may interrupt desktop audio and may not restore cleanly. Do not run it during routine validation; run it only when intentionally testing PulseAudio recovery.

1. Start the program with a socket and a template that exposes unavailable-state data:

```sh
/tmp/waybar-pulseaudio-sources --sock /tmp/waybar-pulseaudio-sources-test.sock --text '{{.Index}}|{{.State}}|{{.Available}}' --class '{{.State}}' --tooltip '{{.Desc}}'
```

2. Stop or restart PulseAudio/PipeWire PulseAudio compatibility in the local user session.

3. Confirm the program emits unavailable status:

```json
{"text":"-1|unavailable|false","tooltip":"...","class":"unavailable"}
```

4. Restore PulseAudio availability.

5. Confirm the program reconnects and emits normal source status.

6. Repeat the test, but switch sources while unavailable:

```sh
/tmp/waybar-pulseaudio-sources switch --sock /tmp/waybar-pulseaudio-sources-test.sock
```

7. Confirm the program retries connection immediately. After PulseAudio is restored, confirm the pending click is applied as one source-cycle request.

8. Stop the test process with `Ctrl-C` and remove the socket:

```sh
rm -f /tmp/waybar-pulseaudio-sources-test.sock
```

# Socket Error

1. Start with a socket path whose parent directory does not exist:

```sh
/tmp/waybar-pulseaudio-sources --sock /tmp/missing-dir/waybar-pulseaudio-sources.sock
```

2. Confirm the program exits with a fatal stderr message reporting that the socket could not be bound.

3. Create a regular file at the socket path:

```sh
printf 'not a socket\n' > /tmp/waybar-pulseaudio-sources-test.sock
/tmp/waybar-pulseaudio-sources --sock /tmp/waybar-pulseaudio-sources-test.sock
```

4. Confirm startup fails without removing or modifying the regular file.

5. Remove the temporary file:

```sh
rm -f /tmp/waybar-pulseaudio-sources-test.sock
```

# Duplicate Output

This test writes a temporary log file and runs until stopped.

1. Start the program and capture stdout:

```sh
/tmp/waybar-pulseaudio-sources --sock /tmp/waybar-pulseaudio-sources-test.sock | tee /tmp/waybar-pulseaudio-sources.log
```

2. Trigger PulseAudio events that do not change the rendered default source state.

3. Confirm duplicate JSON lines are not repeatedly written for the same rendered state.

4. Stop the process with `Ctrl-C` and remove temporary files:

```sh
rm -f /tmp/waybar-pulseaudio-sources-test.sock /tmp/waybar-pulseaudio-sources.log
```
