import { describe, expect, it } from 'vitest'
import { classifyReleaseAsset, missingStableArtifacts, stableArtifactRequirements } from '../../src/lib/releases/artifactContract'
import { normalizeManifest, normalizeRelease, releaseChannelForTag, validatePublishedStable } from '../../src/lib/releases/normalize'
import { formatBytes, recommendAsset } from '../../src/lib/releases/recommend'
import { validateReleaseFileSet } from '../../src/lib/releases/releaseFiles'
import type { GitHubRelease, ReleaseManifest } from '../../src/lib/releases/types'
import releaseFixture from '../fixtures/github-releases.json'

function release(tag: string, names: string[]): GitHubRelease {
  return {
    tag_name: tag,
    name: `Switchyard ${tag}`,
    body: `Highlights for ${tag}.`,
    html_url: `https://github.com/dcuellar322/switchyard/releases/tag/${tag}`,
    published_at: '2026-07-17T00:00:00Z',
    created_at: '2026-07-17T00:00:00Z',
    prerelease: tag.includes('-'),
    draft: false,
    assets: names.map((name) => ({ name, browser_download_url: `https://github.com/dcuellar322/switchyard/releases/download/${tag}/${name}`, size: 10_485_760 })),
  }
}

describe('release artifact contract', () => {
  it('classifies exact CLI and desktop artifact patterns', () => {
    expect(classifyReleaseAsset('switchyard_Darwin_arm64.tar.gz')).toEqual({ platform: 'macos', architecture: 'arm64', packageType: 'tar.gz' })
    expect(classifyReleaseAsset('Switchyard_1.0.0_x64-setup.exe')).toEqual({ platform: 'windows', architecture: 'amd64', packageType: 'nsis' })
    expect(classifyReleaseAsset('Switchyard_1.0.0_amd64.AppImage')).toEqual({ platform: 'linux', architecture: 'amd64', packageType: 'appimage' })
    expect(classifyReleaseAsset('checksums.txt')).toBeUndefined()
  })

  it('classifies every supported desktop format and architecture spelling', () => {
    expect(classifyReleaseAsset('switchyard_Linux_arm64.tar.gz')).toMatchObject({ platform: 'linux', architecture: 'arm64', packageType: 'tar.gz' })
    expect(classifyReleaseAsset('switchyard_Windows_x86_64.zip')).toMatchObject({ platform: 'windows', architecture: 'amd64', packageType: 'zip' })
    expect(classifyReleaseAsset('Switchyard-universal.app.tar.gz')).toMatchObject({ architecture: 'universal', packageType: 'app' })
    expect(classifyReleaseAsset('Switchyard_arm64.msi')).toMatchObject({ platform: 'windows', packageType: 'msi' })
    expect(classifyReleaseAsset('Switchyard_x64.deb')?.packageType).toBe('deb')
    expect(classifyReleaseAsset('Switchyard_amd64.rpm')?.packageType).toBe('rpm')
    expect(classifyReleaseAsset('Switchyard.AppImage')).toMatchObject({ architecture: 'unknown', packageType: 'appimage' })
  })

  it('does not accept incomplete stable matrices', () => {
    expect(missingStableArtifacts([{ platform: 'macos', architecture: 'arm64', packageType: 'dmg' }])).toContain('Windows amd64 signed installer')
  })

  it('enforces the same complete, checksummed matrix on release workflow files', () => {
    const fixture = releaseFixture as { releases: GitHubRelease[] }
    const fileNames = fixture.releases.find((candidate) => candidate.tag_name === 'v1.0.0')!.assets.map((asset) => asset.name)
    expect(() => validateReleaseFileSet({ channel: 'stable', fileNames, checksummedNames: new Set(fileNames) })).not.toThrow()
    expect(() => validateReleaseFileSet({ channel: 'stable', fileNames: fileNames.slice(1), checksummedNames: new Set(fileNames) })).toThrow(/missing required artifacts/)
    expect(() => validateReleaseFileSet({ channel: 'stable', fileNames, checksummedNames: new Set(fileNames.slice(1)) })).toThrow(/missing checksums/)
    expect(() => validateReleaseFileSet({ channel: 'stable', fileNames: [...fileNames, fileNames[0]!], checksummedNames: new Set(fileNames) })).toThrow(/duplicate asset names/)
  })

  it('requires prerelease workflows to contain a recognized checksummed artifact', () => {
    expect(() => validateReleaseFileSet({ channel: 'beta', fileNames: ['README.txt'], checksummedNames: new Set(['README.txt']) })).toThrow(/no recognized/)
    expect(() => validateReleaseFileSet({ channel: 'nightly', fileNames: ['switchyard_Linux_x86_64.tar.gz'], checksummedNames: new Set() })).toThrow(/missing checksums/)
  })
})

