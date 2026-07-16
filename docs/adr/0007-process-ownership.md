# ADR-0007: Native process ownership and reconciliation

- Status: Accepted
- Date: 2026-07-15

## Context

PIDs are reused, parent processes can exit before children, and projects may be
started outside Switchyard. Treating a stored PID as ownership risks signaling
the wrong process or reporting false state.

## Decision

Start managed commands without a shell by default in an OS process group or Job
Object. Persist run ID, PID, executable, start time, working directory, and
available platform identity evidence. Reconcile using that fingerprint plus
ports and declared service metadata. Gracefully terminate the process tree,
then escalate after a configured timeout. External processes remain observed,
not owned, until adoption is safe and explicit.

## Consequences

Process adapters require platform-specific identity and tree tests. Some live
state remains honestly unknown on constrained platforms. Restart policy is
opt-in and cannot conceal repeated crashes.
