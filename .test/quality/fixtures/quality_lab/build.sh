#!/usr/bin/env bash
set -euo pipefail

TARGET="${1:?target project directory is required}"
FIXTURE_ROOT="$(cd "$(dirname "$0")" && pwd)"

if [[ -e "$TARGET" ]]; then
  echo "quality fixture target already exists: $TARGET" >&2
  exit 2
fi

mkdir -p "$(dirname "$TARGET")"
jzero new \
  --cache \
  --name quality_lab \
  --module example.com/quality-lab \
  --output "$TARGET"

cp -R "$FIXTURE_ROOT/inputs/." "$TARGET/"
cp -R "$FIXTURE_ROOT/overlay/." "$TARGET/"
jzero gen --desc desc/api --working-dir "$TARGET"

go -C "$TARGET" test \
  ./internal/audit \
  ./internal/catalog \
  ./internal/approval \
  ./internal/fulfillment \
  ./internal/job \
  ./internal/order \
  ./internal/payment \
  ./internal/pricing \
  ./internal/refund \
  ./internal/shipment \
  ./internal/subscription \
  ./internal/tenant
