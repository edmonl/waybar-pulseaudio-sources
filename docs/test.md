# Manual Test Plan

Run commands from the project root.

Before each test, record any current state the test may affect, such as the default PulseAudio source, source mute state, running test process, pidfile, or temporary log file. After each test, confirm that affected state has been restored or cleaned up.

Tests that start `/tmp/waybar-pulseaudio-sources` without `timeout` run until stopped. If a test hangs or is interrupted, stop the process with `Ctrl-C` when it is in the foreground, or use the cleanup command below when it is running in another shell.

Use this cleanup after tests that start the program and after interrupted tests:

```sh
pidfile=/tmp/waybar-pulseaudio-sources-test.pid
test ! -r "$pidfile" || kill "$(cat "$pidfile")"
rm -f "$pidfile" /tmp/waybar-pulseaudio-sources.log
```

# Build Checks

This check writes `/tmp/waybar-pulseaudio-sources`, which later tests use. Remove it after manual testing if it is no longer needed.

1. Check formatting, run tests, vet, and build:

```sh
test -z "$(gofmt -l *.go pulse/*.go)"
go test ./...
go vet ./...
go build -o /tmp/waybar-pulseaudio-sources .
```

2. If the formatting check prints files, run `gofmt -w` on those files and inspect the resulting diff before continuing.

# Startup Smoke Test

1. Start the program with a temporary pidfile:

```sh
timeout 3s /tmp/waybar-pulseaudio-sources --pidfile /tmp/waybar-pulseaudio-sources-test.pid
```

2. Confirm stdout contains one newline-delimited JSON object similar to:

```json
{"text":"65%","tooltip":"Microphone Name","class":"source","percentage":65}
```

3. Confirm the pidfile was removed after exit:

```sh
test ! -e /tmp/waybar-pulseaudio-sources-test.pid
```

4. If interrupted, run the cleanup command from the top of this file.

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

6. If interrupted, stop the process and remove the default pidfile:

```sh
pidfile="$XDG_RUNTIME_DIR/waybar-pulseaudio-sources.pid"
test ! -r "$pidfile" || kill "$(cat "$pidfile")"
rm -f "$pidfile"
```

# Disabled Pidfile

1. Start the program with pidfile output disabled:

```sh
timeout 3s /tmp/waybar-pulseaudio-sources --pidfile ''
```

2. Confirm the program starts and emits status without requiring a pidfile.

# Format Flag

1. Start the program with a custom format:

```sh
timeout 3s /tmp/waybar-pulseaudio-sources --pidfile /tmp/waybar-pulseaudio-sources-test.pid --format '{{.Desc}} {{.Volume}}%'
```

2. Confirm stdout contains a JSON object whose `text` field contains the default source description and volume.

3. Start the program with an invalid format:

```sh
/tmp/waybar-pulseaudio-sources --pidfile '' --format '{{.Description}}'
```

4. Confirm startup fails with a fatal `--format` error.

5. Start the program with an empty format:

```sh
/tmp/waybar-pulseaudio-sources --pidfile '' --format ''
```

6. Confirm startup fails with a fatal `--format` error.

# Invalid Default Pidfile Directory

1. Start without `--pidfile` and with `$XDG_RUNTIME_DIR` unset:

```sh
env -u XDG_RUNTIME_DIR /tmp/waybar-pulseaudio-sources
```

2. Confirm startup fails because pidfile output is enabled but no default path can be determined.

3. Start without `--pidfile` and with `$XDG_RUNTIME_DIR` set to a relative path:

```sh
env XDG_RUNTIME_DIR=relative /tmp/waybar-pulseaudio-sources
```

4. Confirm startup fails because the default pidfile directory is not absolute.

# Explicit Pidfile Path

1. Start the program with a relative pidfile path:

```sh
(cd /tmp && timeout 3s /tmp/waybar-pulseaudio-sources --pidfile waybar-pulseaudio-sources-test.pid)
```

2. Confirm the pidfile was written relative to `/tmp` and removed after exit:

```sh
test ! -e /tmp/waybar-pulseaudio-sources-test.pid
```

3. Start the program with a blank non-empty pidfile value:

```sh
/tmp/waybar-pulseaudio-sources --pidfile '   '
```

4. Confirm startup fails because the explicit pidfile value is blank after trimming.

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

4. Start the program with a pidfile:

```sh
/tmp/waybar-pulseaudio-sources --pidfile /tmp/waybar-pulseaudio-sources-test.pid
```

5. In another shell, send `SIGUSR1`:

