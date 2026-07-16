# Switchyard asynchronous event contract

The Phase 1 event stream is available at `/ws/v1/events`. Later phases extend
the payload catalog while preserving the versioned envelope.

```json
{
  "id": "random stable event id",
  "type": "system.connected",
  "occurredAt": "2026-07-15T12:00:00Z",
  "sequence": 1,
  "payload": {
    "apiVersion": "v1"
  }
}
```

Clients treat unknown event types as ignorable and refresh server state when a
future stream indicates its replay window is unavailable.
