import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

export const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
export const repositoryRoot = resolve(siteRoot, '..')
export const docsRoot = resolve(repositoryRoot, 'docs')
export const publicRoot = resolve(siteRoot, 'public')
