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
