# Embedded terminal and agent sessions

Switchyard owns interactive PTYs in the Go control plane. The browser is a
bounded input/output adapter; Vue, xterm.js, HTTP, and WebSocket handlers do not
resolve commands or own process lifetime.

```text
browser xterm.js
  | authenticated same-origin WebSocket
  | binary input/output + JSON resize
  v
terminal application service
  | owner check, scrollback, lifecycle, audit
  +--> trusted launch resolver --> catalog/actions/environments
  +--> Unix PTY adapter --------> owned process group
  +--> SQLite repository -------> metadata only
```

## Typed launch boundary

The public create contract has no command field. A request selects exactly one
reviewed capability:

| Kind | Resolution |
|---|---|
| `shell` | `sh`, `bash`, or `zsh` login shell at the trusted checkout root |
| `service` | `docker compose exec` for a service declared by the accepted manifest |
| `database` | one of `psql`, `mysql`, `redis-cli`, `mongosh`, or `sqlite3` in a declared service |
| `agent` | configured Codex or Claude Code executable at the selected checkout |
| `action` | an accepted manifest action explicitly classified `interactive` |

Compose files are containment-checked against the selected project or worktree
root. Non-shell actions preserve argument arrays. Shell actions must be
explicit, contain one reviewed string, and cannot request `sudo` or `doas`.
No generic terminal, SQL, Docker, or shell tool is exposed through MCP.

## Ownership and transport

Every session is owned by the authenticated local principal that created it:
the browser session ID, privileged IPC principal, or scoped agent identity.
List, get, attach, and terminate operations enforce that owner. Browser PTY
WebSockets require the session cookie and an exact same-origin `Origin` before
the application attach is attempted.

Server frames are JSON text control messages (`ready` and `exit`) or binary PTY
bytes. Client frames are binary PTY input or one bounded JSON `resize` message.
Input frames are limited to 64 KiB, dimensions to 500 by 300, and subscriber
queues are non-blocking. A slow browser is detached rather than allowed to
block the PTY or grow memory without bound.

## Persistence semantics

Closing a tab, navigating away, losing connectivity, or reloading detaches the
browser. It does not terminate the process. An unattached session remains live
for 30 minutes and can be reconnected from the same authenticated browser
session. Explicit termination, command exit, idle expiry, or daemon shutdown
ends the PTY process group. PTYs cannot resume after daemon restart; previously
active metadata is marked `interrupted`.

Each live session retains at most 1 MiB of reconnect scrollback in daemon
memory. SQLite stores only owner, project/worktree, kind/provider/target,
working directory, status, timestamps, byte count, truncation flag, exit code,
and redaction-safe lifecycle audit facts. It never stores input, output,
commands, arguments, or environment values.

## Browser safety and accessibility

xterm.js uses a 2,000-line presentation scrollback inside the 1 MiB server
bound. Plain and OSC 8 links accept only HTTP(S), require Command or Control,
and open with `noopener,noreferrer`. Screen-reader mode is enabled, keyboard
focus remains inside the terminal when attached, and resize follows a debounced
`ResizeObserver`. Loading, empty, detached, ended, error, and slow-consumer
states are explicit.

## Agent metadata boundary

An agent session is a terminal session with provider metadata. Switchyard
records provider, project, checkout/worktree, working directory, lifecycle,
visible-output byte count, and reconnect-buffer truncation. The capture policy
is literally `user_visible_terminal_output_only`: Switchyard neither requests
nor claims access to hidden reasoning or provider-private state. Rich provider
SDK orchestration remains outside this reliable PTY boundary.
