export type ReleaseChannelName = 'stable' | 'beta' | 'nightly'
export type ReleasePlatform = 'macos' | 'windows' | 'linux' | 'wsl' | 'headless'
export type ReleaseArchitecture = 'arm64' | 'amd64' | 'universal' | 'unknown'
export type PackageType = 'dmg' | 'app' | 'msi' | 'nsis' | 'deb' | 'rpm' | 'appimage' | 'tar.gz' | 'zip'

export interface ReleaseAsset {
  name: string
  platform: ReleasePlatform
  architecture: ReleaseArchitecture
  packageType: PackageType
  downloadUrl: string
  sizeBytes: number
  checksum?: string
  sbomUrl?: string
  signatureUrl?: string
  attestationUrl?: string
}

export interface ReleaseChannel {
  channel: ReleaseChannelName
  version: string
  tag: string
  title: string
  summary?: string
  notes?: string
  publishedAt: string
  releaseNotesUrl: string
  prerelease: boolean
  assets: ReleaseAsset[]
}

export interface ReleaseManifest {
  schemaVersion: 'switchyard.release-manifest/v1'
  generatedAt: string
  repository: 'dcuellar322/switchyard'
  channels: Partial<Record<ReleaseChannelName, ReleaseChannel>>
  releases: ReleaseChannel[]
}

export interface GitHubReleaseAsset {
  name: string
  browser_download_url: string
  size: number
}

export interface GitHubRelease {
  tag_name: string
  name?: string | null
  body?: string | null
  html_url: string
  published_at: string | null
  created_at: string
  prerelease: boolean
  draft: boolean
  assets: GitHubReleaseAsset[]
}
