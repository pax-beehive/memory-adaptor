#!/bin/sh
set -u
event="${1:-}"
case "$event" in session_start|user_input|tool_use|tool_failure|turn_end) ;; *) exit 0 ;; esac
find_paxm() {
  if [ -n "${PAXM_BINARY:-}" ] && [ -x "$PAXM_BINARY" ]; then printf '%s\n' "$PAXM_BINARY"; return; fi
  if command -v paxm >/dev/null 2>&1; then command -v paxm; return; fi
  for p in "${HOME:-}/.local/bin/paxm" "${HOME:-}/go/bin/paxm"; do [ -x "$p" ] && { printf '%s\n' "$p"; return; }; done
  return 1
}
paxm_bin="$(find_paxm 2>/dev/null || true)"
[ -n "$paxm_bin" ] || exit 0
PAXM_INTEGRATION_OWNER=claude-plugin "$paxm_bin" __hook --target claude --event "$event" 2>/dev/null || exit 0
