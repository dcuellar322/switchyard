# Project manifest v1 reference

The portable manifest lives at `.switchyard/project.yml` and uses
`schemaVersion: switchyard.dev/v1` with `kind: Project`. A machine-local overlay
may live at `.switchyard/project.local.yml`; it must not be committed when it
contains workstation-specific paths or ports.

The generated normative JSON Schema is
`internal/manifest/schema/project.schema.json`. Unknown fields fail validation.
Effective precedence is runtime override, local overlay, portable manifest,
accepted deterministic inference, then current deterministic discovery. The
effective API retains field-level provenance.

Top-level sections are `metadata`, `repository`, `runtime`, `lifecycle`,
`services`, `ports`, `endpoints`, `actions`, and `resourcePolicy`. Runtime
drivers are `compose`, `process`, and `external`. Process commands are argument
arrays unless a manifest explicitly opts into reviewed shell behavior. Secret
values are never manifest fields; use credential-store references.

Compose `runtime.compose.profiles` is the allowlist of optional profile names
that a person may select for `start` or `rebuild`. It does not enable those
profiles by default. Deterministic Compose discovery records the names while
keeping profiled services out of the core topology.

Actions declare type, working directory, risk, timeout, and typed command or
target data. Destructive actions still require operation preview and explicit
confirmation. Health checks and resource policies affect readiness and
diagnostics but cannot grant execution authority.

Use the examples under `examples/projects`, the generated schema, and:

```bash
switchyard manifest validate <project>
switchyard manifest explain <project>
switchyard manifest diff <project>
```

Alpha/beta `switchyard.dev/v1alpha1` documents are read compatibly and can be
rewritten with the backup-producing migration command documented in
[compatibility](compatibility.md).
