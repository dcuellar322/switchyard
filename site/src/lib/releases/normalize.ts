import { classifyReleaseAsset, missingStableArtifacts } from './artifactContract'
import type {
  GitHubRelease,
  GitHubReleaseAsset,
  ReleaseAsset,
  ReleaseChannel,
  ReleaseChannelName,
  ReleaseManifest,
} from './types'

const repository = 'dcuellar322/switchyard' as const
const tagPattern = /^v(\d+\.\d+\.\d+)(?:-(beta\.\d+|nightly\.\d{8}))?$/

export function releaseChannelForTag(tag: string): ReleaseChannelName | undefined {
  const match = tagPattern.exec(tag)
  if (!match) return undefined
  if (!match[2]) return 'stable'
  return match[2].startsWith('beta.') ? 'beta' : 'nightly'
}

function supportAsset(assets: GitHubReleaseAsset[], baseName: string, extensions: string[]): string | undefined {
  const match = assets.find((asset) => extensions.some((extension) => asset.name === `${baseName}${extension}`))
  return match?.browser_download_url
}

function parseChecksums(body: string | undefined): Map<string, string> {
  const checksums = new Map<string, string>()
  for (const line of body?.split('\n') ?? []) {
    const match = /^([a-fA-F0-9]{64})\s+\*?(.+)$/.exec(line.trim())
    if (match?.[1] && match[2]) checksums.set(match[2], match[1].toLowerCase())
  }
  return checksums
}

export function normalizeRelease(
  release: GitHubRelease,
  checksumBody?: string,
): ReleaseChannel | undefined {
  if (release.draft) return undefined
  const channel = releaseChannelForTag(release.tag_name)
  if (!channel) return undefined
  const checksums = parseChecksums(checksumBody)
  const assets: ReleaseAsset[] = release.assets.flatMap((asset) => {
    const identity = classifyReleaseAsset(asset.name)
    if (!identity) return []
    return [
      {
        name: asset.name,
        ...identity,
        downloadUrl: asset.browser_download_url,
        sizeBytes: asset.size,
        checksum: checksums.get(asset.name),
        sbomUrl: supportAsset(release.assets, asset.name, ['.cyclonedx.json']),
        signatureUrl: supportAsset(release.assets, asset.name, ['.sig', '.asc', '.sigstore.json']),
        attestationUrl: `https://github.com/${repository}/attestations`,
      },
    ]
  })
  return {
    channel,
    version: release.tag_name.replace(/^v/, ''),
    tag: release.tag_name,
    title: release.name?.trim() || `Switchyard ${release.tag_name}`,
    summary: release.body?.split(/\n\s*\n/, 1)[0]?.replace(/^#+\s*/, '').trim() || undefined,
    notes: release.body?.trim() || undefined,
    publishedAt: release.published_at ?? release.created_at,
    releaseNotesUrl: release.html_url,
    prerelease: release.prerelease,
    assets,
  }
}

export function normalizeManifest(
  releases: GitHubRelease[],
  checksumBodies: ReadonlyMap<string, string> = new Map(),
): ReleaseManifest {
  const channels: ReleaseManifest['channels'] = {}
  const normalizedReleases: ReleaseChannel[] = []
  for (const release of releases) {
    const channelName = releaseChannelForTag(release.tag_name)
    if (!channelName) continue
    const normalized = normalizeRelease(release, checksumBodies.get(release.tag_name))
    if (!normalized) continue
    normalizedReleases.push(normalized)
    if (!channels[channelName]) channels[channelName] = normalized
  }
  const sourceDates = releases.flatMap((release) => [release.published_at ?? release.created_at]).sort()
  return {
    schemaVersion: 'switchyard.release-manifest/v1',
    generatedAt: sourceDates.at(-1) ?? '1970-01-01T00:00:00Z',
    repository,
    channels,
    releases: normalizedReleases,
  }
}

export function validatePublishedStable(manifest: ReleaseManifest): void {
  const stable = manifest.channels.stable
  if (!stable) return
  const missing = missingStableArtifacts(stable.assets)
  if (missing.length > 0) {
    throw new Error(`Published stable release ${stable.tag} is missing required artifacts:\n- ${missing.join('\n- ')}`)
  }
  const withoutChecksums = stable.assets.filter((asset) => !asset.checksum).map((asset) => asset.name)
  if (withoutChecksums.length > 0) {
    throw new Error(`Published stable release ${stable.tag} is missing checksums for:\n- ${withoutChecksums.join('\n- ')}`)
  }
}
