import type { ReleaseArchitecture, ReleaseAsset, ReleaseChannel, ReleasePlatform } from './types'

const packagePriority: Record<ReleasePlatform, ReleaseAsset['packageType'][]> = {
  macos: ['dmg', 'app', 'tar.gz'],
  windows: ['msi', 'nsis', 'zip'],
  linux: ['appimage', 'deb', 'rpm', 'tar.gz'],
  wsl: ['tar.gz'],
  headless: ['tar.gz', 'zip'],
}

export function recommendAsset(
  release: ReleaseChannel | undefined,
  platform: ReleasePlatform,
  architecture: ReleaseArchitecture,
): ReleaseAsset | undefined {
  if (!release) return undefined
  const normalizedPlatform = platform === 'wsl' ? 'linux' : platform
  const candidates = release.assets.filter(
    (asset) =>
      asset.platform === normalizedPlatform &&
      (asset.architecture === architecture || asset.architecture === 'universal'),
  )
  return candidates.sort(
    (left, right) =>
      packagePriority[platform].indexOf(left.packageType) - packagePriority[platform].indexOf(right.packageType),
  )[0]
}

export function formatBytes(value: number): string {
  if (value < 1024 * 1024) return `${Math.max(1, Math.round(value / 1024))} KiB`
  return `${(value / (1024 * 1024)).toFixed(1)} MiB`
}