```sh
kill -SIGUSR1 "$(cat /tmp/waybar-pulseaudio-sources-test.pid)"
```

6. Confirm the default source changed:

```sh
pactl get-default-source
```

7. Confirm the program emits an updated JSON line.

8. Restore the original default source and stop the test process:

```sh
pactl set-default-source "$original_source"
pidfile=/tmp/waybar-pulseaudio-sources-test.pid
test ! -r "$pidfile" || kill "$(cat "$pidfile")"
rm -f "$pidfile"
```

# PulseAudio Event Updates

This test may change the default PulseAudio source and source mute state.

1. Record the current default source:

```sh
original_source=$(pactl get-default-source)
printf '%s\n' "$original_source"
```

2. Start the program with a pidfile:

```sh
/tmp/waybar-pulseaudio-sources --pidfile /tmp/waybar-pulseaudio-sources-test.pid
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

6. Confirm the `class` changes between `source` and `muted`.

7. Restore the muted source's original mute state and the original default source, then stop the test process:

```sh
case "$event_source_mute" in
  *yes) pactl set-source-mute "$event_source" 1 ;;
  *no) pactl set-source-mute "$event_source" 0 ;;
esac
pactl set-default-source "$original_source"
pidfile=/tmp/waybar-pulseaudio-sources-test.pid
test ! -r "$pidfile" || kill "$(cat "$pidfile")"
rm -f "$pidfile"
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

4. Start the program with a pidfile:

```sh
/tmp/waybar-pulseaudio-sources --pidfile /tmp/waybar-pulseaudio-sources-test.pid
```

5. Cycle sources with `SIGUSR1`:

```sh
kill -SIGUSR1 "$(cat /tmp/waybar-pulseaudio-sources-test.pid)"
```

6. Confirm monitor sources are not selected as the default by this program.

7. Restore the original default source and stop the test process:

```sh
pactl set-default-source "$original_source"
pidfile=/tmp/waybar-pulseaudio-sources-test.pid
test ! -r "$pidfile" || kill "$(cat "$pidfile")"
rm -f "$pidfile"
```

# Unavailable Status

This test interrupts PulseAudio availability.
It may interrupt desktop audio and may not restore cleanly. Do not run it during routine validation; run it only when intentionally testing PulseAudio recovery.

1. Start the program with a pidfile:

```sh
/tmp/waybar-pulseaudio-sources --pidfile /tmp/waybar-pulseaudio-sources-test.pid
```

2. Stop or restart PulseAudio/PipeWire PulseAudio compatibility in the local user session.

3. Confirm the program emits unavailable status:

```json
{"text":"Unavailable ","tooltip":"...","class":"unavailable"}
```

4. Restore PulseAudio availability.

5. Confirm the program reconnects and emits normal source status.

6. Repeat the test, but send `SIGUSR1` while unavailable:

```sh
kill -SIGUSR1 "$(cat /tmp/waybar-pulseaudio-sources-test.pid)"
```

7. Confirm the program retries connection immediately. After PulseAudio is restored, confirm the pending click is applied as one source-cycle request.

8. Stop the test process and remove its pidfile:

```sh
pidfile=/tmp/waybar-pulseaudio-sources-test.pid
test ! -r "$pidfile" || kill "$(cat "$pidfile")"
rm -f "$pidfile"
```

# Pidfile Error

1. Start with a pidfile path whose parent directory does not exist:

```sh
/tmp/waybar-pulseaudio-sources --pidfile /tmp/missing-dir/waybar-pulseaudio-sources.pid
```

2. Confirm the program exits with a fatal stderr message similar to:

```text
waybar-pulseaudio-sources: write pidfile: open /tmp/missing-dir/waybar-pulseaudio-sources.pid: no such file or directory
```

3. No cleanup is needed unless the command unexpectedly created files.

# Duplicate Output

This test writes a temporary log file and runs until stopped.

1. Start the program and capture stdout:

```sh
/tmp/waybar-pulseaudio-sources --pidfile /tmp/waybar-pulseaudio-sources-test.pid | tee /tmp/waybar-pulseaudio-sources.log
```

2. Trigger PulseAudio events that do not change the rendered default source state.

3. Confirm duplicate JSON lines are not repeatedly written for the same rendered state.

4. Stop the process and remove temporary files:

```sh
pidfile=/tmp/waybar-pulseaudio-sources-test.pid
test ! -r "$pidfile" || kill "$(cat "$pidfile")"
rm -f "$pidfile" /tmp/waybar-pulseaudio-sources.log
```
