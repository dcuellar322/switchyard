# Signed team configuration and encrypted sync

Switchyard can share portable project templates, policy packs, curated plugin
metadata, and enterprise configuration without a hosted account. This feature
is optional: a fresh installation has no trusted publisher, bundle, sync key,
registry, or remote dependency.

## Trust model

Every `switchyard.bundle/v1` document is signed with Ed25519 over a canonical
envelope. Installation succeeds only when all of these checks pass:

1. the publisher's exact public key was trusted with explicit confirmation;
2. the publisher ID is the SHA-256-derived identity of that key;
3. the bundle signature, schema, kind, ID, timestamps, and expiration pass;
4. its JSON payload is bounded, kind-specific, and portable; and
5. the payload contains no secret-bearing fields, private keys, or absolute
   host paths.

Signing private keys and age identities are CLI-owned files and are never sent
to the daemon. Generate a new owner-only signing key, verify its public
identity, and trust the public half through an independently authenticated
channel:

```text
switchyard team key generate --output maintainer.signing-key.json
switchyard team key show maintainer.signing-key.json
switchyard team publisher trust --name "Maintainers" \
  --public-key <reviewed-base64-public-key> --yes
```

Do not commit private signing-key or age-identity files. Rotate a compromised
publisher by ceasing to install its bundles and distributing a newly generated
public identity out of band. The current v1 trust store is additive; removing a
publisher is deliberately not an unaudited shortcut.

## Bundle workflow

Payloads are ordinary bounded JSON documents. Sign and install them as separate
reviewed operations:

```text
switchyard team bundle sign policy-pack org-policy \
  --name "Organization policy" --version 1.0.0 \
  --payload policy.json --key maintainer.signing-key.json \
  --output org-policy.bundle.json

switchyard team bundle install org-policy.bundle.json --yes
switchyard team bundle list
switchyard team policy
switchyard team registry
```

Project-template payloads contain a manifest and a bounded variable catalog.
Rendering replaces only declared `{{variable}}` placeholders and validates the
complete result with the normal project-manifest validator before it can be
written:

```text
switchyard team template render template.go-service \
  --set name=payments --set root=/work/payments \
  --output switchyard.project.json
```

Policy packs use allowlists. Multiple packs and enterprise configurations are
combined by restrictive intersection, so an empty allowed list denies that
optional capability. Team policy can restrict remote inventory and lifecycle
actions, registry publishers, and whether a user may opt in to anonymous
metrics. It never broadens the built-in permission model.

A signed plugin registry supplies metadata, HTTPS download locations, and
SHA-256 digests only. Registry discovery does not download, install, trust,
enable, or grant capabilities to a plugin. The normal exact-fingerprint and
least-privilege plugin review remains mandatory.

## Encrypted configuration sync

Sync exports only trusted public publishers and their verified portable
bundles. It excludes projects, repository paths and contents, machine or tunnel
credentials, fleet registrations, operations, logs, terminals, environment
values, settings, and runtime state.

Create an age X25519 identity on the receiving machine and share only its
recipient string:

```text
switchyard team sync key-generate --output receiving-machine.agekey
switchyard team sync export --recipient age1... --output team-config.age
switchyard team sync preview team-config.age \
  --identity receiving-machine.agekey
switchyard team sync import team-config.age \
  --identity receiving-machine.agekey --yes
```

The encrypted file uses the standard armored age format. Preview decrypts only
in memory, validates every public identity and signature, lists bundle IDs, and
warns about replaced publisher metadata or signed bundle revisions. Import
requires a second explicit confirmation. Encryption protects confidentiality;
signature verification and the review step establish publisher trust.

Back up signing keys and age identities with the same controls used for other
developer credentials. Losing an age identity makes its encrypted exports
unrecoverable. Losing a signing key does not affect installed bundles, but new
revisions must use a newly trusted publisher.

