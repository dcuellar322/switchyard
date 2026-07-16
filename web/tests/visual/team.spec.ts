import { expect, test } from '@playwright/test'

import { installAlphaMocks } from '../helpers/alphaMocks'
import { browserBootstrapPath } from '../helpers/browserSession'

const observedAt = '2026-07-16T15:20:00Z'
const publisherId = `publisher-${'a'.repeat(32)}`

test('signed team configuration keeps trust, policy, and registry provenance visible', async ({ page }) => {
  await installAlphaMocks(page)
  await page.route('**/api/v1/team/publishers', (route) => route.fulfill({ contentType: 'application/json', json: [{ id: publisherId, name: 'Switchyard maintainers', publicKey: 'cHVibGlj', trustedAt: observedAt }] }))
  await page.route('**/api/v1/team/bundles**', (route) => route.fulfill({ contentType: 'application/json', json: [{ schemaVersion: 'switchyard.bundle/v1', kind: 'policy-pack', metadata: { id: 'policy.official', name: 'Organization baseline', version: '2.1.0', publisherId, createdAt: observedAt }, payload: { allowedRemoteCapabilities: ['inventory.read'], allowedRemoteActions: ['start'], allowedPluginPublishers: [publisherId], telemetryAllowed: false }, signature: { keyId: publisherId, algorithm: 'Ed25519', value: 'reviewed-signature' }, installedAt: observedAt }] }))
  await page.route('**/api/v1/team/policy', (route) => route.fulfill({ contentType: 'application/json', json: { sourceBundleIds: ['policy.official'], allowedRemoteCapabilities: ['inventory.read'], allowedRemoteActions: ['start'], allowedPluginPublishers: [publisherId], telemetryAllowed: false, requireSignedConfiguration: true } }))
  await page.route('**/api/v1/plugin-registry', (route) => route.fulfill({ contentType: 'application/json', json: [{ id: 'devcontainer-inspector', name: 'Dev Container inspector', version: '1.2.0', summary: 'Reviewed development-container metadata.', publisher: publisherId, downloadUrl: 'https://plugins.example.test/devcontainer.tar.gz', sha256: 'b'.repeat(64), platforms: ['darwin/arm64', 'linux/amd64'], capabilities: ['project.inspect'] }] }))

  await page.goto(browserBootstrapPath('/team'))
  await expect(page.getByRole('heading', { name: 'Team' })).toBeVisible()
  await expect(page.getByText('Organization baseline')).toBeVisible()
  await expect(page.getByText('Dev Container inspector')).toBeVisible()
  await expect(page).toHaveScreenshot('signed-team-configuration.png', { animations: 'disabled', fullPage: true })
})

test('telemetry consent displays the exact bounded payload and opt-out controls', async ({ page }) => {
  await installAlphaMocks(page)
  await page.route('**/api/v1/telemetry', (route) => route.fulfill({ contentType: 'application/json', json: {
    settings: { enabled: true, endpoint: 'https://metrics.example.test/v1', installationId: 'anonymous-preview', updatedAt: observedAt },
    counters: [{ name: 'runtime.operation', value: 8 }, { name: 'remote.operation', value: 2 }],
    preview: { schemaVersion: 'switchyard.telemetry/v1', installationId: 'anonymous-preview', version: '1.0.0', os: 'darwin', architecture: 'arm64', counters: [{ name: 'runtime.operation', value: 8 }, { name: 'remote.operation', value: 2 }], generatedAt: observedAt },
    lastSentAt: observedAt,
  } }))

  await page.goto(browserBootstrapPath('/settings'))
  await expect(page.getByRole('heading', { name: 'Anonymous usage counters' })).toBeVisible()
  await expect(page.getByText('anonymous-preview')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Disable and clear counters' })).toBeVisible()
  await expect(page).toHaveScreenshot('telemetry-consent.png', { animations: 'disabled', fullPage: true })
})
