export interface Integration {
  slug: string
  name: string
  summary: string
  description: string
  setup: string[]
  docsHref: string
}

export const integrations: Integration[] = [
  {
    slug: 'docker-compose',
    name: 'Docker Compose',
    summary: 'Installed Compose CLI for lifecycle, Docker API for observation.',
    description: 'Switchyard respects the user’s Docker context and Compose semantics while mapping services through canonical labels.',
    setup: ['Install Docker with the Compose plugin.', 'Add or discover a repository with a Compose file.', 'Review the normalized lifecycle plan before trust.'],
    docsHref: '/docs/architecture/docker-compose-runtime/',
  },
  {
    slug: 'native-processes',
    name: 'Native processes',
    summary: 'uv, npm, pnpm, Make, scripts, and argument-array commands.',
    description: 'Managed processes run in OS process groups or Job Objects with explicit cwd, environment, cancellation, and identity evidence.',
    setup: ['Declare or discover a start command.', 'Review working directory, arguments, ports, and health.', 'Start and follow stdout/stderr through the project log stream.'],
    docsHref: '/docs/architecture/native-process-runtime/',
  },
  {
    slug: 'codex',
    name: 'Codex',
    summary: 'MCP project operations plus bounded assisted onboarding.',
    description: 'Codex uses Switchyard’s typed MCP tools for project lifecycle and its own repository tools for source work.',
    setup: ['Run `switchyard agent install codex`.', 'Choose an observe or develop profile.', 'Use MCP tools to inspect status before any mutation.'],
    docsHref: '/docs/mcp/',
  },
  {
    slug: 'claude-code',
    name: 'Claude Code',
    summary: 'Shared MCP vocabulary and provider-neutral project guidance.',
    description: 'Claude Code receives the same bounded tools, permission profiles, and audited operation contracts as other MCP clients.',
    setup: ['Run `switchyard agent install claude`.', 'Review the generated local MCP configuration.', 'Start a scoped project session or connect from Claude Code.'],
    docsHref: '/docs/mcp/',
  },
  {
    slug: 'vscode',
    name: 'VS Code',
    summary: 'Open the trusted repository or exact worktree as a declared action.',
    description: 'Editor launch remains a small platform adapter. It never moves repository or runtime policy into the UI.',
    setup: ['Install the `code` shell command.', 'Review the editor action and working directory.', 'Open from the project or workspace action menu.'],
    docsHref: '/docs/architecture/developer-workflows/',
  },
  {
    slug: 'mcp',
    name: 'Generic MCP clients',
    summary: 'Provider-neutral tools, resources, prompts, and progress.',
    description: 'Any compatible local MCP client can connect over stdio and receive the profile-scoped Switchyard surface.',
    setup: ['Run `switchyard mcp serve --transport stdio`.', 'Pass an explicit permission profile and project scope.', 'Poll or follow durable operations by ID.'],
    docsHref: '/docs/mcp/',
  },
]
