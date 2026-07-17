import { expect, test } from '@playwright/test'

for (const [name, route] of [['homepage', '/'], ['download', '/download/'], ['docs', '/docs/'], ['community', '/community/']] as const) {
  test(`${name} visual baseline`, async ({ page, browserName, isMobile }) => {
    test.skip(browserName !== 'chromium' || Boolean(isMobile), 'single deterministic visual project')
    await page.goto(route)
    await expect(page).toHaveScreenshot(`${name}.png`, { fullPage: true })
  })
}
