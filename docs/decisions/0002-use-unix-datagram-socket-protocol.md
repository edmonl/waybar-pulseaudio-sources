# Use Unix Datagram Socket Protocol
Date: 2026-07-08

## Status
Accepted

## Context
The control channel only needs to send a small local command from a short-lived click command to the long-running module. A stream socket would require connection accept handling and a response protocol that the current source-switching workflow does not need.

## Decision
Use Unix datagram sockets for the module control protocol.

## Consequences
+ One click command maps to one datagram, which keeps command framing simple.
+ The long-running module can read commands directly without accepting connections.
+ The `switch` command can exit after sending the datagram.
- The command does not receive confirmation that PulseAudio completed the switch.
- Startup and cleanup still need to handle stale socket path files.
