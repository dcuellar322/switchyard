import { expect, test } from '@playwright/test'

import { browserBootstrapPath } from '../helpers/browserSession'

test('renders live daemon status', async ({ page }) => {
  await page.goto(browserBootstrapPath())
  await expect(page.getByRole('heading', { name: 'Switchyard is taking shape.' })).toBeVisible()
  await expect(page.getByText('ready', { exact: true })).toBeVisible()
  await expect(page.getByText('Event stream: connected')).toBeVisible()
  await expect(page.getByText('SQLite migration state')).toBeVisible()
})
