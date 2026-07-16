# Health and log observability

Phase 7 implements ADR-0014 as a boundary above both runtime drivers. Docker
and native-process adapters still emit raw runtime observations and log lines;
they do not own retention, health policy, redaction, or browser state.

## Log pipeline

```text
Docker/process raw stream
          |
          v
canonical redactor
          |
          +--> bounded per-service ring
          +--> rotating private NDJSON segment
          +--> live WebSocket subscribers
          +--> query/export APIs
```

The redactor runs before every sink. Built-in rules cover bearer credentials,
secret-like environment assignments, credential-bearing URLs, and common
access-key forms. Repeated `switchyard daemon --redact-pattern` flags add local
regular expressions. Resolved keychain values are registered with the redactor
in memory at launch and are never written as redaction configuration.

SQLite stores segment identity, run and operation correlation, line indexes,
time/sequence ranges, sizes, and checksums. Log content remains in mode `0600`
NDJSON files below the private data directory. A SQLite transaction remains
uncommitted until its corresponding line is appended, so queries never observe
an index ahead of the file. Stable content digests discard the initial-tail
overlap produced when a collector reconnects.

Segments rotate by byte cap. Closed segments have SHA-256 checksums and are
deleted oldest-first by both age and aggregate byte cap. File deletion is
staged so a failed metadata update can restore the original path, and startup
repairs interrupted staged deletions. Compression, full-text search, and
cross-project analytics remain outside this phase.

`GET /api/v1/projects/{projectId}/logs` queries the persisted stream by service,
run, operation, time, and tail limit. The export endpoint emits already-redacted
plain text or NDJSON. `/ws/v1/logs` subscribes before replay, resumes after a
durable sequence, and removes replay/live overlap before delivery. Subscriber
overflow closes the socket so the same cursor protocol can recover the gap.

## Health evaluation

Trusted manifests can declare HTTP status and JSON-path assertions, TCP
connects, process-alive checks, Docker health, shell-free commands, and
composite `all`/`any` checks. Each declaration has an ID plus initial delay,
interval, timeout, retry count, severity, and required flag. HTTP and TCP checks
are loopback-only at both manifest validation and execution.

The scheduler:

- applies initial delay and stable per-project jitter;
- honors the shortest configured interval in each project;
- evaluates up to four projects concurrently;
- gives every individual probe a strict context deadline;
- re-observes Docker/process state between readiness retries;
- persists bounded, sanitized results and cancels with daemon shutdown.

Lifecycle start, restart, and rebuild operations execute first, then wait for
required checks. A readiness failure fails the operation but does not stop or
misclassify the running service. The HTTP runtime view overlays `degraded` only
when a required or non-informational health result is unhealthy. The health response
separately marks observations as `connected`, `stale`, or `disconnected`.

## UI behavior

The project diagnostics view shows runtime services, health results, observer
freshness, and a persisted/live log preview. Log rows are keyed by durable
sequence. The browser reconnects with its last sequence, merges the HTTP
snapshot with WebSocket replay, and deduplicates overlap. Disconnected and
stale observations remain visible as warnings rather than being rendered as a
false stopped or healthy state.
