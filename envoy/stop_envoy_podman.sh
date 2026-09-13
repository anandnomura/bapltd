#!/usr/bin/env bash
# ==============================================================================
# BAP - Stop Envoy Proxy Container
# ==============================================================================

CONTAINER_NAME="bap-envoy-pep"

if command -v podman >/dev/null 2>&1; then
    podman rm -f "${CONTAINER_NAME}" 2>/dev/null && echo "[+] Podman container '${CONTAINER_NAME}' stopped and removed." || echo "[-] Container was not running."
elif command -v docker >/dev/null 2>&1; then
    docker rm -f "${CONTAINER_NAME}" 2>/dev/null && echo "[+] Docker container '${CONTAINER_NAME}' stopped and removed." || echo "[-] Container was not running."
else
    echo "[-] No container runtime found."
fi

