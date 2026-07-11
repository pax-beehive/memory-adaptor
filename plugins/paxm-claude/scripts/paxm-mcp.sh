#!/bin/sh
set -u
if [ -n "${PAXM_BINARY:-}" ] && [ -x "$PAXM_BINARY" ]; then exec "$PAXM_BINARY" mcp serve; fi
if command -v paxm >/dev/null 2>&1; then exec paxm mcp serve; fi
for p in "${HOME:-}/.local/bin/paxm" "${HOME:-}/go/bin/paxm"; do [ -x "$p" ] && exec "$p" mcp serve; done
exit 127
