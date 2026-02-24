#!/usr/bin/env bash
# PreToolUse hook: Block Edit/Write on lock files
# Exit 2 = block with message, Exit 0 = allow

set -euo pipefail

INPUT=$(cat)

FILE_PATH=$(echo "$INPUT" | python3 -c "
import sys, json
data = json.load(sys.stdin)
ti = data.get('tool_input', {})
print(ti.get('file_path', ti.get('command', '')))
" 2>/dev/null)

BASENAME=$(basename "$FILE_PATH" 2>/dev/null || echo "")

case "$BASENAME" in
  composer.lock|package-lock.json|go.sum)
    echo "BLOCKED: Cannot edit $BASENAME — lock files are managed by package managers. Run 'composer update', 'npm install', or 'go mod tidy' instead."
    exit 2
    ;;
esac

exit 0
