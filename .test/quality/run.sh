#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CASE="${1:-quality_lab}"
CONFIG="$ROOT/.test/quality/cases/$CASE.json"

if [[ ! -f "$CONFIG" ]]; then
  echo "quality case not found: $CONFIG" >&2
  exit 2
fi

python3 -m unittest \
  discover \
  -s "$ROOT/.test/quality/tools/tests" \
  -p 'test_prompt_independence.py' \
  -v

python3 "$ROOT/.test/quality/tools/quality.py" \
  --root "$ROOT" \
  --config "$CONFIG" \
  run
