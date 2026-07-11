#!/bin/sh
set -eu
if [ -n "${PAXM_BINARY:-}" ] && [ -x "$PAXM_BINARY" ]; then exec "$PAXM_BINARY" setup --integration claude-plugin "$@"; fi
if command -v paxm >/dev/null 2>&1; then exec paxm setup --integration claude-plugin "$@"; fi
for p in "${HOME:-}/.local/bin/paxm" "${HOME:-}/go/bin/paxm"; do [ -x "$p" ] && exec "$p" setup --integration claude-plugin "$@"; done
echo "paxm is not installed; install it before running plugin setup" >&2
exit 127
