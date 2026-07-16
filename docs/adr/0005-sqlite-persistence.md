# ADR-0005: SQLite, sqlc, migrations, and file-based log segments

- Status: Accepted
- Date: 2026-07-15

## Context

Switchyard is local-first and needs durable relational state without another
service. High-volume logs should not inflate the transactional database.

## Decision

Use SQLite through a pure-Go driver, typed `sqlc` queries, embedded ordered
migrations, WAL mode, foreign keys, and explicit application transaction
boundaries. Store rotating log bodies as files and their metadata/checksums in
SQLite. Store only keychain references for secrets.

## Consequences

Installation remains self-contained and cross-compilation is practical.
Migrations and generated queries become required quality gates. SQLite is an
internal implementation detail, not a public integration contract.
