# Switchyard asynchronous event contract

The authenticated event stream is available at `/ws/v1/events`. Browser clients
send the same-origin session cookie and an exact same-origin `Origin` header.
Privileged local clients use local IPC. Clients reconnect with
`?after=<last-sequence>` to replay durable operation progress.

```json
{
  "id": "random stable event id",
  "type": "system.connected",
  "occurredAt": "2026-07-15T12:00:00Z",
  "sequence": 4132,
  "projectId": "project-id",
  "operationId": "op_id",
  "payload": {
    "apiVersion": "v1"
  }
}
```

The daemon persists events before live publication. A connection receives a
`system.connected` envelope, at most 500 missed events, and then live events.
`system.refresh_required` means the bounded replay window was exceeded and the
client must refresh query state before continuing. Unknown event types are
ignorable.

Project actions use the same durable operation envelopes. An `action.run`
operation emits ordinary queued, running, progress, and terminal events with
its operation and project IDs. Action output and environment values are never
included in these events; clients refresh the operation and action list using
the generated API after reconnect.

## Project log stream

The authenticated log stream is available at:

```text
/ws/v1/logs?projectId=<id>&service=<optional-service>&after=<last-sequence>
```

It uses the same browser session, origin checks, and loopback-only transport as
the event stream. The first message is `{ "type": "logs.connected",
"sequence": N }`. It is followed by every persisted redacted entry after `N`
and then live entries from the same canonical pipeline. Log entries carry a
monotonic `sequence`; clients ignore sequences at or below their cursor and
reconnect with the greatest received value. The server subscribes before
replay, so entries produced during replay are not missed, and it filters replay
overlap before live delivery. Subscriber overflow closes with a retryable
status so cursor replay repairs the gap.
