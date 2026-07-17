import type { ReleaseArchitecture, ReleasePlatform } from '@/lib/releases/types'

export interface PlatformGuide {
  slug: 'macos' | 'windows' | 'linux' | 'wsl'
  releasePlatform: ReleasePlatform
  name: string
  shortName: string
  primaryFormat: string
  requirements: string
  install: string[]
  verify: string
  architectures: ReleaseArchitecture[]
}

export const platformGuides: PlatformGuide[] = [
  {
    slug: 'macos',
    releasePlatform: 'macos',
    name: 'macOS desktop',
    shortName: 'macOS',
    primaryFormat: 'Signed and notarized DMG',
    requirements: 'macOS 13 or newer. Docker is optional and required only for Compose projects.',
    install: ['Open the DMG.', 'Drag Switchyard to Applications.', 'Launch Switchyard and review the first-run local trust prompt.'],
    verify: 'Verify Gatekeeper assessment, code signing, notarization, checksum, and GitHub attestation before first launch.',
    architectures: ['arm64', 'amd64'],
  },
  {
    slug: 'windows',
    releasePlatform: 'windows',
    name: 'Windows desktop',
    shortName: 'Windows',
    primaryFormat: 'Signed MSI or NSIS installer',
    requirements: 'Windows 11 or Windows Server 2022 with Desktop Experience.',
    install: ['Download a signed installer.', 'Confirm the publisher in the Windows security dialog.', 'Install per-user and launch Switchyard.'],
    verify: 'Use Get-AuthenticodeSignature, SHA-256, and GitHub artifact attestation before installation.',
    architectures: ['amd64', 'arm64'],
  },
  {
    slug: 'linux',
    releasePlatform: 'linux',
    name: 'Linux desktop',
    shortName: 'Linux',
    primaryFormat: 'AppImage, deb, or rpm',
    requirements: 'Ubuntu 22.04+/Debian 12+ or a current Fedora-family desktop with WebKitGTK 4.1 and Secret Service.',
    install: ['Choose the package for your distribution.', 'Verify the checksum, signature, and attestation.', 'Install the package or make the AppImage executable.'],
    verify: 'Compare SHA-256, verify the Linux signature when published, and inspect the attached SBOM.',
    architectures: ['amd64', 'arm64'],
  },
  {
    slug: 'wsl',
    releasePlatform: 'wsl',
    name: 'WSL2 headless',
    shortName: 'WSL2',
    primaryFormat: 'Linux CLI tarball inside WSL',
    requirements: 'A supported WSL2 distribution on Windows 11, with Docker integration configured inside the same distribution when needed.',
    install: ['Download the Linux CLI archive inside WSL.', 'Install `switchyard` in a user-owned PATH directory.', 'Run `switchyard doctor`, then `switchyard ui`.'],
    verify: 'Verify the Linux archive checksum and attestation inside the WSL distribution.',
    architectures: ['amd64', 'arm64'],
  },
]

export function platformGuide(slug: string): PlatformGuide | undefined {
  return platformGuides.find((guide) => guide.slug === slug)
}
