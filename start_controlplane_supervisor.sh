#!/bin/bash
cd "$(dirname "$0")"
chmod +x ./scripts/supervise_controlplane.sh 2>/dev/null
exec ./scripts/supervise_controlplane.sh "$@"

