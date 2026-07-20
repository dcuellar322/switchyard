import { expect, test } from '@playwright/test'

import { installAlphaMocks } from '../helpers/alphaMocks'
import { browserBootstrapPath } from '../helpers/browserSession'

test('dashboard aligns with the alpha command-center reference', async ({ page }) => {
  await installAlphaMocks(page)
  await page.goto(browserBootstrapPath())
  await expect(page.getByRole('heading', { name: 'Your development yard' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Alpha API' })).toBeVisible()
  await expect(page.getByText('Daemon connected')).toBeVisible()
  await expect(page).toHaveScreenshot('dashboard-alpha.png', {
    animations: 'disabled',
    fullPage: true,
  })
})

test('dashboard remains coherent at a narrow viewport', async ({ page }) => {
  await page.setViewportSize({ width: 720, height: 1050 })
  await installAlphaMocks(page)
  await page.goto(browserBootstrapPath())
  await expect(page.getByRole('heading', { name: 'Your development yard' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Alpha API' })).toBeVisible()
  await expect(page).toHaveScreenshot('dashboard-narrow.png', {
    animations: 'disabled',
    fullPage: true,
  })
})

test('empty dashboard has an actionable first-run state', async ({ page }) => {
  await installAlphaMocks(page, true)
  await page.goto(browserBootstrapPath())
  await expect(page.getByText('No projects registered')).toBeVisible()
  await expect(page.getByText('Daemon connected')).toBeVisible()
  await expect(page).toHaveScreenshot('dashboard-empty.png', {
    animations: 'disabled',
    fullPage: true,
  })
})
