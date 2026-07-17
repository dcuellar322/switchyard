import type { ReleaseChannel } from '@/lib/releases/types'

export function packageRecordMatches(body: string, release: ReleaseChannel, platform: 'macos' | 'windows'): boolean {
  const assets = release.assets.filter((asset) => asset.platform === platform && (platform !== 'windows' || asset.architecture === 'amd64'))
  if (assets.length === 0) return false
  if (!new RegExp(`(?:version|PackageVersion):?\\s*["']?${release.version.replaceAll('.', '\\.')}["']?`, 'i').test(body)) return false
  return assets.every((asset) => Boolean(asset.checksum) && body.includes(asset.downloadUrl) && body.toLowerCase().includes(asset.checksum!.toLowerCase()))
}
