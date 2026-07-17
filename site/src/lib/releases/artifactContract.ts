import type { PackageType, ReleaseArchitecture, ReleasePlatform } from './types'

export const artifactContractVersion = 'switchyard.release-artifacts/v1' as const

export interface ArtifactIdentity {
  platform: ReleasePlatform
  architecture: ReleaseArchitecture
  packageType: PackageType
}

const archivePattern = /^switchyard_(Darwin|Linux|Windows)_(arm64|x86_64)\.(tar\.gz|zip)$/

function architectureFromName(name: string): ReleaseArchitecture {
  const normalized = name.toLowerCase()
  if (/(^|[_.-])(aarch64|arm64)([_.-]|$)/.test(normalized)) return 'arm64'
  if (/(^|[_.-])(x64|x86_64|amd64)([_.-]|$)/.test(normalized)) return 'amd64'
  if (/(^|[_.-])universal([_.-]|$)/.test(normalized)) return 'universal'
  return 'unknown'
}

export function classifyReleaseAsset(name: string): ArtifactIdentity | undefined {
  const archive = archivePattern.exec(name)
  if (archive) {
    const os = archive[1]
    const arch = archive[2]
    const format = archive[3]
    if (!os || !arch || !format) return undefined
    return {
      platform: os === 'Darwin' ? 'macos' : os === 'Windows' ? 'windows' : 'linux',
      architecture: arch === 'arm64' ? 'arm64' : 'amd64',
      packageType: format as 'tar.gz' | 'zip',
    }
  }

  const normalized = name.toLowerCase()
  const architecture = architectureFromName(name)
  if (normalized.endsWith('.dmg')) return { platform: 'macos', architecture, packageType: 'dmg' }
  if (normalized.endsWith('.app.tar.gz')) return { platform: 'macos', architecture, packageType: 'app' }
  if (normalized.endsWith('.msi')) return { platform: 'windows', architecture, packageType: 'msi' }
  if (normalized.endsWith('-setup.exe')) return { platform: 'windows', architecture, packageType: 'nsis' }
  if (normalized.endsWith('.appimage')) return { platform: 'linux', architecture, packageType: 'appimage' }
  if (normalized.endsWith('.deb')) return { platform: 'linux', architecture, packageType: 'deb' }
  if (normalized.endsWith('.rpm')) return { platform: 'linux', architecture, packageType: 'rpm' }
  return undefined
}

export interface RequiredArtifact {
  platform: ReleasePlatform
  packageTypes: PackageType[]
  architecture: ReleaseArchitecture
  label: string
}

export const stableArtifactRequirements: RequiredArtifact[] = [
  { platform: 'macos', packageTypes: ['dmg'], architecture: 'arm64', label: 'macOS arm64 desktop DMG' },
  { platform: 'macos', packageTypes: ['dmg'], architecture: 'amd64', label: 'macOS amd64 desktop DMG' },
  { platform: 'windows', packageTypes: ['msi', 'nsis'], architecture: 'amd64', label: 'Windows amd64 signed installer' },
  { platform: 'linux', packageTypes: ['appimage'], architecture: 'amd64', label: 'Linux amd64 AppImage' },
  { platform: 'linux', packageTypes: ['deb'], architecture: 'amd64', label: 'Linux amd64 deb' },
  { platform: 'linux', packageTypes: ['rpm'], architecture: 'amd64', label: 'Linux amd64 rpm' },
  { platform: 'macos', packageTypes: ['tar.gz'], architecture: 'arm64', label: 'macOS arm64 CLI archive' },
  { platform: 'macos', packageTypes: ['tar.gz'], architecture: 'amd64', label: 'macOS amd64 CLI archive' },
  { platform: 'linux', packageTypes: ['tar.gz'], architecture: 'arm64', label: 'Linux arm64 CLI archive' },
  { platform: 'linux', packageTypes: ['tar.gz'], architecture: 'amd64', label: 'Linux amd64 CLI archive' },
  { platform: 'windows', packageTypes: ['zip'], architecture: 'arm64', label: 'Windows arm64 CLI archive' },
  { platform: 'windows', packageTypes: ['zip'], architecture: 'amd64', label: 'Windows amd64 CLI archive' },
]

export function missingStableArtifacts(assets: ArtifactIdentity[]): string[] {
  return stableArtifactRequirements
    .filter(
      (requirement) =>
        !assets.some(
          (asset) =>
            asset.platform === requirement.platform &&
            asset.architecture === requirement.architecture &&
            requirement.packageTypes.includes(asset.packageType),
        ),
    )
    .map((requirement) => requirement.label)
}
