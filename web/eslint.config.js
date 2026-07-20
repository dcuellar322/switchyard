import eslint from '@eslint/js'
import prettier from 'eslint-config-prettier'
import vue from 'eslint-plugin-vue'
import tseslint from 'typescript-eslint'

export default tseslint.config(
  {
    ignores: [
      'coverage/**',
      'dist/**',
      'playwright-report/**',
      'src/api/generated/**',
      'test-results/**',
    ],
  },
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  ...vue.configs['flat/recommended'],
  {
    languageOptions: {
      globals: {
        document: 'readonly',
        HTMLInputElement: 'readonly',
        HTMLElement: 'readonly',
        KeyboardEvent: 'readonly',
        localStorage: 'readonly',
        process: 'readonly',
        requestAnimationFrame: 'readonly',
        Response: 'readonly',
        WebSocket: 'readonly',
        window: 'readonly',
      },
    },
  },
  {
    files: ['**/*.vue'],
    languageOptions: {
      parserOptions: {
        parser: tseslint.parser,
      },
    },
    rules: {
      'vue/multi-word-component-names': 'off',
    },
  },
  prettier,
)
