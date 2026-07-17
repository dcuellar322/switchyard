import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

for (const route of ['/', '/download/', '/docs/', '/community/', '/integrations/codex/']) {
  test(`${route} has no serious or critical axe violations`, async ({ page, browserName }) => {
    test.skip(browserName !== 'chromium', 'axe runs once; behavioral journeys cover the cross-browser matrix')
    await page.goto(route)
    const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa']).analyze()
    const severe = results.violations.filter((violation) => violation.impact === 'serious' || violation.impact === 'critical')
    expect(severe, JSON.stringify(severe, null, 2)).toEqual([])
  })
}
