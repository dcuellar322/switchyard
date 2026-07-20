import { expect, test } from '@playwright/test'

import { installAlphaMocks } from '../helpers/alphaMocks'
import { browserBootstrapPath } from '../helpers/browserSession'

const observedAt = '2026-07-16T15:20:00Z'

const workspace = {
  id: 'workspace-product',
  name: 'Product development',
  description:
    'API, storefront, and an isolated feature checkout coordinated for daily development.',
  policy: 'continue',
  profile: 'full',
  members: [
    {
      projectId: 'alpha',
      role: 'dependency',
      order: 0,
      healthGate: true,
      healthTimeoutSeconds: 120,
      status: 'running',
      message: 'project is ready',
    },
    {
      projectId: 'env-feature',
      role: 'application',
      order: 1,
      healthGate: true,
      healthTimeoutSeconds: 120,
      status: 'checking_health',
      message: 'waiting for health gate',
    },
    {
      projectId: 'storefront',
      role: 'application',
      order: 2,
      healthGate: true,
      healthTimeoutSeconds: 120,
      status: 'queued',
    },
  ],
  dependencies: [
    { projectId: 'env-feature', dependsOnProjectId: 'alpha' },
    { projectId: 'storefront', dependsOnProjectId: 'alpha' },
  ],
  recipes: [
    {
      id: 'open-web',
      name: 'Open feature app',
      kind: 'open_url',
      target: 'http://alpha-feature.localhost',
      arguments: [],
      order: 0,
    },
    {
      id: 'open-editor',
      name: 'Open feature checkout',
      kind: 'open_editor',
      projectId: 'env-feature',
      target: 'vscode',
      arguments: [],
      order: 1,
    },
  ],
  profiles: [
    {
      id: 'full',
      name: 'Full workspace',
      projectIds: ['alpha', 'env-feature', 'storefront'],
      maxParallel: 4,
      lowMemory: false,
    },
    {
      id: 'low-memory',
      name: 'Low memory',
      description: 'Dependency-safe sequential startup',
      projectIds: ['alpha', 'env-feature', 'storefront'],
      maxParallel: 1,
      lowMemory: true,
    },
  ],
  lastRun: {
    id: 'workspace-run-1',
    workspaceId: 'workspace-product',
    kind: 'start',
    state: 'running',
    policy: 'continue',
    profileId: 'full',
    removeData: false,
    projects: [
      {
        projectId: 'alpha',
        role: 'dependency',
        status: 'running',
        message: 'project is ready',
        order: 0,
        startedAt: observedAt,
        finishedAt: observedAt,
      },
      {
        projectId: 'env-feature',
        role: 'application',
        status: 'checking_health',
        message: 'waiting for health gate',
        order: 1,
        startedAt: observedAt,
      },
      { projectId: 'storefront', role: 'application', status: 'queued', order: 2 },
    ],
    startedAt: observedAt,
  },
  revision: 3,
  createdAt: observedAt,
  updatedAt: observedAt,
}

test('workspace graph exposes dependency progress and isolated worktrees', async ({ page }) => {
  await installAlphaMocks(page)
  await page.route('**/api/v1/workspaces**', (route) =>
    route.fulfill({ contentType: 'application/json', json: [workspace] }),
  )
  await page.route('**/api/v1/workspaces/workspace-product**', (route) =>
    route.fulfill({ contentType: 'application/json', json: workspace }),
  )
  await page.route('**/api/v1/projects/**', (route) => {
    const url = new URL(route.request().url())
    if (!url.pathname.endsWith('/environments')) return route.fallback()
    const projectId = url.pathname.split('/')[4]
    return route.fulfill({
      contentType: 'application/json',
      json:
        projectId === 'alpha'
          ? [
              {
                id: 'env-feature',
                projectId: 'alpha',
                name: 'feature/workspaces',
                path: '/Users/dev/projects/alpha-feature',
                branch: 'feature/workspaces',
                detached: false,
                bare: false,
                locked: false,
                primary: false,
                availability: 'available',
                state: 'active',
                hostname: 'alpha-feature.localhost',
                target: 'http://127.0.0.1:22080',
                allocation: {
                  composeProjectName: 'sy-alpha-feature-f9348e18',
                  portLeaseNamespace: 'worktree:f9348e18',
                  portOffset: 42,
                  portLeases: [
                    { portId: 'api', protocol: 'tcp', targetPort: 8080, hostPort: 22080 },
                  ],
                },
                registeredAt: observedAt,
                lastObservedAt: observedAt,
                updatedAt: observedAt,
              },
            ]
          : [],
    })
  })

  await page.goto(browserBootstrapPath('/workspaces?workspace=workspace-product'))
  await expect(page.getByRole('heading', { name: 'Product development' })).toBeVisible()
  await expect(page.getByText('feature/workspaces', { exact: true }).first()).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Start order' })).toBeVisible()
  await expect(page).toHaveScreenshot('workspace-progress.png', {
    animations: 'disabled',
    fullPage: true,
  })
})
