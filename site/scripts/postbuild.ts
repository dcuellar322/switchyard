import { appendFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { isPreview } from '../src/config/site'
import { siteRoot } from './paths'

if (isPreview) {
  await appendFile(
    resolve(siteRoot, 'dist/_headers'),
    '\n/*\n  X-Robots-Tag: noindex, nofollow\n  Cache-Control: no-store\n',
    'utf8',
  )
}
