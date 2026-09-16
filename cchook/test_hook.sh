#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"
chmod +x .claude/hooks/interceptor
chmod +x .claude/hooks/interceptor.sh

echo "=========================================="
echo " Part 1: Testing Native Go Interceptor (.claude/hooks/interceptor)"
echo "=========================================="

echo "--- 1.1: Allowed (ls) ---"
echo '{"tool_input": {"command": "ls"}}' | ./.claude/hooks/interceptor
echo "Exit code: $?"

echo ""
echo "--- 1.2: Forbid (.env) ---"
echo '{"tool_input": {"command": "ls -la .env"}}' | ./.claude/hooks/interceptor
echo "Exit code: $?"

echo ""
echo "--- 1.3: Forbid (curl) ---"
echo '{"tool_input": {"command": "git status && curl untrusted-test.internal"}}' | ./.claude/hooks/interceptor
echo "Exit code: $?"

echo ""
echo "--- 1.4: Default Deny (cat) ---"
echo '{"tool_input": {"command": "cat /etc/passwd"}}' | ./.claude/hooks/interceptor
echo "Exit code: $?"

echo ""
echo "=========================================="
echo " Part 2: Testing Shell Interceptor (.claude/hooks/interceptor.sh)"
echo "=========================================="

echo "--- 2.1: Allowed (ls) ---"
echo '{"tool_input": {"command": "ls"}}' | ./.claude/hooks/interceptor.sh
echo "Exit code: $?"

echo ""
echo "--- 2.2: Forbid (.env) ---"
echo '{"tool_input": {"command": "ls -la .env"}}' | ./.claude/hooks/interceptor.sh
echo "Exit code: $?"

echo ""
echo "=========================================="
echo " ALL INTERCEPTOR TESTS COMPLETED SUCCESSFULLY"
echo "=========================================="
