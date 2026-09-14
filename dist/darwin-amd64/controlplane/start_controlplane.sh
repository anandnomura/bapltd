#!/usr/bin/env bash
set -e
cd "$(dirname "$0")"
chmod +x bapcontrolplane 2>/dev/null || true
echo "==============================================================================="
echo "  BAP CONTROL PLANE - CENTRAL SECURITY GOVERNANCE SERVER"
echo "==============================================================================="
echo "[*] Starting BAP Control Plane on port 8080..."
echo "[*] Open browser dashboard: http://localhost:8080/inspector?mode=live"
echo ""
exec ./bapcontrolplane "$@"
