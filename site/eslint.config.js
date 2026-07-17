import eslint from '@eslint/js'
import { defineConfig } from 'eslint/config'
import prettier from 'eslint-config-prettier'
import tseslint from 'typescript-eslint'

export default defineConfig(
  { ignores: ['.astro/**', 'coverage/**', 'dist/**', 'playwright-report/**', 'test-results/**'] },
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  {
    languageOptions: {
      globals: {
        document: 'readonly',
        fetch: 'readonly',
        HTMLAnchorElement: 'readonly',
        HTMLButtonElement: 'readonly',
        HTMLElement: 'readonly',
        navigator: 'readonly',
        process: 'readonly',
        Response: 'readonly',
        URL: 'readonly',
        window: 'readonly',
      },
    },
  },
  prettier,
)
