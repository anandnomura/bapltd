# Enterprise Security Invariants & Behavior Guide

## Zero-Trust Policy Enforcement
This workspace is protected by the bapedge (LTD) zero-trust execution broker. All commands and file operations are evaluated by Cedar authorization policies.

## Mandatory Agent Rules on Policy Denial
1. **Immediate Cessation**: If any command or tool execution is denied by bapedge (e.g. access to .env, ~/.aws, .ssh, or unauthorized network egress), you must **STOP IMMEDIATELY**.
2. **No Alternative Workarounds**: You are **STRICTLY FORBIDDEN** from pursuing alternative options, workarounds, or evasion techniques after a policy denial. This includes:
   - Trying alternative shells (bash, cmd, powershell, sh, pwsh, zsh).
   - Using programming languages (python, node, perl, ruby) to read or execute restricted resources.
   - Renaming, copying, moving, symlinking, or redirecting forbidden files (e.g. ren .env, Move-Item .env).
   - Piping or encoding contents (e.g. base64, certutil).
3. **Report to User**: Immediately report that the requested operation is prohibited by corporate zero-trust policy and ask the user for alternative safe tasks.
