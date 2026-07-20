import { defineConfig, devices } from '@playwright/test'

const externalBaseURL = process.env.SWITCHYARD_E2E_BASE_URL
const daemonAddress = process.env.SWITCHYARD_E2E_DAEMON_ADDRESS ?? '127.0.0.1:29616'

export default defineConfig({
  testDir: './tests',
  snapshotPathTemplate: '{testDir}/{testFilePath}-snapshots/{arg}-{platform}{ext}',
  // Every browser flow shares one daemon, database, port registry, and Docker
  // engine. Serial execution prevents settings, discovery, and lifecycle flows
  // from racing each other or competing during cold Vite transforms.
  fullyParallel: false,
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  expect: {
    timeout: 15_000,
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
          url: `http://${daemonAddress}/api/v1/system`,
          reuseExistingServer: false,
          timeout: 120_000,
          gracefulShutdown: { signal: 'SIGTERM', timeout: 5_000 },
        },
        {
          command: 'pnpm dev',
          env: { SWITCHYARD_E2E_DAEMON_ADDRESS: daemonAddress },
          url: 'http://127.0.0.1:4173',
          reuseExistingServer: false,
          timeout: 120_000,
        },
      ],
})
