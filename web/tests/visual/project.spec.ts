import { expect, test } from '@playwright/test'

import { installAlphaMocks } from '../helpers/alphaMocks'
import { browserBootstrapPath } from '../helpers/browserSession'

test('project detail aligns with the alpha reference', async ({ page }) => {
  await installAlphaMocks(page)
  await page.goto(browserBootstrapPath('/projects/alpha'))
  await expect(page.getByRole('heading', { name: 'Alpha API' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Live logs' })).toBeVisible()
  await expect(page.getByText('Daemon connected')).toBeVisible()
  await expect(page).toHaveScreenshot('project-alpha.png', {
    animations: 'disabled',
    fullPage: true,
  })
})

test('project detail discloses degraded observer state', async ({ page }) => {
  await installAlphaMocks(page)
  await page.goto(browserBootstrapPath('/projects/billing'))
  await expect(page.getByRole('heading', { name: 'Billing Worker' })).toBeVisible()
  await expect(page.getByText(/Docker is unavailable/)).toBeVisible()
  await expect(page.getByText('Daemon connected')).toBeVisible()
  await expect(page).toHaveScreenshot('project-degraded.png', {
    animations: 'disabled',
    fullPage: true,
  })
})

test('project terminal presents typed launch and persistence controls', async ({ page }) => {
  await installAlphaMocks(page)
  await page.goto(browserBootstrapPath('/projects/alpha'))
  await page.getByRole('tab', { name: 'terminal' }).click()
  await expect(page.getByRole('heading', { name: 'Interactive terminal' })).toBeVisible()
  await expect(page.getByText(/Disconnecting detaches the browser/)).toBeVisible()
  await expect(page.getByRole('button', { name: 'New session' })).toBeEnabled()
  await expect(page).toHaveScreenshot('project-terminal.png', {
    animations: 'disabled',
    fullPage: true,
  })
})
