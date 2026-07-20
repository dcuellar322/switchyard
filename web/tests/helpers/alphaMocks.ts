import type { Page, Route } from '@playwright/test'

const observedAt = '2026-07-15T14:30:00Z'
const projects = [
  {
    id: 'alpha',
    slug: 'alpha-api',
    displayName: 'Alpha API',
    description: 'Primary application API',
    trustState: 'trusted',
    primaryLocation: '/Users/dev/projects/alpha-api',
    tags: ['api', 'node'],
    manifestRevision: 3,
    createdAt: observedAt,
    updatedAt: observedAt,
  },
  {
    id: 'storefront',
    slug: 'storefront',
    displayName: 'Storefront',
    description: 'Customer application',
    trustState: 'trusted',
    primaryLocation: '/Users/dev/projects/storefront',
    tags: ['web', 'vue'],
    manifestRevision: 2,
    createdAt: observedAt,
    updatedAt: observedAt,
  },
  {
    id: 'billing',
    slug: 'billing-worker',
    displayName: 'Billing Worker',
    description: 'Background jobs',
    trustState: 'trusted',
    primaryLocation: '/Users/dev/projects/billing-worker',
    tags: ['worker', 'python'],
    manifestRevision: 1,
    createdAt: observedAt,
    updatedAt: observedAt,
  },
  {
    id: 'docs',
    slug: 'docs-site',
    displayName: 'Docs Site',
    description: 'Documentation',
    trustState: 'trusted',
    primaryLocation: '/Users/dev/projects/docs-site',
    tags: ['web', 'docs'],
    manifestRevision: 1,
    createdAt: observedAt,
    updatedAt: observedAt,
  },
]

function json(route: Route, value: unknown) {
  return route.fulfill({ contentType: 'application/json', json: value })
}

function runtime(projectId: string) {
  const degraded = projectId === 'billing'
  const stopped = projectId === 'docs'
  const services = stopped
    ? []
    : [
        {
          id: projectId === 'storefront' ? 'web' : 'api',
          runtimeName: 'primary',
          state: 'running',
          health: degraded ? 'unhealthy' : 'healthy',
          ports: [
            {
              hostIp: '127.0.0.1',
              hostPort: projectId === 'storefront' ? 15173 : 18080,
              containerPort: 8080,
              protocol: 'tcp',
            },
          ],
          observedAt,
        },
        ...(projectId === 'alpha'
          ? [
              {
                id: 'database',
                runtimeName: 'postgres',
                state: 'running',
                health: 'healthy',
                ports: [
                  {
                    hostIp: '127.0.0.1',
                    hostPort: 15432,
                    containerPort: 5432,
                    protocol: 'tcp',
                  },
                ],
                observedAt,
              },
            ]
          : []),
      ]
  return {
    projectId,
    driver: projectId === 'storefront' || projectId === 'docs' ? 'process' : 'compose',
    projectIdentity: projectId,
    state: degraded ? 'degraded' : stopped ? 'stopped' : 'running',
    origin: 'switchyard',
    engine:
      projectId === 'billing'
        ? {
            connected: false,
            errorCode: 'docker_unavailable',
            errorMessage: 'Docker Desktop is not running',
          }
        : {
            connected: true,
            context: 'desktop-linux',
            serverVersion: '29.0.0',
          },
    services,
    observedAt,
  }
}

function metrics(projectId: string) {
  const base =
    projectId === 'alpha' ? 1_073_741_824 : projectId === 'storefront' ? 314_572_800 : 134_217_728
  return [
    {
      timestamp: observedAt,
      projectId,
      serviceId: projectId === 'storefront' ? 'web' : 'api',
      cpuPercent: projectId === 'alpha' ? 14.8 : 3.4,
      cpuAvailable: true,
      memoryBytes: base,
      memoryLimit: 2_147_483_648,
      memoryAvailable: true,
      networkRxBytes: 12_400_000,
      networkTxBytes: 4_500_000,
      networkAvailable: true,
      diskReadBytes: 8_200_000,
      diskWriteBytes: 2_100_000,
      diskAvailable: true,
      processCount: 2,
      restartCount: 0,
      partial: false,
    },
  ]
}

