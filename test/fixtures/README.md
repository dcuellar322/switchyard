# Deterministic fixture projects

These repositories are inert test data until an integration test explicitly
starts one. Discovery tests may read only the bounded files selected by the
scanner allowlist; they never execute repository commands.

| Scenario from the implementation plan | Fixture |
|---|---|
| Healthy Compose lifecycle | `compose-healthy` |
| Degraded Compose health | `compose-degraded` |
| Fixed host-port collision | `compose-port-conflict` |
| uv process lifecycle | `uv-single-process` |
| npm process lifecycle | `node-single-process` |
| Compose plus process discovery | `mixed-compose-and-process` |
| Nested two-application repository | `monorepo-two-apps` |
| Externally owned runtime | `external-process` |
| Git worktree source project | `worktree-project` |
| Adversarial repository prose | `malicious-readme` |
| Live/persisted/export redaction | `secret-redaction` |

`compose-runtime`, `mixed-project`, `ai-ambiguous-project`, and
`action-project` remain focused regression fixtures used by earlier phase
tests. The canonical inventory test prevents any required scenario above from
being removed accidentally.
