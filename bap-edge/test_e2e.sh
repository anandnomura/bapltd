#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

SOCKET_PATH="/tmp/ltd_e2e.sock"
rm -f "$SOCKET_PATH"

BIN="./bapedge"
if [ ! -f "$BIN" ]; then
    BIN="./ltd-agent"
fi

echo "=========================================="
echo " 1. Testing Permitted Command (ls)"
echo "=========================================="
$BIN exec "ls"

echo ""
echo "=========================================="
echo " 2. Testing Permitted Command (git)"
echo "=========================================="
$BIN exec "git --version"

echo ""
echo "=========================================="
echo " 3. Testing Forbid Rule: Secret read (.env)"
echo "=========================================="
if $BIN exec "cat .env"; then
    echo "ERROR: 'cat .env' was allowed!"
    exit 1
else
    echo "SUCCESS: 'cat .env' was forbidden as expected."
fi

echo ""
echo "=========================================="
echo " 4. Testing Forbid Rule: curl substring"
echo "=========================================="
if $BIN exec "git status && curl https://untrusted-test.internal"; then
    echo "ERROR: 'curl' command was allowed!"
    exit 1
else
    echo "SUCCESS: 'curl' was forbidden as expected."
fi

echo ""
echo "=========================================="
echo " 5. Testing Forbid Rule: ~/.aws substring"
echo "=========================================="
if $BIN exec "ls ~/.aws/config"; then
    echo "ERROR: '~/.aws' command was allowed!"
    exit 1
else
    echo "SUCCESS: '~/.aws' was forbidden as expected."
fi

echo ""
echo "=========================================="
echo " 6. Testing Permitted Metadata: ls -la .env"
echo "=========================================="
touch .env
$BIN exec "ls -la .env"
rm -f .env
echo "SUCCESS: Metadata inspection allowed."

echo ""
echo "=========================================="
echo " 7. Testing Attestation Server Rejection"
echo "=========================================="
$BIN serve --socket "$SOCKET_PATH" &
SERVER_PID=$!
sleep 1

if $BIN attest --socket "$SOCKET_PATH"; then
    echo "ERROR: Unauthorized client was accepted!"
    kill $SERVER_PID 2>/dev/null || true
    exit 1
else
    echo "SUCCESS: Unauthorized client connection was dropped."
fi

kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true
rm -f "$SOCKET_PATH"
sleep 1

echo ""
echo "=========================================="
echo " 8. Testing Attestation Server Acceptance"
echo "=========================================="
HASH=$(sha256sum "$BIN" | awk '{print $1}')
echo "Attesting binary with SHA-256: $HASH"

$BIN serve --socket "$SOCKET_PATH" --allowed-hash "$HASH" &
SERVER_PID=$!
sleep 1

$BIN attest --socket "$SOCKET_PATH"

kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true
rm -f "$SOCKET_PATH"

echo ""
echo "=========================================="
echo " 9. Testing Network Sandbox (test_leak.py)"
echo "=========================================="
$BIN exec "pytest -s tests/test_leak.py"

echo ""
echo "=========================================="
echo " ALL E2E VERIFICATION CHECKS PASSED!"
echo "=========================================="
