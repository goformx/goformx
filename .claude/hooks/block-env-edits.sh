#!/usr/bin/env bash
# PreToolUse hook: Block Edit/Write on .env files (allows .env.example)
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

# Allow .env.example
if [[ "$BASENAME" == ".env.example" ]]; then
  exit 0
fi

# Block .env and .env.* files
if [[ "$BASENAME" == ".env" ]] || [[ "$BASENAME" == .env.* ]]; then
  echo "BLOCKED: Cannot edit $BASENAME — environment files contain secrets. Edit .env.example instead and copy manually."
  exit 2
fi

exit 0
