import { expect, test } from '@playwright/test'

import { installAlphaMocks } from '../helpers/alphaMocks'
import { browserBootstrapPath } from '../helpers/browserSession'

const observedAt = '2026-07-16T15:20:00Z'

function metric(projectId: string, serviceId: string, cpu: number, memory: number) {
  return {
    timestamp: observedAt,
    projectId,
    serviceId,
    resolutionSeconds: 0,
    sampleCount: 1,
    cpuPercent: cpu,
    cpuMaxPercent: cpu + 3,
    cpuAvailable: true,
    memoryBytes: memory,
    memoryMaxBytes: memory + 16_777_216,
    memoryLimit: 1_073_741_824,
    memoryAvailable: true,
    networkRxBytes: 12_400_000,
    networkTxBytes: 4_500_000,
    networkAvailable: true,
    diskReadBytes: 8_200_000,
    diskWriteBytes: 2_100_000,
    diskAvailable: true,
    processCount: 2,
    restartCount: 0,
    healthLatencyMs: 18,
    healthAvailable: true,
    storageBytes: 1_073_741_824,
    storageClassification: 'estimated',
    partial: false,
  }
}

test('resource intelligence matches the approved hierarchy and attribution states', async ({
  page,
}) => {
  await installAlphaMocks(page)
  const alpha = metric('alpha', '', 24.8, 805_306_368)
  const storefront = metric('storefront', '', 8.2, 314_572_800)
  const projects = [
    {
      projectId: 'alpha',
      name: 'Alpha API',
      driver: 'compose',
      state: 'running',
      active: true,
      metric: alpha,
      services: [
        { serviceId: 'api', metric: metric('alpha', 'api', 18.3, 536_870_912) },
        { serviceId: 'database', metric: metric('alpha', 'database', 6.5, 268_435_456) },
      ],
      budget: { cpuPercent: 80, memoryBytes: 751_619_276, storageBytes: 4_294_967_296 },
      warnings: [
        {
          code: 'RESOURCE_MEMORY_SUSTAINED',
          resource: 'memory',
          limit: 751_619_276,
          observed: 805_306_368,
          unit: 'bytes',
          samples: 3,
          sustainedFrom: '2026-07-16T15:19:40Z',
          message: 'memory exceeded its configured threshold for 3 consecutive samples.',
        },
      ],
    },
    {
      projectId: 'storefront',
      name: 'Storefront',
      driver: 'process',
      state: 'running',
      active: true,
      metric: storefront,
      services: [{ serviceId: 'web', metric: metric('storefront', 'web', 8.2, 314_572_800) }],
      budget: { cpuPercent: 70, memoryBytes: 536_870_912, storageBytes: 0 },
      warnings: [],
    },
  ]
  await page.route('**/api/v1/resources', (route) =>
    route.fulfill({
      contentType: 'application/json',
      json: {
        observedAt,
        projects,
        storage: {
          bytes: 6_442_450_944,
          reclaimableBytes: 1_342_177_280,
          classification: 'shared',
          resourceCount: 4,
        },
        footprint: {
          databaseBytes: 8_388_608,
          databaseWalBytes: 1_048_576,
          databaseShmBytes: 32_768,
          logBytes: 67_108_864,
          logSegments: 12,
          metricRows: 4_812,
          classification: 'exclusive',
        },
        retention: {
          sampleIntervalSeconds: 10,
          rawSeconds: 3_600,
          minuteSeconds: 86_400,
          quarterHourSeconds: 2_592_000,
          maximumHistoryPoints: 1_000,
          logSeconds: 604_800,
          logBytes: 268_435_456,
        },
        warnings: [],
      },
    }),
  )
  await page.route('**/api/v1/resources/storage', (route) =>
    route.fulfill({
      contentType: 'application/json',
      json: {
        connected: true,
        observedAt,
        summary: {
          bytes: 6_442_450_944,
          reclaimableBytes: 1_342_177_280,
          classification: 'shared',
          resourceCount: 4,
        },
        projects: [],
        resources: [
          {
            kind: 'container',
            id: 'f3e18c5b7d21',
            name: 'alpha-api-1',
            projectIds: ['alpha'],
            bytes: 134_217_728,
            reclaimable: false,
            classification: 'exclusive',
            reason:
              'Writable layer is uniquely associated through the canonical Compose project label.',
          },
          {
            kind: 'image',
            id: 'sha256:126d9fe84b0a',
            name: 'alpha-api:dev',
            projectIds: ['alpha'],
            bytes: 1_073_741_824,
            reclaimable: false,
            classification: 'estimated',
            reason: 'Associated with one project, but layer exclusivity is not proven.',
          },
          {
            kind: 'volume',
            id: 'alpha_postgres',
            name: 'alpha_postgres',
            projectIds: ['alpha'],
            bytes: 2_147_483_648,
            reclaimable: true,
            classification: 'exclusive',
            reason: 'Local volume is referenced only by one canonical Compose project.',
          },
          {
            kind: 'build_cache',
            id: 'cache-93a1',
            name: 'regular',
            projectIds: [],
            bytes: 536_870_912,
            reclaimable: true,
            classification: 'shared',
            reason: 'Docker reports this build cache record as shared.',
          },
        ],
        warnings: ['Image layers and build cache may be shared.'],
      },
    }),
  )
  await page.route('**/api/v1/projects/*/metrics/history**', (route) => {
    const url = new URL(route.request().url())
    const projectId = url.pathname.split('/')[4] ?? 'alpha'
    const points = Array.from({ length: 12 }, (_, index) => ({
      ...metric(projectId, '', 10 + index * 1.3, 536_870_912 + index * 12_582_912),
      timestamp: new Date(Date.parse(observedAt) - (11 - index) * 300_000).toISOString(),
      resolutionSeconds: 60,
      sampleCount: 6,
    }))
    return route.fulfill({
      contentType: 'application/json',
      json: {
        projectId,
        serviceId: '',
        resolutionSeconds: 60,
        from: points[0]!.timestamp,
        to: observedAt,
        points,
      },
    })
  })

  await page.goto(browserBootstrapPath('/resources?project=alpha'))
  await expect(page.getByRole('heading', { name: 'Resources' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Time-series evidence' })).toBeVisible()
  await expect(page.getByText('Inspection only · no delete capability')).toBeVisible()
  await expect(page).toHaveScreenshot('resource-intelligence.png', {
    animations: 'disabled',
    fullPage: true,
  })
})
