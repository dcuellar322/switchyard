import { mkdir, rm, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import releaseManifest from '../src/data/release-manifest.json' with { type: 'json' }
import { generateHomebrewCask, generateWingetManifests } from '../src/lib/distribution/generate'
import type { ReleaseManifest } from '../src/lib/releases/types'
import { siteRoot } from './paths'

const manifest = releaseManifest as ReleaseManifest
const stable = manifest.channels.stable
const expectedTagIndex = process.argv.indexOf('--expect-tag')
const expectedTag = expectedTagIndex >= 0 ? process.argv[expectedTagIndex + 1] : undefined
if (!stable) throw new Error('Package drafts require a published stable release')
if (expectedTag && stable.tag !== expectedTag) throw new Error(`Refusing package generation: expected ${expectedTag}, release manifest contains ${stable.tag}`)

const output = resolve(siteRoot, '.generated/package-drafts')
await rm(output, { force: true, recursive: true })
await mkdir(resolve(output, 'homebrew/Casks'), { recursive: true })
await writeFile(resolve(output, 'homebrew/Casks/switchyard.rb'), generateHomebrewCask(stable))
const winget = generateWingetManifests(stable)
const wingetDirectory = resolve(output, `winget/manifests/d/DavidCuellar/Switchyard/${stable.version}`)
await mkdir(wingetDirectory, { recursive: true })
await Promise.all(Object.entries(winget).map(([name, body]) => writeFile(resolve(wingetDirectory, name), body)))
await writeFile(resolve(output, 'REVIEW.md'), `# Switchyard ${stable.tag} package drafts\n\nGenerated only from the validated stable release manifest. Review URLs, SHA-256 values, fresh install, upgrade, uninstall, and publisher identity before submitting either manifest upstream.\n`)
console.log(`Generated reviewed package drafts for ${stable.tag} in ${output}`)
