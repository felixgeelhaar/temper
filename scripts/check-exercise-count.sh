#!/usr/bin/env bash
# Asserts that the exercise count claimed in user-facing docs matches the
# actual count in the exercises/ tree. Run in CI to catch doc/code drift
# before it reaches users.
set -euo pipefail

cd "$(dirname "$0")/.."

actual=$(find exercises -name '*.yaml' -not -name 'pack.yaml' | wc -l | tr -d ' ')
echo "actual exercise files: $actual"

failures=0
check() {
  local file=$1
  local expected_pattern=$2
  if ! grep -q "$expected_pattern" "$file"; then
    echo "FAIL: $file does not mention '$expected_pattern'"
    failures=$((failures + 1))
  fi
}

check docs/quickstart.md  "$actual exercises"
check docs/roadmap.md      "$actual exercises"
check docs/index.md        "$actual exercises"

if [ "$failures" -ne 0 ]; then
  echo
  echo "Exercise count drift detected. Update docs to reflect $actual exercises."
  exit 1
fi

echo "OK: docs reference the correct exercise count ($actual)."
