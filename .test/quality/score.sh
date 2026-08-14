#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PROJECT="${1:?usage: .test/quality/score.sh <project-root> <skill-root> [case]}"
SKILL_ROOT="${2:?usage: .test/quality/score.sh <project-root> <skill-root> [case]}"
CASE="${3:-quality_lab}"
CONFIG="$ROOT/.test/quality/cases/$CASE.json"

python3 "$ROOT/.test/quality/tools/quality.py" \
  --root "$ROOT" \
  --config "$CONFIG" \
  --project "$PROJECT" \
  --skill-root "$SKILL_ROOT" \
  score
