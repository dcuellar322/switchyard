import vue from '@vitejs/plugin-vue'
import { writeFile } from 'node:fs/promises'
import { defineConfig } from 'vitest/config'

const daemonAddress = process.env.SWITCHYARD_E2E_DAEMON_ADDRESS ?? '127.0.0.1:19616'

export default defineConfig({
  plugins: [
    vue(),
    {
      name: 'preserve-embed-placeholder',
      async closeBundle() {
        await writeFile(new URL('./dist/.gitkeep', import.meta.url), '')
      },
    },
  ],
  server: {
    host: '127.0.0.1',
    proxy: {
      '/api': `http://${daemonAddress}`,
      '/ws': {
        target: `ws://${daemonAddress}`,
        ws: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./tests/setup.ts'],
    include: ['tests/unit/**/*.test.ts', 'src/**/*.test.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      include: ['src/**/*.{ts,vue}'],
      exclude: ['src/api/generated/**', 'src/main.ts'],
    },
  },
})
