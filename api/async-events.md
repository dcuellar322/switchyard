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
