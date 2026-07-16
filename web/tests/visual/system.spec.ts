import { expect, test } from '@playwright/test'

import { browserBootstrapPath } from '../helpers/browserSession'

test('walking skeleton matches the approved system view', async ({ page }) => {
  await page.goto(browserBootstrapPath())
  await expect(page.getByText('Event stream: connected')).toBeVisible()
  await expect(page).toHaveScreenshot('system-status.png', {
    animations: 'disabled',
    fullPage: true,
  })
})
