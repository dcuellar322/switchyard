#!/bin/sh
set -eu

mkdir -p .cache
GOCACHE="${GOCACHE:-$(pwd)/.cache/go-build}" \
  go build -trimpath -o .cache/switchyard-e2e ./cmd/switchyard
exec ./.cache/switchyard-e2e daemon \
  --data-dir .switchyard-data/e2e \
  --address 127.0.0.1:19616
