# Privacy

Switchyard is local-first and has no required product telemetry or cloud
account. Project metadata, manifests, operations, audit history, logs,
diagnoses, feedback, recipes, plugins, and settings remain on the local machine
unless the user explicitly exports or sends them.

Deterministic discovery reads only bounded allowlisted repository files and
does not execute code. Optional AI onboarding or diagnosis sends the exact
previewed, bounded, redacted evidence bundle to the provider the user selected;
provider use is not required for any deterministic workflow. Provider terms
and retention apply after that explicit request.

Support bundles exclude source, secrets, arbitrary environment values, and
application logs by default. Logs and diagnostic evidence share the same
redaction pipeline before display, persistence, export, or provider use.
Plugins are external programs with separately reviewed fingerprints and scopes;
their own network or data behavior remains the plugin publisher's
responsibility within permissions granted by the user.

Optional peer federation sends only the bounded identity, trusted-project, and
registered-environment fields documented in the federation guide. It excludes
repository locations, source, Git changes, logs, terminal output, secrets,
environment values, and runtime-native identifiers. Switchyard does not supply
the tunnel or relay remote traffic through a hosted service.

Encrypted team sync contains only explicitly trusted public publisher records
and verified portable configuration bundles. The standard age file excludes
projects, repository paths and contents, machine credentials, fleet records,
operations, logs, terminals, environment values, and runtime state. Decryption
occurs in the CLI process and import is previewed before confirmation.

Anonymous usage metrics are off by default and have no vendor-controlled
destination. Opt-in requires an explicit HTTPS endpoint and displays the full
payload: a random anonymous installation ID, Switchyard version, operating
system, architecture, fixed operation-category counters, and generation time.
It contains no project, machine, action, plugin, provider, path, command, log,
diagnostic, or error detail. Disabling metrics clears the installation ID and
all pending counters. A signed team policy may prohibit opt-in.
