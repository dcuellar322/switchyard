#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
rustc_bin=${RUSTC:-rustc}
target=$($rustc_bin --print host-tuple)
version=${SWITCHYARD_VERSION:-0.1.0-alpha.0}
commit=$(git -C "$repo_root" rev-parse --short=12 HEAD 2>/dev/null || printf unknown)
built_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
output="$repo_root/desktop/src-tauri/binaries/switchyard-$target"

mkdir -p "$(dirname -- "$output")" "$repo_root/.cache/go-build"
cd "$repo_root"
GOCACHE="$repo_root/.cache/go-build" go build -trimpath \
  -ldflags "-X switchyard.dev/switchyard/internal/foundation/buildinfo.version=$version -X switchyard.dev/switchyard/internal/foundation/buildinfo.commit=$commit -X switchyard.dev/switchyard/internal/foundation/buildinfo.builtAt=$built_at" \
  -o "$output" ./cmd/switchyard
chmod 0755 "$output"
