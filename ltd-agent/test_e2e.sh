#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

SOCKET_PATH="/tmp/ltd_e2e.sock"
rm -f "$SOCKET_PATH"

echo "=========================================="
echo " 1. Testing Permitted Command (ls)"
echo "=========================================="
./ltd-agent exec "ls"

echo ""
echo "=========================================="
echo " 2. Testing Permitted Command (git)"
echo "=========================================="
./ltd-agent exec "git --version"

echo ""
echo "=========================================="
echo " 3. Testing Forbid Rule: .env substring"
echo "=========================================="
if ./ltd-agent exec "ls -la .env"; then
    echo "ERROR: 'ls -la .env' was allowed!"
    exit 1
else
    echo "SUCCESS: 'ls -la .env' was forbidden as expected."
fi

echo ""
echo "=========================================="
echo " 4. Testing Forbid Rule: curl substring"
echo "=========================================="
if ./ltd-agent exec "git status && curl https://evil.com"; then
    echo "ERROR: 'curl' command was allowed!"
    exit 1
else
    echo "SUCCESS: 'curl' was forbidden as expected."
fi

echo ""
echo "=========================================="
echo " 5. Testing Forbid Rule: ~/.aws substring"
echo "=========================================="
if ./ltd-agent exec "ls ~/.aws/config"; then
    echo "ERROR: '~/.aws' command was allowed!"
    exit 1
else
    echo "SUCCESS: '~/.aws' was forbidden as expected."
fi

echo ""
echo "=========================================="
echo " 6. Testing Default Deny: cat (not in whitelist)"
echo "=========================================="
if ./ltd-agent exec "cat README.md"; then
    echo "ERROR: 'cat' command was allowed!"
    exit 1
else
    echo "SUCCESS: 'cat' was denied by default as expected."
fi

echo ""
echo "=========================================="
echo " 7. Testing Attestation Server Rejection"
echo "=========================================="
./ltd-agent serve --socket "$SOCKET_PATH" &
SERVER_PID=$!
sleep 1

if ./ltd-agent attest --socket "$SOCKET_PATH"; then
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
HASH=$(sha256sum ./ltd-agent | awk '{print $1}')
echo "Attesting binary with SHA-256: $HASH"

./ltd-agent serve --socket "$SOCKET_PATH" --allowed-hash "$HASH" &
SERVER_PID=$!
sleep 1

./ltd-agent attest --socket "$SOCKET_PATH"

kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true
rm -f "$SOCKET_PATH"

echo ""
echo "=========================================="
echo " 9. Testing Network Sandbox (test_leak.py)"
echo "=========================================="
./ltd-agent exec "pytest -s tests/test_leak.py"

echo ""
echo "=========================================="
echo " ALL E2E VERIFICATION CHECKS PASSED!"
echo "=========================================="
