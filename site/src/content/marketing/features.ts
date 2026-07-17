export interface Feature {
  slug: string
  title: string
  summary: string
  description: string
  proof: string[]
  docsHref: string
  icon: string
}

export const features: Feature[] = [
  {
    slug: 'projects',
    title: 'Project command center',
    summary: 'See the repository, services, health, Git, endpoints, actions, and resources as one project.',
    description:
      'Switchyard keeps the developer’s project intent above container and process details. Every screen and automation surface resolves back to one reviewed project definition.',
    proof: ['Evidence-backed onboarding', 'Trusted portable manifests', 'Honest external-runtime state'],
    docsHref: '/docs/architecture/project-onboarding/',
    icon: '◇',
  },
  {
    slug: 'runtimes',
    title: 'Compose and native runtimes',
    summary: 'Operate Docker Compose, uv, npm, Make, and other native processes from the same lifecycle.',
    description:
      'The Go control plane plans every lifecycle operation, observes Docker through its API, and owns native process trees without pretending that externally started services belong to it.',
    proof: ['Previewable lifecycle plans', 'No shell by default', 'Compose and process reconciliation'],
    docsHref: '/docs/architecture/docker-compose-runtime/',
    icon: '▷',
  },
  {
    slug: 'logs-health-resources',
    title: 'Logs, health, and resources',
    summary: 'Follow bounded redacted logs, health transitions, and resource pressure without changing tools.',
    description:
      'Live streams and retained evidence share one event model. Health describes readiness separately from process state, while storage values disclose when attribution is shared or estimated.',
    proof: ['Redaction before every sink', 'Start-and-wait health gates', 'Bounded retention'],
    docsHref: '/docs/architecture/observability/',
    icon: '≋',
  },
  {
    slug: 'ports-git-actions',
    title: 'Ports, Git, and trusted actions',
    summary: 'Catch collisions before startup and run the project actions you have already reviewed.',
    description:
      'Declarations, reservations, and live bindings stay distinct, with source evidence for every conflict. Git and action adapters expose practical context without turning Switchyard into a broad shell or Git GUI.',
    proof: ['Stopped-project reservations', 'Git porcelain v2', 'Risk-classified actions'],
    docsHref: '/docs/architecture/developer-workflows/',
    icon: '⇄',
  },
  {
    slug: 'workspaces-worktrees',
    title: 'Workspaces and worktrees',
    summary: 'Start related projects in dependency order and run isolated feature worktrees side by side.',
    description:
      'Workspace graphs coordinate health-gated startup without erasing per-project operations. Worktree environments receive exact Compose identities and port leases so parallel work remains explicit.',
    proof: ['Validated dependency DAGs', 'Partial-failure policy', 'Per-worktree port leases'],
    docsHref: '/docs/architecture/workspaces/',
    icon: '⌘',
  },
  {
    slug: 'agents-mcp',
    title: 'Codex, Claude Code, and MCP',
    summary: 'Give coding agents typed project context and approved operations instead of another unrestricted shell.',
    description:
      'Observe, develop, maintain, and admin profiles are enforced by application services. Tool responses are bounded, structured, redacted, and audited regardless of what repository text or a model requests.',
    proof: ['No generic MCP shell', 'Permission-scoped mutations', 'Schema-constrained proposals'],
    docsHref: '/docs/mcp/',
    icon: '✦',
  },
  {
    slug: 'terminals',
    title: 'Embedded and external terminals',
    summary: 'Open a real project PTY or hand work off to the terminal you already use.',
    description:
      'Authenticated terminal sessions preserve resize, Unicode, full-screen applications, bounded scrollback, and explicit ownership. External terminal handoff stays first class.',
    proof: ['Real PTY and ConPTY adapters', 'Session ownership', 'Safe link handling'],
    docsHref: '/docs/architecture/terminal-sessions/',
    icon: '>_',
  },
  {
    slug: 'plugins',
    title: 'Capability-scoped plugins',
    summary: 'Extend stable product contracts without loading arbitrary code into the daemon.',
    description:
      'Plugins run as supervised processes over a versioned protocol, declare capabilities and scopes, and must pass a conformance suite before they are trusted.',
    proof: ['Out-of-process isolation', 'Explicit capability review', 'Versioned JSON-RPC contract'],
    docsHref: '/docs/plugin-sdk/',
    icon: '⬡',
  },
  {
    slug: 'security-local-first',
    title: 'Local-first security',
    summary: 'Keep the control plane local, repositories untrusted until review, and destructive actions explicit.',
    description:
      'Essential operation works offline. Privileged clients use user-scoped IPC, browser access stays on loopback with session and CSRF checks, and secrets remain keychain references.',
    proof: ['No cloud account required', 'Explicit repository trust', 'Preview and authorization for risk'],
    docsHref: '/docs/security/threat-model/',
    icon: '◉',
  },
]
