# AI-assisted onboarding

Phase 11 adds an optional provider step after deterministic discovery. AI is a
proposal source, not an authority: the deterministic scanner remains usable
with every provider disabled, and only the catalog's ordinary human approval
transaction can create a trusted manifest snapshot.

## Boundary and data flow

```text
deterministic proposal
        |
        v
bounded structural sanitizer + canonical redactor
        |
        +--> exact JSON preview, byte count, redaction count, SHA-256 receipt
        |
        v
provider-neutral ProposalProvider
        |
        v
JSON Schema validation + evidence-ID verification
        |
        v
typed merge + dry-run + immutable untrusted proposal revision
        |
        v
browser/CLI human approval
```

The evidence bundle contains relative source paths, exact source ranges,
structured deterministic findings, bounded excerpts, the deterministic
candidate, confidence, and unresolved fields. It never contains the selected
absolute repository root. Environment values, secret-reference keys and
accounts, common credential formats, configured redaction patterns, and known
secret values are removed before preview, persistence, or dispatch. `.env`,
secret-named files, and full existing manifests are never excerpted.

The preview's `encoded` object is byte-for-byte the JSON sent to the provider.
The run stores the same bytes and their SHA-256 digest. Raw provider output is
validated and discarded; it is not persisted as a log or diagnostic payload.

## Provider isolation

All providers implement the consumer-owned `ProposalProvider` interface under
`internal/agents/application`. Provider-specific protocol code stays under
`internal/agents/providers`.

- Codex runs with `exec`, `--sandbox read-only`, an empty temporary working
  root, ephemeral sessions, ignored user configuration/rules, and
  `--output-schema`. The prompt travels on stdin, not argv.
- Claude Code runs in print mode with plan permissions, bare/no-session mode,
  an empty tool set, an empty temporary working root, `--json-schema`, and
  explicit turn/cost limits.
- The OpenAI-compatible adapter calls only the daemon-configured endpoint and
  model. It uses Chat Completions Structured Outputs, disables redirects,
  disables proxies for plaintext local endpoints, rejects URL credentials,
  and permits plaintext HTTP only for localhost or private IP literals.

CLI subprocesses inherit an allowlist rather than the daemon environment. A
repository cannot select or shadow the executable: bare command names resolve
only through absolute `PATH` entries, and configured paths must be absolute.
Provider stderr and HTTP errors are bounded and redacted.

The daemon flags are:

```text
--ai-codex-executable /absolute/path/or/bare-name
--ai-codex-model optional-model
--ai-claude-executable /absolute/path/or/bare-name
--ai-claude-model optional-model
--ai-openai-endpoint https://provider.example/v1
--ai-openai-model configured-model
--ai-openai-api-key-env OPENAI_API_KEY
```

The API key value is read only from the named process environment variable and
is never persisted or returned. Local endpoints may omit it.

## Merge and approval policy

Providers return a complete candidate plus claims at a fixed set of JSON
Pointers. Claims may reference only evidence IDs present in the sent bundle.
Switchyard computes confidence from those evidence records and caps provider
confidence below deterministic authority; model-reported confidence is not
accepted.

High-confidence deterministic values are retained and surfaced as conflicts
with `kept_deterministic` resolution. New commands, process definitions,
actions, Compose files/services, ports, and endpoints must exactly match
deterministic evidence. Provider requests for shell execution, environment
values, secret references, arbitrary working directories, health commands,
lifecycle changes, or resource policy changes are rejected and shown at field
level.

The merged candidate receives the canonical schema/domain/path/tool/port/health
validation dry-run. It becomes a new `proposed` catalog revision only; it never
writes the repository or calls acceptance. Agent identities are explicitly
forbidden from accepting assisted revisions, including admin MCP profiles, so
generation cannot self-approve.

Generation runs through the durable operation coordinator. Evidence/output
bytes, timeout, turns, output tokens, and optional cost are bounded. Context
cancellation terminates HTTP calls or CLI process groups and persists a
cancelled receipt without changing the deterministic proposal.
