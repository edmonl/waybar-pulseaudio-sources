# Use Unix Socket Control Channel
Date: 2026-07-08

## Status
Accepted

## Context
The current click-control mechanism stores a process ID in a pidfile and uses a signal to request source switching. This can signal the wrong process if the pidfile is stale and the PID has been reused. It also makes liveness detection depend on a process ID rather than on the actual control endpoint.

The module needs a local, lightweight control path for Waybar click events.

## Decision
Use a Unix domain socket as the control channel for commands sent to the long-running module process. Remove pidfile-based switching and internal signal handling for source switching.

## Consequences
+ Switch requests target the module's actual control endpoint instead of an arbitrary process ID.
+ The command protocol can grow beyond source switching if needed.
+ Startup can detect an already-running module by connecting to the socket.
- The implementation must manage stale socket files.
- Users with custom paths must configure the same socket path for the long-running command and the `switch` command.
