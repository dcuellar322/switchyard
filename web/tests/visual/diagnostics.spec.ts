import { expect, test } from '@playwright/test'

import { installAlphaMocks } from '../helpers/alphaMocks'
import { browserBootstrapPath } from '../helpers/browserSession'

const observedAt = '2026-07-16T15:20:00Z'

test('diagnostic review keeps evidence, actions, alerts, and automation limits visible', async ({ page }) => {
  await installAlphaMocks(page)
  await page.route('**/api/v1/ai-providers', (route) => route.fulfill({ contentType: 'application/json', json: [{ id: 'codex', name: 'Codex CLI', kind: 'cli', model: 'gpt-5-codex', available: true, supportedBudgetKinds: ['timeout', 'turns', 'tokens'] }] }))
  await page.route('**/api/v1/projects/alpha/diagnoses', (route) => route.fulfill({
    contentType: 'application/json',
    json: {
      id: 'diagnosis-alpha', version: 'switchyard.dev/diagnosis/v1alpha1', projectId: 'alpha', bundleSha256: 'a'.repeat(64), bundleBytes: 8421,
      generatedAt: observedAt, deterministic: true, warnings: [], cleanupPreview: { estimatedBytes: 184_549_376, candidates: 7, unknownSizes: 1, executable: false },
      evidence: [
        { id: 'runtime.service.api', kind: 'runtime', summary: 'API restarted 4 times in the current run', source: 'runtime observer', data: {}, untrusted: false, redacted: false, truncated: false, observedAt },
        { id: 'logs.api.recent', kind: 'logs', summary: 'Recent redacted service output', source: 'local log store', data: {}, untrusted: true, redacted: true, truncated: false, observedAt },
        { id: 'health.database', kind: 'health', summary: 'Required database readiness check is unhealthy', source: 'health observer', data: {}, untrusted: false, redacted: false, truncated: false, observedAt },
      ],
      hypotheses: [
        { id: 'repeated-crash-api', code: 'REPEATED_CRASH', title: 'API is repeatedly crashing', summary: 'The managed API process restarted four times and remains degraded.', severity: 'error', confidence: 0.99, source: 'deterministic', evidenceIds: ['runtime.service.api', 'logs.api.recent'], notifies: true, suggestedActions: [{ actionId: 'tests', name: 'Run tests', risk: 'mutating', reason: 'Accepted project test action can reproduce the failure without changing source.' }] },
        { id: 'database-unhealthy', code: 'UNHEALTHY_DEPENDENCY', title: 'Required database check is failing', summary: 'The database readiness check is unhealthy, so dependent services may fail.', severity: 'warning', confidence: 0.97, source: 'deterministic', evidenceIds: ['health.database'], notifies: true, suggestedActions: [] },
      ],
    },
  }))
  await page.route('**/api/v1/automation-recipes**', (route) => route.fulfill({ contentType: 'application/json', json: [{ id: 'recipe-crash-tests', projectId: 'alpha', name: 'Verify repeated API crashes', triggerCode: 'REPEATED_CRASH', actionId: 'tests', enabled: true, cooldownSeconds: 3600, maxRunsPerDay: 3, lastRunAt: '2026-07-16T14:20:00Z', runsToday: 1, runsDay: '2026-07-16', createdAt: observedAt, updatedAt: observedAt }] }))
  await page.route('**/api/v1/diagnostic-notifications**', (route) => route.fulfill({ contentType: 'application/json', json: [{ id: 'notification-alpha-crash', projectId: 'alpha', code: 'REPEATED_CRASH', title: 'API is repeatedly crashing', detail: 'The managed API process restarted four times and remains degraded.', occurrences: 2, firstSeenAt: '2026-07-16T15:15:00Z', lastSeenAt: observedAt }] }))

  await page.goto(browserBootstrapPath('/agents'))
  await expect(page.getByRole('heading', { name: 'Evidence first. Automation on a leash.' })).toBeVisible()
  await expect(page.getByText('API is repeatedly crashing').first()).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Explicit triggers and limits' })).toBeVisible()
  await expect(page).toHaveScreenshot('diagnostic-automation-review.png', { animations: 'disabled', fullPage: true })
})