function git(projectId: string) {
  const dirty = projectId === 'storefront' ? 4 : 0
  return {
    projectId,
    repository: true,
    branch: projectId === 'billing' ? 'feature/retry-jobs' : 'main',
    detached: false,
    head: 'f6d52a0d9b3f',
    upstream: 'origin/main',
    ahead: projectId === 'billing' ? 2 : 0,
    behind: 0,
    changes: { staged: 0, modified: dirty, untracked: 0, conflicted: 0 },
    stashes: 0,
    lastCommit: {
      hash: 'f6d52a0d9b3f0000',
      shortHash: 'f6d52a0',
      author: 'Switchyard',
      subject: 'feat: polish local workflows',
      committedAt: observedAt,
    },
    remotes: [],
    worktrees: [],
    observedAt,
  }
}

function actions(projectId: string) {
  return {
    projectId,
    projectName: projects.find((item) => item.id === projectId)?.displayName ?? projectId,
    actions: [
      {
        id: 'browser',
        name: 'Open app',
        type: 'browser.open',
        command: [],
        workingDirectory: '.',
        shell: false,
        captureOutput: false,
        target: `http://127.0.0.1:${projectId === 'storefront' ? 15173 : 18080}`,
        risk: 'read_only',
        timeoutSeconds: 5,
      },
      {
        id: 'terminal',
        name: 'Open terminal',
        type: 'terminal.open',
        command: [],
        workingDirectory: '.',
        shell: false,
        captureOutput: false,
        risk: 'interactive',
        timeoutSeconds: 5,
      },
      {
        id: 'tests',
        name: 'Run tests',
        type: 'tests.run',
        command: ['make', 'test'],
        workingDirectory: '.',
        shell: false,
        captureOutput: true,
        risk: 'mutating',
        timeoutSeconds: 300,
      },
    ],
  }
}

