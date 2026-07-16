# Durable operations and local transport

Phase 2 establishes the command foundation used by later project, runtime,
workspace, UI, CLI, and agent adapters.

## Operation lifecycle

Every mutation is represented by one durable operation:

```text
queued -> running -> succeeded
                  -> failed
                  -> cancelled
                  -> partially_succeeded
queued            -> cancelled
                  -> failed
```

Terminal states never transition again. Operation steps and audit events are
append-only. State changes use a compare-and-swap update so a concurrent writer
cannot silently overwrite another terminal decision.

An idempotency key is unique within a project. Repeating the same request
returns the existing operation and never schedules a second executor. HTTP
mutation adapters require an opaque `Idempotency-Key` between 8 and 128
characters; application services enforce the durable uniqueness rule.

## Scheduling and cancellation

A keyed gate permits one active lifecycle mutation per project. Operations for
different projects can execute concurrently. Cancellation is durable before
the live executor receives context cancellation, so a daemon restart cannot
forget the request.

Daemon shutdown stops accepting requests, cancels active operation contexts,
waits for terminal transitions, closes HTTP and IPC listeners, and only then
closes SQLite.

## Restart behavior

Recovery is deterministic:

| Persisted state | Recovery action |
|---|---|
| `queued` | Resume through the registered executor. |
| `queued` with cancellation requested | Mark `cancelled`. |
| `running` | Mark `failed` with `DAEMON_RESTARTED`; never guess whether an external side effect completed. |
| terminal | Leave unchanged. |

Later runtime executors reconcile observed external state before a user retries
an interrupted operation.

## Events and replay

Operation events are inserted into the SQLite journal before live publication.
The sequence is monotonic. WebSocket clients reconnect with
`/ws/v1/events?after=<last-sequence>`. The daemon replays at most 500 events;
`system.refresh_required` tells a client to refresh query state when that bound
is exceeded. Slow live subscribers are disconnected and recover through replay.

## Local transport and browser sessions

CLI traffic uses HTTP semantics over a user-permissioned Unix-domain socket on
macOS and Linux. The platform adapter reserves the Windows named-pipe identity;
the Windows implementation is completed in Phase 18. The socket and database
are mode `0600`, and the data directory is `0700`.

The loopback browser API is a separate router:

1. `switchyard ui` requests a one-time, one-minute bootstrap token over local
   IPC.
2. The browser exchanges that token once for an HttpOnly, SameSite=Strict
   session cookie and an eight-hour CSRF token.
3. Browser queries require the session cookie. Mutations also require the CSRF
   token and an idempotency key.
4. WebSockets require the session cookie and an exact same-origin `Origin`.

Bootstrap and session responses are not cacheable. Browser responses set a
restrictive content-security policy, no-referrer policy, MIME sniffing
protection, and frame denial.
