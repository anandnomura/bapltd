#!/usr/bin/env bash
# ==============================================================================
# BAP (Bounded Authority Plane) - Launch Envoy Proxy on Podman / Docker (Linux/WSL/macOS)
# Runs Envoy on port 10000 with ext_authz linked to BAP Control Plane on port 8080.
# ==============================================================================

set -e

CONTAINER_NAME="bap-envoy-pep"
ENVOY_IMAGE="docker.io/envoyproxy/envoy:v1.31-latest"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${SCRIPT_DIR}/envoy.yaml"

echo "==============================================================================="
echo "   BAP OPTION 1: ENVOY PROXY ON PODMAN / DOCKER"
echo "   Zero-Trust Ingress Gateway PEP (ext_authz Filter)"
echo "==============================================================================="

# 1. Determine container engine (podman preferred, fallback to docker)
if command -v podman >/dev/null 2>&1; then
    ENGINE="podman"
elif command -v docker >/dev/null 2>&1; then
    ENGINE="docker"
else
    echo "[ERROR] Neither 'podman' nor 'docker' was found in your PATH."
    echo "        Please install Podman: sudo dnf install podman / sudo apt-get install podman"
    exit 1
fi
echo "[+] Detected container engine: ${ENGINE}"

# 2. Verify BAP Control Plane is running on host port 8080
if ! curl -s -m 2 http://localhost:8080/api/v1/health >/dev/null 2>&1; then
    echo "[WARNING] BAP Control Plane does not appear to be running on http://localhost:8080."
    echo "          Start bapcontrolplane first: ./bapcontrolplane -port 8080"
fi

# 3. Clean up existing container if running
if ${ENGINE} ps -a --format "{{.Names}}" | grep -q "^${CONTAINER_NAME}$"; then
    echo "[*] Removing existing container '${CONTAINER_NAME}'..."
    ${ENGINE} rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
fi

# 4. Determine host-gateway mapping
# In rootless podman or docker on Linux, host.containers.internal maps to host-gateway
ADD_HOST_FLAG="--add-host host.containers.internal:host-gateway"
VOLUME_FLAG="-v ${CONFIG_FILE}:/etc/envoy/envoy.yaml:ro,Z"

echo "[*] Launching Envoy PEP container on port 10000..."
${ENGINE} run -d \
    --name "${CONTAINER_NAME}" \
    -p 10000:10000 \
    ${ADD_HOST_FLAG} \
    ${VOLUME_FLAG} \
    "${ENVOY_IMAGE}" \
    envoy -c /etc/envoy/envoy.yaml --log-level info

echo "[+] Envoy PEP successfully launched!"
echo "    - Envoy Ingress URL     : http://localhost:10000/api/v1/financial-records"
echo "    - Control Plane Target  : http://host.containers.internal:8080/api/v1/auth/envoy"
echo "    - View container logs   : ${ENGINE} logs -f ${CONTAINER_NAME}"
echo "    - Stop container        : ./stop_envoy_podman.sh"
echo "==============================================================================="

