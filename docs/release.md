# Release engineering

Tagged releases are built only from validated `v*` tags by
`.github/workflows/release.yml`.
The workflow cross-compiles CLI archives with GoReleaser, emits SHA-256
checksums and per-archive SBOMs, builds native Tauri bundles on each operating
system, signs updater payloads, applies Apple notarization and Windows
Authenticode signing, generates desktop CycloneDX SBOMs, attests the combined
artifact set, and creates a draft GitHub release for final human review.

## Release channels

| Channel | Tag form | Distribution behavior |
|---|---|---|
| Nightly | `v1.2.0-nightly.20260716` | Unsigned cross-platform CLI snapshots are also built daily from the default branch and retained as short-lived workflow artifacts. A tagged nightly uses the protected release workflow and is marked prerelease. |
| Alpha | `v1.2.0-alpha.1` | Signed draft prerelease for early contract and migration testing. |
| Beta | `v1.2.0-beta.1` | Signed draft prerelease after the supported-platform workflow matrix passes. |
| Stable | `v1.2.0` | Signed draft release eligible to become the latest published version after human review. |

`scripts/release-channel.sh` is the shared fail-closed classifier. The release
workflow checks out the exact tag for both tag pushes and manual dispatches;
manual dispatch cannot build an arbitrary branch under a release name. Tags
with unknown prerelease labels are rejected. Nightly workflow artifacts are
deliberately unsigned and are never updater-authoritative.

The protected `desktop-release` environment supplies Apple credentials,
`WINDOWS_CERTIFICATE`, `WINDOWS_CERTIFICATE_PASSWORD`, Linux GPG key material,
updater signing keys, and updater endpoint/public-key configuration. Reviewers
confirm that no secret appears in logs or artifacts. The AppImage receives an
embedded GPG signature; signed updater payloads and GitHub attestations cover
every Linux bundle. Repository-specific deb/rpm metadata signing may be added
by a downstream package repository without changing application trust.

## Release candidate checklist

1. Run `make quality` from a clean checkout and confirm generated files stay clean.
2. Confirm native `CI` jobs pass on macOS, Linux, and Windows.
3. Exercise the primary workflow matrix in [platform support](platform-support.md).
4. Test fresh install, alpha/beta upgrade, current-version reinstall, old-binary
   newer-schema refusal, restore-based downgrade, uninstall retaining data, and
   reinstall attaching to retained data.
5. Review dependency, CodeQL, secret-scan, race, vulnerability, and license results.
6. Tag the exact reviewed commit, let the protected workflow build it, and
   inspect the draft release before publishing.

Verify downloads with `checksums.txt`, the published Sigstore bundle, platform
signature tools (`codesign`/`spctl` or `Get-AuthenticodeSignature`), and GitHub's
artifact attestation verification. Compare the included SBOM to the artifact
name and release tag. Reject any updater artifact without its Tauri signature.

The release workflow never publishes partial success: all CLI and three
desktop jobs must complete before the draft release job runs.