describe('release normalization', () => {
  it('normalizes the local stable, beta, and nightly release fixture', () => {
    const fixture = releaseFixture as { releases: GitHubRelease[]; checksums: Record<string, string> }
    const manifest = normalizeManifest(fixture.releases, new Map(Object.entries(fixture.checksums)))
    expect(Object.keys(manifest.channels).sort()).toEqual(['beta', 'nightly', 'stable'])
    expect(() => validatePublishedStable(manifest)).not.toThrow()
  })

  it('selects the newest release per known channel and ignores drafts or unknown tags', () => {
    const draft = release('v2.0.0', [])
    draft.draft = true
    const manifest = normalizeManifest([draft, release('v1.1.0-beta.1', []), release('preview', []), release('v1.0.0', [])])
    expect(manifest.channels.stable?.tag).toBe('v1.0.0')
    expect(manifest.channels.beta?.tag).toBe('v1.1.0-beta.1')
    expect(releaseChannelForTag('v1.2.0-nightly.20260717')).toBe('nightly')
    expect(releaseChannelForTag('latest')).toBeUndefined()
  })

  it('fails closed when a published stable release lacks assets', () => {
    const manifest = normalizeManifest([release('v1.0.0', ['Switchyard_1.0.0_arm64.dmg'])])
    expect(() => validatePublishedStable(manifest)).toThrow(/missing required artifacts/)
  })

  it('normalizes notes, support artifacts, checksums, and fallback metadata', () => {
    const candidate = release('v1.0.0', ['Switchyard_1.0.0_arm64.dmg', 'Switchyard_1.0.0_arm64.dmg.sig', 'Switchyard_1.0.0_arm64.dmg.cyclonedx.json'])
    candidate.name = '  '
    candidate.body = '# Headline\n\nDetails'
    candidate.published_at = null
    const checksum = `${'A'.repeat(64)} *Switchyard_1.0.0_arm64.dmg\ninvalid line`
    const normalized = normalizeRelease(candidate, checksum)
    expect(normalized).toMatchObject({ title: 'Switchyard v1.0.0', summary: 'Headline', publishedAt: candidate.created_at })
    expect(normalized?.assets[0]).toMatchObject({ checksum: 'a'.repeat(64) })
    expect(normalized?.assets[0]?.signatureUrl).toMatch(/\.sig$/)
    expect(normalized?.assets[0]?.sbomUrl).toMatch(/cyclonedx/)
    expect(normalizeRelease(release('preview', []))).toBeUndefined()
  })

  it('rejects a complete stable artifact matrix when any checksum is absent', () => {
    const assets = stableArtifactRequirements.map((requirement, index) => ({
      name: requirement.label,
      platform: requirement.platform,
      architecture: requirement.architecture,
      packageType: requirement.packageTypes[0]!,
      downloadUrl: `https://example.test/${index}`,
      sizeBytes: 1,
      checksum: index === 0 ? undefined : 'a'.repeat(64),
    }))
    const stable: ReleaseManifest = { schemaVersion: 'switchyard.release-manifest/v1', generatedAt: '2026-07-17T00:00:00Z', repository: 'dcuellar322/switchyard', channels: { stable: { channel: 'stable', version: '1.0.0', tag: 'v1.0.0', title: 'Switchyard v1.0.0', publishedAt: '2026-07-17T00:00:00Z', releaseNotesUrl: 'https://example.test', prerelease: false, assets } }, releases: [] }
    expect(() => validatePublishedStable(stable)).toThrow(/missing checksums/)
  })
})

describe('recommendation', () => {
  it('prefers the platform primary package and never invents a missing asset', () => {
    const manifest = normalizeManifest([release('v1.0.0-beta.1', ['switchyard_Linux_x86_64.tar.gz', 'Switchyard_1.0.0_amd64.AppImage'])])
    expect(recommendAsset(manifest.channels.beta, 'linux', 'amd64')?.packageType).toBe('appimage')
    expect(recommendAsset(manifest.channels.beta, 'windows', 'amd64')).toBeUndefined()
    expect(formatBytes(10_485_760)).toBe('10.0 MiB')
  })

  it('returns no recommendation without a release', () => {
    expect(recommendAsset(undefined, 'macos', 'arm64')).toBeUndefined()
    expect(formatBytes(1024)).toBe('1 KiB')
  })
})

it('accepts a manifest without a published stable channel', () => {
  const manifest: ReleaseManifest = { schemaVersion: 'switchyard.release-manifest/v1', generatedAt: '1970-01-01T00:00:00Z', repository: 'dcuellar322/switchyard', channels: {}, releases: [] }
  expect(() => validatePublishedStable(manifest)).not.toThrow()
})