export async function installAlphaMocks(page: Page, empty = false) {
  await page.route('**/api/v1/settings', (route) =>
    json(route, {
      settings: {
        revision: 4,
        projectRoots: ['/Users/dev/projects'],
        ports: { rangeStart: 15000, rangeEnd: 19999, excluded: [15432] },
        retention: {
          logAgeSeconds: 604800,
          logMaximumBytes: 268435456,
          metricRawSeconds: 3600,
          metricMinuteSeconds: 86400,
          metricQuarterHourSeconds: 2592000,
          maximumMetricHistoryPoints: 1000,
        },
        tools: { terminal: 'integrated', editor: 'vscode' },
        ai: {
          defaultProvider: 'codex',
          providers: [
            { id: 'codex', enabled: true, executable: 'codex' },
            { id: 'claude', enabled: true, executable: 'claude' },
            { id: 'openai-compatible', enabled: false, credentialReference: 'env:OPENAI_API_KEY' },
          ],
        },
        permissions: { defaultAgentProfile: 'observe' },
        appearance: { density: 'comfortable', timeDisplay: 'relative', theme: 'dark' },
        updatedAt: observedAt,
      },
      pendingRestart: [],
    }),
  )
  await page.route('**/api/v1/terminal-sessions**', (route) => json(route, []))
  await page.route('**/api/v1/agents/sessions**', (route) => json(route, []))
  await page.route('**/api/v1/projects/*/environments', (route) => json(route, []))
  await page.route('**/api/v1/host', (route) =>
    json(route, {
      cpuPercent: 21.4,
      memoryUsedBytes: 8_589_934_592,
      memoryTotalBytes: 34_359_738_368,
      docker: {
        connected: true,
        storageBytes: 17_179_869_184,
        reclaimableBytes: 4_294_967_296,
        attribution: 'shared',
      },
      observedAt,
      warnings: [],
    }),
  )
  await page.route('**/api/v1/operations**', (route) => json(route, []))
  await page.route('**/api/v1/ports', (route) =>
    json(route, {
      observedAt,
      warnings: [],
      facts: [
        {
          id: 'alpha-api',
          kind: 'binding',
          projectId: 'alpha',
          projectName: 'Alpha API',
          serviceId: 'api',
          host: '127.0.0.1',
          port: 18080,
          target: 8080,
          protocol: 'tcp',
          source: 'docker',
          evidence: 'Docker published port',
          observedAt,
        },
        {
          id: 'alpha-db',
          kind: 'binding',
          projectId: 'alpha',
          projectName: 'Alpha API',
          serviceId: 'database',
          host: '127.0.0.1',
          port: 15432,
          target: 5432,
          protocol: 'tcp',
          source: 'docker',
          evidence: 'Docker published port',
          observedAt,
        },
        {
          id: 'storefront-web',
          kind: 'binding',
          projectId: 'storefront',
          projectName: 'Storefront',
          serviceId: 'web',
          host: '127.0.0.1',
          port: 15173,
          target: 15173,
          protocol: 'tcp',
          source: 'process',
          evidence: 'Managed process listener',
          observedAt,
        },
      ],
      conflicts: [],
    }),
  )
  await page.route('**/api/v1/projects**', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback()
    const url = new URL(route.request().url())
    const path = url.pathname.replace('/api/v1', '')
    if (path === '/projects') return json(route, empty ? [] : projects)
    const match = path.match(/^\/projects\/([^/]+)(?:\/(.*))?$/)
    if (!match) return route.fallback()
    const projectId = match[1]!
    const suffix = match[2] ?? ''
    if (!suffix)
      return json(
        route,
        projects.find((item) => item.id === projectId),
      )
    if (suffix === 'runtime') return json(route, runtime(projectId))
    if (suffix === 'health')
      return json(route, {
        projectId,
        status: projectId === 'billing' ? 'unhealthy' : 'healthy',
        observerState: projectId === 'billing' ? 'disconnected' : 'connected',
        results: [
          {
            projectId,
            serviceId: 'api',
            checkId: 'readiness',
            type: 'http',
            status: projectId === 'billing' ? 'unhealthy' : 'healthy',
            severity: projectId === 'billing' ? 'critical' : 'info',
            required: true,
            latencyMs: 18,
            message: projectId === 'billing' ? 'Docker observer disconnected' : 'HTTP 200',
            observedAt,
          },
        ],
        observedAt,
      })
    if (suffix === 'logs')
      return json(
        route,
        Array.from({ length: 16 }, (_, index) => ({
          sequence: index + 1,
          timestamp: new Date(Date.parse(observedAt) + index * 1_000).toISOString(),
          projectId,
          serviceId: index % 3 === 0 ? 'database' : 'api',
          runId: 'run-alpha',
          source: 'docker',
          stream: index === 12 ? 'stderr' : 'stdout',
          level: index === 12 ? 'warn' : 'info',
          message:
            index === 12
              ? 'connection pool recovered after 120ms'
              : `request completed status=200 duration=${18 + index}ms`,
          redacted: false,
          attributes: {},
        })),
      )
    if (suffix === 'metrics') return json(route, metrics(projectId))
    if (suffix === 'git') return json(route, git(projectId))
    if (suffix === 'actions') return json(route, actions(projectId))
    if (suffix === 'manifest/explain')
      return json(route, {
        manifest: {
          schemaVersion: 'switchyard.dev/v1',
          kind: 'Project',
          metadata: { id: projectId },
          runtime: { driver: runtime(projectId).driver },
        },
        provenance: { 'runtime.driver': 'accepted manifest' },
        sources: [{ name: 'accepted', path: '.switchyard/project.yml' }],
      })
    return route.fallback()
  })
}
