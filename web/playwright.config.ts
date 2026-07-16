import { defineConfig, devices } from '@playwright/test'

const externalBaseURL = process.env.SWITCHYARD_E2E_BASE_URL

export default defineConfig({
  testDir: './tests',
  snapshotPathTemplate: '{testDir}/{testFilePath}-snapshots/{arg}{ext}',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  expect: {
    toHaveScreenshot: {
      threshold: 0.3,
      maxDiffPixelRatio: 0.02,
    },
  },
  use: {
    baseURL: externalBaseURL ?? 'http://127.0.0.1:4173',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'e2e',
      testMatch: /e2e\/.*\.spec\.ts/,
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'visual',
      testMatch: /visual\/.*\.spec\.ts/,
      use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 1050 } },
    },
  ],
  webServer: externalBaseURL
    ? undefined
    : [
        {
          command: './scripts/run-e2e-daemon.sh',
          cwd: '..',
          url: 'http://127.0.0.1:19616/api/v1/system',
          reuseExistingServer: !process.env.CI,
          timeout: 120_000,
          gracefulShutdown: { signal: 'SIGTERM', timeout: 5_000 },
        },
        {
          command: 'pnpm dev',
          url: 'http://127.0.0.1:4173',
          reuseExistingServer: !process.env.CI,
          timeout: 120_000,
        },
      ],
})
