import { expect, test } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import { unlinkSync } from 'node:fs'

import { browserBootstrapPath } from '../helpers/browserSession'

test('scans a repository into an evidence-backed review', async ({ page }) => {
  test.setTimeout(90_000)
  await page.goto(browserBootstrapPath())
  await page.getByRole('link', { name: 'Discovery' }).click()
  await page.getByLabel('Repository path').fill('./test/fixtures/mixed-project')
  await page.getByRole('button', { name: 'Scan repository' }).click()

  await expect(
    page.getByRole('heading', { name: 'Switchyard Mixed Fixture' }).first(),
  ).toBeVisible()
  await expect(page.getByText('No repository code is executed')).toBeVisible()
  await expect(page.getByText('compose.service').first()).toBeVisible()
  await expect(page.getByText('compose.yaml:3').first()).toBeVisible()
  await expect(page.getByText('18082 → 8000')).toBeVisible()
  await expect(page.getByText('uv run pytest')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Approve and trust project' })).toBeEnabled()
  await expect(page.getByText('switchyard-secret-canary-never-return')).toHaveCount(0)

  await page.getByRole('button', { name: 'Approve and trust project' }).click()
  await expect(page).toHaveURL(/\/projects\//)
  await expect(page.getByRole('heading', { name: 'Switchyard Mixed Fixture' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Services' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Live logs' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Git' })).toBeVisible()
  const terminalShortcut = page.getByRole('button', { name: 'Terminal' })
  await expect(terminalShortcut).toBeEnabled()
  await terminalShortcut.click()
  await expect(page.getByRole('tab', { name: 'terminal' })).toHaveAttribute('aria-selected', 'true')
  await page.getByRole('combobox', { name: 'Shell', exact: true }).selectOption('sh')
  await page.getByRole('button', { name: 'New session' }).click()
  await expect(page.getByText('connected', { exact: true })).toBeVisible({ timeout: 15_000 })
  const terminalInput = page.locator('.xterm-helper-textarea')
  await terminalInput.pressSequentially("printf 'switchyard-terminal-\\342\\234\\223\\n'")
  await terminalInput.press('Enter')
  await expect(page.locator('.xterm-rows')).toContainText('switchyard-terminal-✓', {
    timeout: 15_000,
  })
  await page.getByRole('tab', { name: 'git' }).click()
  await page.getByRole('tab', { name: 'terminal' }).click()
  await page.getByRole('button', { name: /Switchyard Mixed Fixture shell/ }).click()
  await expect(page.locator('.xterm-rows')).toContainText('switchyard-terminal-✓', {
    timeout: 15_000,
  })
  await page.getByRole('button', { name: 'Terminate process' }).click()
  await page.getByRole('link', { name: 'Ports' }).click()
  await expect(page.getByRole('heading', { name: 'Port registry' })).toBeVisible()
  await expect(page.getByText('18082/tcp').first()).toBeVisible({
    timeout: 15_000,
  })
})

test('manages a native process project through durable browser operations', async ({ page }) => {
  test.setTimeout(60_000)
  await page.goto(browserBootstrapPath())
  await page.getByRole('link', { name: 'Discovery' }).click()
  await page.getByLabel('Repository path').fill('./test/fixtures/node-single-process')
  await page.getByRole('button', { name: 'Scan repository' }).click()
  await expect(
    page.getByRole('heading', { name: 'npm single-process fixture' }).first(),
  ).toBeVisible()
  await page.getByRole('button', { name: 'Approve and trust project' }).click()
  await expect(page.getByRole('heading', { name: 'npm single-process fixture' })).toBeVisible()

  await page.getByRole('button', { name: 'Start', exact: true }).click()
  await expect(page.getByText('Running', { exact: true }).first()).toBeVisible({
    timeout: 20_000,
  })
  await page.getByRole('tab', { name: 'logs' }).click()
  await expect(page.getByText(/npm fixture listening/)).toBeVisible({
    timeout: 15_000,
  })

  await page.getByRole('button', { name: 'Stop' }).click()
  await expect(page.getByText('Stopped', { exact: true }).first()).toBeVisible({
    timeout: 20_000,
  })
})

test('coordinates a trusted project through the workspace builder and lifecycle', async ({
  page,
}) => {
  test.setTimeout(90_000)
  await page.goto(browserBootstrapPath())
  await page.getByRole('link', { name: 'Discovery' }).click()
  await page.getByLabel('Repository path').fill('./test/fixtures/node-single-process')
  await page.getByRole('button', { name: 'Scan repository' }).click()
  const approve = page.getByRole('button', { name: 'Approve and trust project' })
  if (await approve.isEnabled()) {
    await approve.click()
    await expect(page).toHaveURL(/\/projects\//)
    await expect(page.getByRole('heading', { name: 'npm single-process fixture' })).toBeVisible()
  }

  await page.getByRole('link', { name: 'Workspaces' }).click()
  await page.getByRole('button', { name: /New workspace$/ }).click()
  await page.getByLabel('Name').fill('Native lifecycle workspace')
  await page
    .locator('.project-option')
    .filter({ hasText: 'npm single-process fixture' })
    .getByRole('checkbox')
    .check()
  await page.getByRole('button', { name: 'Create workspace' }).click()
  await expect(page.getByRole('heading', { name: 'Native lifecycle workspace' })).toBeVisible()

  const start = page.getByRole('button', { name: /Start$/ })
  const stop = page.getByRole('button', { name: /Stop all/ })
  await start.click()
  await expect(stop).toBeEnabled({ timeout: 30_000 })
  await expect(page.getByText('start · succeeded')).toBeVisible({ timeout: 30_000 })
  await stop.click()
  await expect(start).toBeEnabled({ timeout: 30_000 })
  await expect(page.getByText('stop · succeeded')).toBeVisible({ timeout: 30_000 })
})

test('manages a real Compose project through the same browser workflow', async ({ page }) => {
  test.setTimeout(240_000)
  const fixtureRoot = '../test/fixtures/compose-runtime'
  const fixtureBinary = `${fixtureRoot}/.switchyard-fixture-server`
  let architecture = execFileSync('docker', ['version', '--format', '{{.Server.Arch}}'], {
    encoding: 'utf8',
  }).trim()
  if (architecture === 'x86_64') architecture = 'amd64'
  if (architecture === 'aarch64') architecture = 'arm64'
  execFileSync('go', ['build', '-trimpath', '-o', '.switchyard-fixture-server', 'server.go'], {
    cwd: fixtureRoot,
    env: {
      ...process.env,
      CGO_ENABLED: '0',
      GOOS: 'linux',
      GOARCH: architecture,
    },
  })
  const cleanupFixture = () =>
    execFileSync(
      'docker',
      [
        'compose',
        '--project-directory',
        fixtureRoot,
        '--file',
        `${fixtureRoot}/compose.yaml`,
        '--project-name',
        'switchyard-phase5-fixture',
        'down',
        '--volumes',
        '--remove-orphans',
      ],
      { encoding: 'utf8' },
    )
  cleanupFixture()
  let projectId = ''
  try {
    await page.goto(browserBootstrapPath())
    await page.getByRole('link', { name: 'Discovery' }).click()
    await page.getByLabel('Repository path').fill('./test/fixtures/compose-runtime')
    await page.getByRole('button', { name: 'Scan repository' }).click()
    const approveButton = page.getByRole('button', {
      name: 'Approve and trust project',
    })
    if (await approveButton.isEnabled()) {
      await approveButton.click()
    } else {
      await expect(page.getByText('Trusted', { exact: true }).first()).toBeVisible()
      await page.locator('.existing-projects a').filter({ hasText: 'compose-runtime' }).click()
    }
    await expect(page).toHaveURL(/\/projects\//)
    projectId = new URL(page.url()).pathname.split('/').at(-1)!
    const operationResponse = page.waitForResponse(
      (response) =>
        response.request().method() === 'POST' &&
        /\/api\/v1\/projects\/[^/]+\/operations$/.test(new URL(response.url()).pathname),
    )
    await page.getByRole('button', { name: 'Start', exact: true }).click()
    const operation = (await (await operationResponse).json()) as {
      id: string
    }
    await expect
      .poll(
        async () =>
          page.evaluate(async (operationId) => {
            const response = await fetch(`/api/v1/operations/${operationId}`)
            const body = (await response.json()) as { state: string }
            return body.state
          }, operation.id),
        { timeout: 180_000, message: 'Compose start operation should succeed' },
      )
      .toBe('succeeded')
    await expect
      .poll(
        async () =>
          page.evaluate(async (id) => {
            const response = await fetch(`/api/v1/projects/${id}/runtime`)
            const body = (await response.json()) as {
              state: string
              origin: string
            }
            return `${body.state}:${body.origin}`
          }, projectId),
        {
          timeout: 45_000,
          message: 'Compose runtime should be attributed to Switchyard',
        },
      )
      .toBe('running:switchyard')
    await expect(page.getByText('Running', { exact: true }).first()).toBeVisible({
      timeout: 90_000,
    })
    await expect(page.getByText('compose · switchyard')).toBeVisible({
      timeout: 15_000,
    })
    await page.getByRole('button', { name: 'Stop' }).click()
    await expect(page.getByText('Stopped', { exact: true }).first()).toBeVisible({
      timeout: 30_000,
    })
  } finally {
    cleanupFixture()
    unlinkSync(fixtureBinary)
  }
})
