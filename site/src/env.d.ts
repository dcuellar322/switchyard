/// <reference types="astro/client" />

interface ImportMetaEnv {
  readonly SITE_URL?: string
  readonly SITE_PREVIEW?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
