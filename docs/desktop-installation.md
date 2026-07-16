# Desktop and headless installation

## macOS desktop

Published desktop releases are expected to provide a signed, notarized DMG.
Open the DMG, drag Switchyard to Applications, and launch it normally. The
application starts or attaches to the same per-user daemon used by the CLI.
The menu-bar tray remains available when the main window is hidden.

Use **Launch at login** in the tray only if Switchyard should start for that
macOS account. Use **Keep running when window closes** to choose between
hide-to-tray and exiting the desktop adapter. **Quit Switchyard** exits the
adapter but intentionally leaves an already running daemon available to CLI,
browser, MCP, and terminal sessions.

Release builds check the configured HTTPS release feed only when **Check for
Updates…** is selected. Every downloaded bundle must pass the embedded public
key signature check before installation. Debug and locally unsigned builds do
not enable updates.

Deep links use these bounded forms:

```text
switchyard://project/<project-id>
switchyard://workspace/<workspace-id>
```

An already running instance receives the link, verifies daemon compatibility,
opens the corresponding local route, and focuses the existing window.

## Build from source

Install the pinned Go, Node/pnpm, and Rust toolchains, then run:

```bash
pnpm install --frozen-lockfile
make desktop-quality
make desktop-build
```

The native bundles are written under
`desktop/src-tauri/target/debug/bundle/`. A source build is intentionally
unsigned and has no update authority. Maintainers use the protected
`desktop-release` GitHub environment to supply Apple signing/notarization and
Minisign updater credentials; secrets are never committed or placed in an
application config file.

## Headless or CLI-only use

The Go binary is the supported installation for SSH hosts, CI workers, and
developers who do not want a native shell:

```bash
make build
install -m 0755 bin/switchyard "$HOME/.local/bin/switchyard"
switchyard doctor
switchyard ui
```

`switchyard doctor` and all other client commands start the per-user daemon on
demand. No Tauri or Rust runtime is required by the built Go binary.

## Uninstall and data choices

Removing `Switchyard.app` removes only the desktop adapter. It does not delete
repositories, containers, volumes, project state, logs, or the daemon database.
This is the safe default and allows a later reinstall or continued CLI use.

On macOS, durable control-plane data is under:

```text
~/Library/Application Support/Switchyard/
```

Desktop-only preferences are under the per-application Tauri config directory
for `dev.switchyard.desktop`. Launch-at-login can be disabled from the tray
before uninstalling; macOS Login Items can also remove it afterward.

For a complete removal, first stop active project/workspace operations and
quit the desktop adapter. Stop the daemon gracefully (for example, terminate
the `switchyard daemon` process from Activity Monitor), then choose one of:

1. Keep the `Switchyard` data directory for a future reinstall.
2. Move it to a private backup and verify the backup before deletion.
3. Permanently delete it only after reviewing `switchyard.db`, `logs/`, and
   runtime metadata. This does not remove Docker volumes or modify repositories;
   those remain explicit project-runtime decisions.

Never remove the data directory while the daemon is running. A newer database
is rejected by an older binary rather than being silently downgraded or
mutated.
