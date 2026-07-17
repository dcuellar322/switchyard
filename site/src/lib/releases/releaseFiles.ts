import { classifyReleaseAsset, missingStableArtifacts } from './artifactContract'
import type { ReleaseChannelName } from './types'

export interface ReleaseFileValidationInput {
  channel: ReleaseChannelName
  fileNames: string[]
  checksummedNames: ReadonlySet<string>
}

export function validateReleaseFileSet({
  channel,
  fileNames,
  checksummedNames,
}: ReleaseFileValidationInput): void {
  const duplicates = fileNames.filter((name, index) => fileNames.indexOf(name) !== index)
  if (duplicates.length > 0) {
    throw new Error(`Release files contain duplicate asset names:\n- ${[...new Set(duplicates)].join('\n- ')}`)
  }

  const artifacts = fileNames.flatMap((name) => {
    const identity = classifyReleaseAsset(name)
    return identity ? [{ name, ...identity }] : []
  })
  if (artifacts.length === 0) throw new Error('Release files contain no recognized Switchyard artifacts')

  if (channel === 'stable') {
    const missing = missingStableArtifacts(artifacts)
    if (missing.length > 0) {
      throw new Error(`Stable release files are missing required artifacts:\n- ${missing.join('\n- ')}`)
    }
  }

  const missingChecksums = artifacts
    .filter((artifact) => !checksummedNames.has(artifact.name))
    .map((artifact) => artifact.name)
  if (missingChecksums.length > 0) {
    throw new Error(`Release files are missing checksums for:\n- ${missingChecksums.join('\n- ')}`)
  }
}
