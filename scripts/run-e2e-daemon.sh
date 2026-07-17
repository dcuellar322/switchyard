#!/bin/sh
set -eu

data_dir=.switchyard-data/e2e
address=${SWITCHYARD_E2E_DAEMON_ADDRESS:-127.0.0.1:29616}

# Browser tests exercise first-run and trust transitions, so persisted state from a
# previous invocation would make the suite order-dependent. This path is reserved
# for test data and intentionally recreated for every managed Playwright server.
rm -rf "$data_dir"
mkdir -p .cache
GOCACHE="${GOCACHE:-$(pwd)/.cache/go-build}" \
  go build -trimpath -o .cache/switchyard-e2e ./cmd/switchyard
exec ./.cache/switchyard-e2e daemon \
  --data-dir "$data_dir" \
  --address "$address"
