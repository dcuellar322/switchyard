# Migrating alpha and beta data to v1

Normal v1 startup migrates an older Switchyard database automatically while
holding the single-daemon lock. Before the first schema change, it runs SQLite
`quick_check`, creates a consistent private backup named
`switchyard.db.v<old>.pre-v<new>.bak`, verifies that backup, and only then
applies ordered embedded migrations. A newer-than-supported database is never
opened for mutation.

For an explicit offline review:

```bash
switchyard data inspect
switchyard data backup --output /private/path/switchyard-before-v1.bak
switchyard data migrate
switchyard data migrate --write
```

`data inspect` and the default `data migrate` are read-only. `--write` refuses
to run while `daemon.lock` exists. Stop Switchyard first; do not delete a live
lock. The automatic backup is non-overwriting, so a failed prior attempt stays
available for inspection rather than being replaced.

Projects, accepted manifest snapshots, audit history, operations, workspaces,
plugin registrations, and diagnostics remain in SQLite. Portable manifests
remain in repositories. Browser settings remain in that browser, and desktop
preferences remain in the platform application-config directory; neither is
rewritten by database migration.

Downgrade is restore-based, not a reverse mutation. Stop the daemon, keep the
new database, restore the pre-migration backup to a separate data directory,
and run the older matching binary against that copy. Never point an old binary
at a newer schema; it will reject it. Verify the copy with `data inspect`
before changing which data directory is active.
