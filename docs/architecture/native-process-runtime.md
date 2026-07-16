# Native process runtime

Phase 6 implements accepted ADR-0007 behind the driver-neutral runtime
application boundary. The process driver owns command construction,
supervision, identity reconciliation, output capture, metrics, and external
listener classification. HTTP, CLI, catalog, and operation handlers do not
call operating-system process APIs.

## Manifest contract

A process project declares reusable process definitions and maps each product
service to one definition:

```yaml
runtime:
  driver: process
  process:
    environment:
      PORT: "18082"
    secrets:
      API_TOKEN:
        provider: keychain
        key: example-api-token
        account: developer@example.com
    processes:
      - id: api
        command: [uv, run, fastapi, dev, app/main.py, --port, "18082"]
        workingDirectory: .
        stopTimeoutSeconds: 10
        restart:
          mode: on-failure
          maxRetries: 2
          backoffSeconds: 1

services:
  - id: api
    source: {process: api}
```

Project environment is overlaid on the daemon environment, then the process
overlay wins. A secret reference wins over a plain value at the same layer and
is resolved only at launch. Secret values are never included in plans,
manifests, run records, or process metadata. macOS uses the login Keychain via
`security`; Linux uses Secret Service via `secret-tool`.

Argument arrays execute without a shell. Known shell executables, whitespace
in the executable field, and shell-control syntax in that field are rejected
unless `shell: true` is present. Explicit shell mode accepts exactly one script
string, is visible in the lifecycle preview, and uses the platform shell.
Working directories must remain inside the trusted project root.

## Ownership and persistence

Every managed launch creates a `runs` row and at least one `run_processes` row.
The fingerprint is a SHA-256 digest of canonical executable path, operating
system process start time, and canonical working directory. The persisted
record also includes PID, process-group ID, run ID, observation time, origin,
restart count, exit code, and termination reason.

A PID is never sufficient evidence. Inspection and stop operations re-read the
live executable, start time, working directory, and process group and require
the exact stored fingerprint. A reused PID becomes `stale`/`identity_lost` and
is never signalled. A bounded two-second launch-handoff window prevents a fast
launcher such as npm from being declared stale while its child fingerprint is
being recorded; ownership still requires an exact fingerprint before the
service becomes `running`.

Child members of the OS process group are discovered continuously and stored.
If the original parent exits, a verified child keeps the run active. This lets
a restarted Switchyard daemon reconcile, measure, and stop a process tree that
survived the previous daemon without claiming unrelated processes.

## Lifecycle and restart

Dependency ordering is topological and deterministic. Start walks dependencies
first; stop walks the reverse order. Cancellation of a multi-service start
rolls back every process started by that operation.

Stop sends graceful termination to the verified process group and waits for
the manifest timeout. Remaining members receive forced termination and the run
records a `_forced` reason. Restart is stop followed by start. Pause, unpause,
rebuild, teardown, and volume flags are rejected for process runtimes.

Crash restart is disabled by default. `on-failure` restarts only non-zero exits,
uses the declared bounded retry count/backoff, records every new fingerprint,
and reports the retry count. Exhausted or failed restarts preserve the original
exit code and a useful terminal reason.

## Observation, logs, metrics, and external processes

Stdout and stderr use separate inherited pipes so child output retains its
stream identity even after the launcher exits. Each bounded log entry includes
project, service, process definition, run, source, level, and timestamp. Live
metrics include CPU, resident memory, and host memory capacity for a verified
member. Per-process network byte attribution is not claimed.

An unmanaged listener is reported as `running_external` only when a declared
TCP port maps to a live PID and the declared executable matches that process or
one of its bounded ancestors/arguments. Switchyard exposes the observation but
does not create a run ID, stream its logs, or stop it. Missing permissions or
insufficient evidence produce `unknown` or `stopped`, never managed ownership.

## Platform boundary

macOS and Linux create and signal real process groups. Windows builds retain a
bounded single-process fallback; Job Object ownership, tree termination, and
full Windows acceptance coverage remain part of Phase 18 cross-platform
hardening. Interactive PTYs remain Phase 14 work.

