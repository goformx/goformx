#!/usr/bin/env bash
# PostToolUse hook: Auto-run Laravel Pint after PHP file edits in goformx-laravel
# Always exits 0 — formatting is advisory, never blocks

INPUT=$(cat)

FILE_PATH=$(echo "$INPUT" | python3 -c "
import sys, json
data = json.load(sys.stdin)
ti = data.get('tool_input', {})
print(ti.get('file_path', ''))
" 2>/dev/null)

# Only run for PHP files in goformx-laravel (not vendor)
if [[ "$FILE_PATH" == *.php ]] && [[ "$FILE_PATH" == *goformx-laravel/* ]] && [[ "$FILE_PATH" != */vendor/* ]]; then
  LARAVEL_DIR="/home/jones/dev/goformx/goformx-laravel"
  if [[ -f "$LARAVEL_DIR/vendor/bin/pint" ]]; then
    cd "$LARAVEL_DIR" && vendor/bin/pint --dirty --format agent 2>&1 || true
  fi
fi

exit 0
