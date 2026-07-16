import { expect, test } from '@playwright/test'

import { browserBootstrapPath } from '../helpers/browserSession'

test('scans a repository into an evidence-backed review', async ({ page }) => {
  await page.goto(browserBootstrapPath())
  await page.getByRole('button', { name: 'Projects' }).click()
  await page.getByLabel('Repository path').fill('./test/fixtures/mixed-project')
  await page.getByRole('button', { name: 'Scan repository' }).click()

  await expect(page.getByRole('heading', { name: 'Switchyard Mixed Fixture' }).first()).toBeVisible()
  await expect(page.getByText('No repository code is executed')).toBeVisible()
  await expect(page.getByText('compose.service').first()).toBeVisible()
  await expect(page.getByText('compose.yaml:3').first()).toBeVisible()
  await expect(page.getByText('18082 → 8000')).toBeVisible()
  await expect(page.getByText('uv run pytest')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Approve and trust project' })).toBeEnabled()
  await expect(page.getByText('switchyard-secret-canary-never-return')).toHaveCount(0)

  await page.getByRole('button', { name: 'Approve and trust project' }).click()
  await expect(page.getByText('Project diagnostics')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Health checks' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Log preview' })).toBeVisible()
  await expect(page.getByText('No health checks are declared for this project.')).toBeVisible()
})
