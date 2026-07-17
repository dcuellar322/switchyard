import { createHash } from 'node:crypto'
import { readdir, readFile, stat, writeFile } from 'node:fs/promises'
import { basename, resolve } from 'node:path'
import { validateReleaseFileSet } from '../src/lib/releases/releaseFiles'
import type { ReleaseChannelName } from '../src/lib/releases/types'
import { repositoryRoot } from './paths'

async function filesUnder(directory: string): Promise<string[]> {
  const entries = await readdir(directory, { withFileTypes: true })
  return (await Promise.all(entries.map((entry) => {
    const path = resolve(directory, entry.name)
    return entry.isDirectory() ? filesUnder(path) : [path]
  }))).flat()
}

async function sha256(path: string): Promise<string> {
  return createHash('sha256').update(await readFile(path)).digest('hex')
}

const directoryArgument = process.argv[2]
const channelArgument = process.argv[3]
if (!directoryArgument || !channelArgument) {
  throw new Error('Usage: prepare-release-files <directory> <stable|beta|nightly>')
}
if (!['stable', 'beta', 'nightly'].includes(channelArgument)) {
  throw new Error(`Unknown release channel: ${channelArgument}`)
}

const releaseDirectory = resolve(repositoryRoot, directoryArgument)
if (!(await stat(releaseDirectory)).isDirectory()) throw new Error(`${releaseDirectory} is not a directory`)

const checksumPath = resolve(releaseDirectory, 'checksums.txt')
const files = (await filesUnder(releaseDirectory)).filter((path) => path !== checksumPath)
const fileNames = files.map((path) => basename(path))
const checksumEntries = await Promise.all(files.map(async (path) => `${await sha256(path)}  ${basename(path)}`))
const checksummedNames = new Set(fileNames)

validateReleaseFileSet({
  channel: channelArgument as ReleaseChannelName,
  fileNames,
  checksummedNames,
})

await writeFile(checksumPath, `${checksumEntries.sort().join('\n')}\n`, 'utf8')
console.log(`Validated ${fileNames.length} release files for ${channelArgument} and wrote aggregate checksums.`)
