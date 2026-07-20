import { expect, test } from '@playwright/test'

import { installAlphaMocks } from '../helpers/alphaMocks'
import { browserBootstrapPath } from '../helpers/browserSession'

const observedAt = '2026-07-16T15:20:00Z'

test.use({ viewport: { width: 1440, height: 1100 } })

test('plugin review shows executable trust, grants, health, and supervision', async ({ page }) => {
  await installAlphaMocks(page)
  await page.route('**/api/v1/plugins', (route) =>
    route.fulfill({
      contentType: 'application/json',
      json: [
        {
          id: 'devcontainer-inspector',
          name: 'Dev Container inspector',
          version: '1.2.0',
          protocolVersion: 'switchyard.plugin/v1',
          manifestPath:
            '/Users/dev/Library/Application Support/Switchyard/plugins/devcontainer/plugin.json',
          fingerprint: '3f2ad3f8f42994da722754e0ab4b9af1c7cda4f0894d230fd36dc57f56d00b71',
          capabilities: ['project.inspect', 'project.operate'],
          requestedScopes: ['project.metadata.read', 'project.files.read', 'project.operate'],
          grantedScopes: ['project.metadata.read', 'project.files.read'],
          available: true,
          enabled: true,
          trust: 'trusted',
          health: 'healthy',
          healthMessage: 'Adapter ready; Dev Container CLI 0.81 detected.',
          discoveredAt: observedAt,
          updatedAt: observedAt,
        },
      ],
    }),
  )
  await page.route('**/api/v1/plugins/devcontainer-inspector/logs**', (route) =>
    route.fulfill({
      contentType: 'application/json',
      json: [
        {
          id: 2,
          pluginId: 'devcontainer-inspector',
          level: 'info',
          message: 'health check completed in 42ms',
          createdAt: observedAt,
        },
        {
          id: 1,
          pluginId: 'devcontainer-inspector',
          level: 'info',
          message: 'protocol switchyard.plugin/v1 negotiated',
          createdAt: '2026-07-16T15:19:00Z',
        },
      ],
    }),
  )

  await page.goto(browserBootstrapPath('/plugins'))
  await expect(page.getByRole('heading', { name: 'Plugins' })).toBeVisible()
  await expect(page.getByText('Daemon connected')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Capability and scope review' })).toBeVisible()
  await expect(page.getByText('Adapter ready; Dev Container CLI 0.81 detected.')).toBeVisible()
  await expect(page).toHaveScreenshot('plugin-permission-review.png', { animations: 'disabled' })
})
