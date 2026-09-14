================================================================================
  BAP EDGE - CLAUDE CODE GOVERNANCE PACKAGE
================================================================================
This folder contains everything needed to run Claude Code governed by BAP Zero-Trust.

Quick Start:
  1. Edit bap-config.json to point to your Linux control plane:
     "controlplane_url": "http://<LINUX_SERVER_IP>:8080"

  2. Run the launcher:
     Windows: run_claude_bap.bat http://<LINUX_SERVER_IP>:8080
     Linux/Mac: ./run_claude_bap.sh http://<LINUX_SERVER_IP>:8080

Features:
  - Sub-1.5ms local Cedar policy evaluation
  - Live workload chip on Central Radar
  - Background process watcher thread (auto-deregisters on exit or window close)
  - Instant remote kill-switch enforcement (< 20ms)
================================================================================
