import { defineConfig } from '@hey-api/openapi-ts'

export default defineConfig({
  input: '../api/openapi.yaml',
  output: {
    clean: true,
    path: 'src/api/generated',
  },
  plugins: ['@hey-api/client-fetch', '@hey-api/typescript', '@hey-api/sdk'],
})
