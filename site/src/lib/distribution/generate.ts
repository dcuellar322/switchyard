import type { ReleaseAsset, ReleaseChannel } from '@/lib/releases/types'

function requiredAsset(release: ReleaseChannel, predicate: (asset: ReleaseAsset) => boolean, label: string): ReleaseAsset {
  const asset = release.assets.find(predicate)
  if (!asset?.checksum) throw new Error(`${release.tag} cannot generate ${label}: a matching checksummed artifact is required`)
  return asset
}

export function generateHomebrewCask(release: ReleaseChannel): string {
  const arm = requiredAsset(release, (asset) => asset.platform === 'macos' && asset.architecture === 'arm64' && asset.packageType === 'dmg', 'Homebrew arm64 cask')
  const intel = requiredAsset(release, (asset) => asset.platform === 'macos' && asset.architecture === 'amd64' && asset.packageType === 'dmg', 'Homebrew amd64 cask')
  return `cask "switchyard" do
  version "${release.version}"

  on_arm do
    sha256 "${arm.checksum}"
    url "${arm.downloadUrl}"
  end

  on_intel do
    sha256 "${intel.checksum}"
    url "${intel.downloadUrl}"
  end

  name "Switchyard"
  desc "Local development command center"
  homepage "https://switchyard.davidcuellar.tech/"

  app "Switchyard.app"

  zap trash: [
    "~/Library/Application Support/Switchyard",
    "~/Library/Caches/tech.davidcuellar.switchyard",
    "~/Library/Preferences/tech.davidcuellar.switchyard.plist",
  ]
end
`
}

function wingetType(asset: ReleaseAsset): 'wix' | 'nullsoft' {
  return asset.packageType === 'msi' ? 'wix' : 'nullsoft'
}

export function generateWingetManifests(release: ReleaseChannel): Record<string, string> {
  const installers = release.assets.filter((asset) => asset.platform === 'windows' && asset.architecture === 'amd64' && (asset.packageType === 'msi' || asset.packageType === 'nsis'))
  if (installers.length === 0 || installers.some((asset) => !asset.checksum)) throw new Error(`${release.tag} cannot generate WinGet manifests: a checksummed amd64 installer is required`)
  const header = `# Created from reviewed Switchyard release ${release.tag}. Do not edit generated values.\n`
  return {
    [`DavidCuellar.Switchyard.yaml`]: `${header}PackageIdentifier: DavidCuellar.Switchyard\nPackageVersion: ${release.version}\nDefaultLocale: en-US\nManifestType: version\nManifestVersion: 1.10.0\n`,
    [`DavidCuellar.Switchyard.locale.en-US.yaml`]: `${header}PackageIdentifier: DavidCuellar.Switchyard\nPackageVersion: ${release.version}\nPackageLocale: en-US\nPublisher: David Cuellar\nPackageName: Switchyard\nLicense: Apache-2.0\nShortDescription: Local development command center\nPackageUrl: https://switchyard.davidcuellar.tech/\nLicenseUrl: https://github.com/dcuellar322/switchyard/blob/${release.tag}/LICENSE\nManifestType: defaultLocale\nManifestVersion: 1.10.0\n`,
    [`DavidCuellar.Switchyard.installer.yaml`]: `${header}PackageIdentifier: DavidCuellar.Switchyard\nPackageVersion: ${release.version}\nInstallerType: ${wingetType(installers[0]!)}\nInstallers:\n${installers.map((asset) => `  - Architecture: x64\n    InstallerType: ${wingetType(asset)}\n    InstallerUrl: ${asset.downloadUrl}\n    InstallerSha256: ${asset.checksum!.toUpperCase()}`).join('\n')}\nManifestType: installer\nManifestVersion: 1.10.0\n`,
  }
}
