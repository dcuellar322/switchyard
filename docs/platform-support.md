# Platform support

Switchyard v1 supports native arm64 and amd64 installations on the following
platform families. The control plane, CLI, browser UI, and MCP server share the
same support level; desktop packaging depends on the platform row.

| Platform | Supported baseline | Desktop bundles | Native capabilities |
|---|---|---|---|
| macOS | macOS 13 or newer | signed/notarized `.app` and DMG | Unix socket, process groups, PTY, port APIs, Keychain, notifications, launch agent |
| Linux | Ubuntu 22.04+/Debian 12+ and current Fedora-family desktops | AppImage, deb, rpm | Unix socket, process groups, PTY, portable port APIs, Secret Service, notifications, XDG autostart |
| Windows | Windows 11 and Windows Server 2022 with Desktop Experience | signed MSI and NSIS | owner-only named pipe, Job Objects, ConPTY, portable port APIs, Credential Manager, notifications, startup task |
| WSL2 | supported distributions on Windows 11 | use the Linux binary inside WSL | Linux socket/process/PTY behavior inside the distribution; explicit interop limits below |

FreeBSD and other Unix-like systems may compile through conservative adapters
but are not v1 release targets.

## Primary workflow matrix

Every release candidate must pass project discovery and approval, Compose and
native-process lifecycle, status/log/metric inspection, ports, actions,
terminal resize/reconnect/termination, MCP observe and authorized mutation,
plugin conformance, data backup/migration, desktop attach, autostart, native
notification, upgrade, uninstall-with-data-retained, and clean reinstall.

The GitHub `CI` workflow executes native adapter tests on macOS, Linux, and
Windows and builds the sidecar on all three. The release workflow builds each
native installer, produces SBOMs, signs applicable platform and updater
artifacts, and creates attestations. The detailed operator checklist is in
[release engineering](release.md).

## WSL2 behavior

Install and run the Linux `switchyard` binary inside each WSL distribution
whose repositories it manages. Its data directory, Unix socket, child
processes, terminals, Docker context, and repository paths stay inside that
distribution's security boundary.

The Windows desktop does not attach directly to a WSL Unix socket, and a WSL
CLI does not attach to the Windows named pipe. Use `switchyard ui` inside WSL
and open its loopback URL through Windows' localhost forwarding. Docker Desktop
integration works when `docker` and `docker compose` work inside that same WSL
distribution. Windows notifications and startup settings are not projected
into WSL; use the Linux desktop integration only when the distribution has a
supported graphical session.

Repositories under the WSL filesystem are recommended. `/mnt/c` repositories
work but inherit Windows filesystem performance, case-sensitivity, permissions,
and file-watching limitations. Switchyard reports adapter failures rather than
claiming parity when the host disables localhost forwarding, systemd user
services, Secret Service, or Docker integration.
