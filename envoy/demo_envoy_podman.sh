#!/usr/bin/env bash
# ==============================================================================
# BAP - Option 1 Standalone Demo Runner (Linux / WSL / macOS)
# Launches Envoy on Podman/Docker and executes demo_envoy.py
# ==============================================================================

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

echo "==============================================================================="
echo "   BAP OPTION 1: STANDALONE ENVOY PROXY ON PODMAN / DOCKER DEMO"
echo "==============================================================================="

# 1. Check if bapcontrolplane is running on port 8080
if ! curl -s -m 2 http://localhost:8080/api/v1/health >/dev/null 2>&1; then
    echo "[*] Starting bapcontrolplane in background..."
    if [ -f "../bapcontrolplane" ]; then
        ../bapcontrolplane -port 8080 &
        sleep 2
    else
        echo "[!] ../bapcontrolplane binary not found. Build it with: cd ../bap-controlplane && go build -o ../bapcontrolplane ./cmd/server"
        exit 1
    fi
fi

# 2. Check if Envoy is listening on port 10000
if ! curl -s -m 1 http://localhost:10000/api/v1/health >/dev/null 2>&1; then
    echo "[*] Envoy is not running on port 10000. Launching via Podman/Docker..."
    ./run_envoy_podman.sh
    sleep 2
fi

# 3. Run demo_envoy.py
python3 demo_envoy.py

