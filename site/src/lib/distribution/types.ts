export type DistributionName = 'homebrew' | 'winget'

export interface DistributionChannel {
  available: boolean
  verifiedVersion?: string
  installCommand: string
  packageUrl: string
  lastChecked?: string
}

export interface DistributionStatus {
  schemaVersion: 'switchyard.distribution-status/v1'
  homebrew: DistributionChannel
  winget: DistributionChannel
}
