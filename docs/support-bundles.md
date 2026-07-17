# Support bundles and internal debug logs

Switchyard maintains a dedicated private control-plane log at
`<data-dir>/internal.ndjson`, with one rotated segment. It receives structured
daemon events only. Project stdout/stderr, terminal output, repository text,
provider payloads, environment values, and resolved credentials are never
routed to this sink. Credential patterns and configured redaction expressions
are applied before each record is persisted. Both segments are bounded to
4 MiB and use owner-only file permissions.

Review the exact support payload without creating a file:

```bash
switchyard doctor --bundle --preview
```

The preview contains the same document later stored in `manifest.json`:

- Switchyard version, commit, API version, and database schema version;
- platform and executable adapter availability without executable paths;
- Docker Engine connectivity when host observation succeeds;
- an allowlisted configuration snapshot containing binding modes, settings
  revision, root and excluded-port counts (never root paths), preferred range,
  tool/appearance choices, retention bounds, provider/model presence, and
  booleans for remote or credential-reference configuration;
- at most 100 recent internal warnings and errors after a second redaction and
  local home/data-directory path replacement pass;
- explicit included and excluded field lists.

Write a new archive only after reviewing that preview:

```bash
switchyard doctor --bundle --output ./switchyard-support.zip
```

The command creates a mode-`0600` ZIP through a private temporary file, syncs
it, atomically renames it, refuses to overwrite an existing destination, and
prints its SHA-256 digest. The archive has exactly two entries:
`manifest.json` and `internal-errors.ndjson`. No implicit attachment or network
upload occurs.

For local troubleshooting without an archive:

```bash
switchyard debug logs --level warn --limit 200
switchyard --jsonl debug logs --level debug --limit 2000
```

`debug logs` reads the same allowlisted internal records and supports only
bounded level filters. Redaction reduces accidental disclosure but cannot
recognize every novel secret or personal value. Review output before sharing,
use private vulnerability reporting for security issues, and never substitute
a support archive for a minimal public reproduction when one can be created.
