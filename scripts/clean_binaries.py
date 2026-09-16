import os

files_to_remove = [
    # Legacy aliases in repo root
    "bapmcp.exe",
    "ltd-agent.exe",
    "bapedge.exe.old",
    "bapmcp.exe.old",
    "ltd-agent.exe.old",

    # Stale binaries in bap-controlplane
    "bap-controlplane/server.exe",
    
    # Stale/duplicate binaries in cchook
    "cchook/bapedge.exe",
    "cchook/bapedge.exe.old",
    "cchook/ltd-agent.exe",
    "cchook/ltd-agent.exe.old",
    "cchook/cchook-interceptor.exe",
    "cchook/interceptor.next.exe",
    "cchook/interceptor.test.exe",

    # Stale/duplicate binaries in copilot
    "copilot/bapedge.exe",
    "copilot/bapedge.exe.old",
    "copilot/ltd-agent.exe",
    "copilot/ltd-agent.exe.old",

    # Stale/duplicate binaries in bap-edge
    "bap-edge/bap-edge.exe",
    "bap-edge/bapedge.next.exe",

    # Stale duplicate in dist claude-client
    "dist/windows-amd64/claude-client/cchook-interceptor.exe",
    "dist/windows-amd64/gateway/bapgateway.exe~"
]

dirs_to_remove = [
    "bap-controlplane/.build-staging-dashboard-fix",
    "bap-edge/dist"
]

removed_count = 0
for f in files_to_remove:
    if os.path.exists(f):
        try:
            os.remove(f)
            print(f"Removed stale file: {f}")
            removed_count += 1
        except Exception as e:
            print(f"Failed to remove {f}: {e}")

import shutil
for d in dirs_to_remove:
    if os.path.exists(d):
        try:
            shutil.rmtree(d, ignore_errors=True)
            print(f"Removed stale directory: {d}")
        except Exception as e:
            print(f"Failed to remove {d}: {e}")

print(f"Total stale binary files removed: {removed_count}")

