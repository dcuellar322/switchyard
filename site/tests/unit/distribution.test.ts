import { describe, expect, it } from 'vitest'
import { generateHomebrewCask, generateWingetManifests } from '../../src/lib/distribution/generate'
import { packageRecordMatches } from '../../src/lib/distribution/status'
import type { ReleaseChannel } from '../../src/lib/releases/types'

const release: ReleaseChannel = {
  channel: 'stable', version: '1.0.0', tag: 'v1.0.0', title: 'Switchyard v1.0.0', publishedAt: '2026-07-17T00:00:00Z', releaseNotesUrl: 'https://github.com/dcuellar322/switchyard/releases/tag/v1.0.0', prerelease: false,
  assets: [
    { name: 'Switchyard_arm64.dmg', platform: 'macos', architecture: 'arm64', packageType: 'dmg', downloadUrl: 'https://github.com/dcuellar322/switchyard/releases/download/v1.0.0/Switchyard_arm64.dmg', sizeBytes: 1, checksum: 'a'.repeat(64) },
    { name: 'Switchyard_x64.dmg', platform: 'macos', architecture: 'amd64', packageType: 'dmg', downloadUrl: 'https://github.com/dcuellar322/switchyard/releases/download/v1.0.0/Switchyard_x64.dmg', sizeBytes: 1, checksum: 'b'.repeat(64) },
    { name: 'Switchyard_x64.msi', platform: 'windows', architecture: 'amd64', packageType: 'msi', downloadUrl: 'https://github.com/dcuellar322/switchyard/releases/download/v1.0.0/Switchyard_x64.msi', sizeBytes: 1, checksum: 'c'.repeat(64) },
  ],
}

describe('package draft generation', () => {
  it('derives Homebrew URLs and checksums only from the stable manifest', () => {
    const cask = generateHomebrewCask(release)
    expect(cask).toContain(release.assets[0]!.downloadUrl)
    expect(cask).toContain('sha256 "aaaaaaaa')
    expect(cask).not.toContain('/latest/download/')
  })

  it('emits a version, locale, and installer WinGet manifest', () => {
    const manifests = generateWingetManifests(release)
    expect(Object.keys(manifests)).toHaveLength(3)
    expect(manifests['DavidCuellar.Switchyard.installer.yaml']).toContain('InstallerSha256: CCCCCCCC')
  })

  it('supports reviewed NSIS installers and rejects missing Windows checksums', () => {
    const nsis = { ...release, assets: [...release.assets.slice(0, 2), { ...release.assets[2]!, packageType: 'nsis' as const, name: 'Switchyard_x64-setup.exe' }] }
    expect(generateWingetManifests(nsis)['DavidCuellar.Switchyard.installer.yaml']).toContain('InstallerType: nullsoft')
    expect(() => generateWingetManifests({ ...release, assets: release.assets.map((asset) => asset.platform === 'windows' ? { ...asset, checksum: undefined } : asset) })).toThrow(/checksummed amd64 installer/)
  })

  it('fails if package integrity cannot be proven', () => {
    expect(() => generateHomebrewCask({ ...release, assets: [] })).toThrow(/checksummed artifact/)
  })

  it('requires version, URL, and checksum before a package channel is marked available', () => {
    const cask = generateHomebrewCask(release)
    expect(packageRecordMatches(cask, release, 'macos')).toBe(true)
    expect(packageRecordMatches(cask.replace(release.assets[0]!.checksum!, '0'.repeat(64)), release, 'macos')).toBe(false)
    expect(packageRecordMatches(cask.replace('version "1.0.0"', 'version "2.0.0"'), release, 'macos')).toBe(false)
    expect(packageRecordMatches(cask, { ...release, assets: [] }, 'macos')).toBe(false)
    expect(packageRecordMatches(generateWingetManifests(release)['DavidCuellar.Switchyard.installer.yaml']!, release, 'windows')).toBe(true)
  })
})
