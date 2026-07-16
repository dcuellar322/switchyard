#!/bin/sh
set -eu

make generate
git diff --exit-code -- \
  internal/platform/sqlite/generated \
  internal/transport/contract/generated \
  web/src/api/generated
