import { expect, test } from '@playwright/test'

test('homepage routes to trusted downloads and the five-minute tutorial', async ({ page }) => {
  await page.goto('/')
  await expect(page).toHaveTitle(/Switchyard — Local Development Command Center/)
  await expect(page.getByRole('heading', { name: 'Your local development command center.' })).toBeVisible()
  await page.getByRole('link', { name: /Download/ }).first().click()
  await expect(page).toHaveURL(/\/download\/$/)
  await expect(page.getByText('No published stable package yet.')).toBeVisible()
  await page.goto('/')
  await page.getByRole('link', { name: 'Five-minute quickstart' }).click()
  await expect(page).toHaveURL(/\/docs\/start\/$/)
})

test('download selection keeps every platform visible and links integrity evidence', async ({ page }) => {
  await page.goto('/download/')
  await page.locator('input[name="platform"][value="linux"]').check()
  await page.locator('input[name="architecture"][value="amd64"]').check()
  await page.getByRole('button', { name: 'Update recommendation' }).click()
  await expect(page).toHaveURL(/platform=linux/)
  await expect(page).toHaveURL(/architecture=amd64/)
  const platformGrid = page.locator('.platform-grid')
  for (const platform of ['macOS', 'Windows', 'Linux', 'WSL2', 'Headless CLI']) {
    await expect(platformGrid.getByRole('link', { name: new RegExp(`^${platform}`) })).toBeVisible()
  }
  await page.getByRole('link', { name: 'Verification guide' }).click()
  await expect(page.getByRole('heading', { name: 'Verify a Switchyard download' })).toBeVisible()
})

test('documentation starts at a tested first-project outcome', async ({ page }) => {
  await page.goto('/docs/')
  await expect(page.getByRole('heading', { level: 1, name: 'Documentation' })).toBeVisible()
  await page.getByRole('link', { name: /Add your first project/ }).first().click()
  await expect(page).toHaveURL(/\/docs\/getting-started\/$/)
  await expect(page.getByText('switchyard project add')).toBeVisible()
})

test('documentation search finds the CLI contract', async ({ page }) => {
  await page.goto('/docs/')
  const openSearch = page.getByRole('button', { name: 'Search' })
  await expect(openSearch).toBeEnabled()
  await openSearch.click()
  const search = page.getByRole('textbox', { name: 'Search' })
  await search.fill('Switchyard CLI contract')
  await page.getByRole('link', { name: /Switchyard CLI contract/ }).first().click()
  await expect(page).toHaveURL(/\/docs\/cli\/$/)
})

test('Codex integration documents setup and authorization failures', async ({ page }) => {
  await page.goto('/integrations/codex/')
  await expect(page.getByRole('heading', { level: 1, name: /Codex/ })).toBeVisible()
  await expect(page.getByText('switchyard agent install codex --profile observe')).toBeVisible()
  await expect(page.getByText(/authorization/i).first()).toBeVisible()
  await expect(page.getByText(/remov/i).first()).toBeVisible()
})

test('community routes questions, bugs, proposals, and security correctly', async ({ page }) => {
  await page.goto('/community/')
  await expect(page.getByRole('link', { name: /Ask in Discussions/ })).toHaveAttribute('href', /\/discussions$/)
  await expect(page.getByRole('link', { name: /Open a bug report/ })).toHaveAttribute('href', /bug_report\.yml/)
  await expect(page.getByRole('link', { name: /Propose a feature/ })).toHaveAttribute('href', /feature_request\.yml/)
  await expect(page.getByRole('link', { name: /Report privately/ })).toHaveAttribute('href', /security\/advisories\/new/)
  await expect(page.getByText('Never post credentials')).toBeVisible()
})

test('mobile navigation exposes primary destinations', async ({ page, isMobile }) => {
  test.skip(!isMobile, 'mobile-only journey')
  await page.goto('/')
  const menu = page.locator('details.mobile-menu')
  await menu.locator('summary').click()
  await expect(menu.getByRole('link', { name: 'Community' })).toBeVisible()
})

test('theme selection persists across marketing pages', async ({ page, isMobile }) => {
  test.skip(Boolean(isMobile), 'theme control is intentionally hidden on narrow screens')
  await page.goto('/')
  const toggle = page.getByRole('button', { name: 'Change color theme' })
  await toggle.click()
  const theme = await page.locator('html').getAttribute('data-theme')
  expect(theme).toMatch(/dark|light/)
  await page.goto('/features/')
  await expect(page.locator('html')).toHaveAttribute('data-theme', theme!)
})
