#!/usr/bin/env bash
# Claude Code PreToolUse Hook Wrapper
# Delegates directly to the compiled Go interceptor binary

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# If native Go binary exists and is executable, delegate to it
if [ -x "$SCRIPT_DIR/interceptor" ]; then
  exec "$SCRIPT_DIR/interceptor" "$@"
fi

# Fallback shell implementation if binary not compiled
PAYLOAD=$(cat)
COMMAND=$(echo "$PAYLOAD" | jq -r '.tool_input.command // empty')

if [ -z "$COMMAND" ]; then
  jq -n '{
    "hookSpecificOutput": {
      "hookEventName": "PreToolUse",
      "permissionDecision": "allow",
      "additionalContext": "No command specified in tool input"
    }
  }'
  exit 0
fi

WORKSPACE_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
LTD_BIN="./ltd-agent"
if [ ! -x "$LTD_BIN" ]; then
  if [ -x "$WORKSPACE_DIR/ltd-agent" ]; then
    LTD_BIN="$WORKSPACE_DIR/ltd-agent"
  elif [ -x "$WORKSPACE_DIR/../ltd-agent/ltd-agent" ]; then
    LTD_BIN="$WORKSPACE_DIR/../ltd-agent/ltd-agent"
  elif command -v ltd-agent >/dev/null 2>&1; then
    LTD_BIN="ltd-agent"
  fi
fi

LTD_OUTPUT=$("$LTD_BIN" exec "$COMMAND" 2>/dev/null) || true
ALLOWED=$(echo "$LTD_OUTPUT" | jq -r '.allowed // false' 2>/dev/null || echo "false")

if [ "$ALLOWED" = "true" ]; then
  PERMISSION="allow"
  CONTEXT=$(echo "$LTD_OUTPUT" | jq -r '.output // ""' 2>/dev/null)
else
  PERMISSION="deny"
  CONTEXT=$(echo "$LTD_OUTPUT" | jq -r '.reason // "Command execution denied by ltd-agent Cedar policy"' 2>/dev/null)
fi

jq -n \
  --arg decision "$PERMISSION" \
  --arg context "$CONTEXT" \
  '{
    "hookSpecificOutput": {
      "hookEventName": "PreToolUse",
      "permissionDecision": $decision,
      "additionalContext": $context
    }
  }'

exit 0
