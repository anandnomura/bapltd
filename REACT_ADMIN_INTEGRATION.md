# Enterprise React Admin UI Integration Guide

## Overview

In enterprise production deployments, the **BAP Control Plane (`bapcontrolplane`)** typically runs as a dedicated Linux service or container on an internal security server (**Server A**, e.g., `https://bap-controlplane.internal:8080`), while the **Management Dashboard / React Web Application** runs on a separate web application server (**Server B**, e.g., `https://bap-dashboard.internal:3000`).

The React UI is a pure administrative web client. **It does not require `bapedge`, local interceptors, or agent shims.** It connects to `bapcontrolplane` over HTTPS using REST and WebSocket APIs.

This guide explains:
1. **Network Topology & Cross-Origin Resource Sharing (CORS)**
2. **Administrative Token Authentication & Initialization Handshake**
3. **Session Management & CSRF Defense**
4. **Step-by-Step React Implementation (Hooks, Services, and State)**
5. **Backend-For-Frontend (BFF) Pattern for High-Security Banking Environments**

---

## 1. Network Topology

```
┌────────────────────────────────────────────────────────┐       ┌────────────────────────────────────────────────────────┐
│             Server B (Web Application Server)          │       │             Server A (Security Governance Core)        │
│          https://bap-dashboard.nomura.com:3000         │       │          https://bap-controlplane.nomura.com:8080      │
│                                                        │       │                                                        │
│  ┌──────────────────────────────────────────────────┐  │       │  ┌──────────────────────────────────────────────────┐  │
│  │       React Admin UI (Browser Client)            │  │ HTTPS │  │             bapcontrolplane (Go Daemon)          │  │
│  │                                                  │  │ REST  │  │                                                  │  │
│  │  • CISO Cockpit & Global Kill-Switch             ├──┼───────┼─►│  • Zero-Trust Policy Engine (Cedar In-Process)   │  │
│  │  • Surgical Agent Revocation                     │  │ CORS  │  │  • SPIFFE Workload Identity Registry            │  │
│  │  • Real-Time Active Radar                        │  │       │  │  • Session Persistence (SQLite / WAL)           │  │
│  │  • Tamper-Evident SHA-256 Merkle Chain           │  │       │  │  • Immutable SHA-256 Merkle Audit Chain         │  │
│  │  • Developer Prompt Telemetry (Leadership Gated) │  │       │  │  • Admin Auth Middleware (BAP_ADMIN_TOKEN)       │  │
│  └──────────────────────────────────────────────────┘  │       │  └──────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────┘       └────────────────────────────────────────────────────────┘
```

---

## 2. Token Initialization & Handshake

Administrative endpoints on `bapcontrolplane` (such as kill-switch, surgical agent revocation, and unmasked prompt visibility) are protected by **Admin Bearer Token Authentication**.

### Starting the Control Plane with Remote Admin Enabled
On Server A, start `bapcontrolplane` with `-allow-remote-admin` and a pre-shared administrative secret:
```bash
# Example systemd service / container startup on Server A
export BAP_ADMIN_TOKEN="nomura-ciso-admin-secret-2026-9f8a2b3c!"
./bapcontrolplane -port 8080 -https -admin-token "$BAP_ADMIN_TOKEN" -allow-remote-admin
```

> [!NOTE]
> If `-admin-token` is omitted, `bapcontrolplane` automatically generates a random 256-bit token (`bap_adm_...`), logs it to stdout, and writes it to `.bap-admin-token`. In production, supply a designated key via corporate vault (HashiCorp Vault, AWS Secrets Manager, CyberArk).

### Authentication Handshake (`POST /api/v1/auth/admin-login`)
When the React UI initializes, it authenticates with Server A:

```http
POST /api/v1/auth/admin-login HTTP/1.1
Host: bap-controlplane.nomura.com:8080
Content-Type: application/json

{
  "token": "nomura-ciso-admin-secret-2026-9f8a2b3c!"
}
```

**Response (HTTP 200 OK)**:
```json
{
  "status": "authorized",
  "role": "ciso_admin",
  "admin_token": "nomura-ciso-admin-secret-2026-9f8a2b3c!",
  "is_loopback": false,
  "permissions": [
    "kill_switch",
    "isolate_agent",
    "view_prompts",
    "reset_sessions",
    "audit_verify"
  ]
}
```
The response sets a `SameSite=Strict` cookie and returns permissions. The React UI stores the token in memory (`useAuthStore`) or `sessionStorage` (which automatically clears when the browser tab is closed).

---

## 3. Communication Security (Beyond HTTPS Alone)

Communicating between two separate servers across a corporate intranet requires defense-in-depth beyond TLS encryption:

1. **Header-Based Cryptographic Verification**:
   Every mutating request from React includes:
   `X-BAP-Admin-Token: <token>` or `Authorization: Bearer <token>`
   If a network actor guesses the URL (`/api/v1/control/kill-switch`), the request is rejected with `401 Unauthorized` and an alert is recorded into the SHA-256 Merkle chain.
2. **CORS Hardening**:
   `bapcontrolplane` inspects the `Origin` header and only responds to authorized browser origins. Preflight `OPTIONS` requests are validated before any mutating `POST` is processed.
3. **Role-Based Developer Prompt Protection**:
   Developer prompts (e.g. what queries engineers sent to Claude Code) are masked as `[Protected: Leadership Authentication Required]` unless the caller presents the verified admin token.
4. **Audit Non-Repudiation**:
   Every action taken in the React UI (e.g., clicking "Isolate Agent" or "Engage Kill-Switch") logs the caller's IP, timestamp, action, and resulting hash to `ltd-audit.jsonl`.

---

## 4. Complete React Integration Implementation

Here is a turnkey, enterprise-ready TypeScript / React integration module.

### `src/services/bapApi.ts`
```typescript
/**
 * BAP Control Plane Client SDK for React Admin Portal
 */

const BAP_BASE_URL = process.env.REACT_APP_BAP_SERVER_URL || "https://bap-controlplane.nomura.com:8080";

export interface BapTelemetryData {
  service: string;
  trust_domain: string;
  chain_status: "valid" | "corrupted";
  kill_switch: boolean;
  admin_authorized: boolean;
  agents: AgentRecord[];
  sessions: SessionRecord[];
  central_events: AuditEvent[];
  last_demo_action?: any;
}

export interface SessionRecord {
  session_id: string;
  app_id: string;
  instance_id: string;
  user_id: string;
  user_email: string;
  status: "active" | "closed" | "revoked";
  user_prompt?: string;
  total_events: number;
  allowed_count: number;
  denied_count: number;
  started_at: string;
}

export interface AuditEvent {
  event_id: string;
  source: string;
  full_command: string;
  decision: "allow" | "deny";
  reason: string;
  user_prompt?: string;
  duration_ms: number;
  timestamp: string;
  event_hash: string;
}

class BapApiClient {
  private adminToken: string | null = null;

  constructor() {
    // Restore session on refresh if stored in sessionStorage
    this.adminToken = sessionStorage.getItem("bap_admin_token");
  }

  public setAdminToken(token: string) {
    this.adminToken = token;
    sessionStorage.setItem("bap_admin_token", token);
  }

  public clearAdminToken() {
    this.adminToken = null;
    sessionStorage.removeItem("bap_admin_token");
  }

  public isAuthenticated(): boolean {
    return Boolean(this.adminToken);
  }

  private getHeaders(): Record<string, string> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      "Accept": "application/json",
    };
    if (this.adminToken) {
      headers["Authorization"] = `Bearer ${this.adminToken}`;
      headers["X-BAP-Admin-Token"] = this.adminToken;
    }
    return headers;
  }

  /**
   * Admin Authentication Handshake
   */
  public async login(token: string): Promise<boolean> {
    const resp = await fetch(`${BAP_BASE_URL}/api/v1/auth/admin-login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token }),
    });

    if (resp.ok) {
      const data = await resp.json();
      this.setAdminToken(data.admin_token || token);
      return true;
    }
    return false;
  }

  /**
   * Fetch Real-Time Fleet Telemetry, Sessions, and Radar
   */
  public async fetchTelemetry(): Promise<BapTelemetryData> {
    const resp = await fetch(`${BAP_BASE_URL}/api/v1/inspector/data`, {
      method: "GET",
      headers: this.getHeaders(),
    });
    if (!resp.ok) throw new Error(`HTTP ${resp.status} fetching telemetry`);
    return await resp.json();
  }

  /**
   * Engage or Disengage Global Fleet Kill-Switch (CISO Override)
   */
  public async setGlobalKillSwitch(enabled: boolean): Promise<any> {
    const resp = await fetch(`${BAP_BASE_URL}/api/v1/control/kill-switch`, {
      method: "POST",
      headers: this.getHeaders(),
      body: JSON.stringify({ enabled }),
    });
    if (!resp.ok) throw new Error(`Failed to toggle kill switch: HTTP ${resp.status}`);
    return await resp.json();
  }

  /**
   * Surgically Isolate / Revoke or Restore a Specific Agent Session
   */
  public async setAgentIsolation(targetSessionId: string, action: "revoke" | "restore"): Promise<any> {
    const resp = await fetch(`${BAP_BASE_URL}/api/v1/control/agent/kill`, {
      method: "POST",
      headers: this.getHeaders(),
      body: JSON.stringify({ target: targetSessionId, action }),
    });
    if (!resp.ok) throw new Error(`Failed to isolate agent: HTTP ${resp.status}`);
    return await resp.json();
  }

  /**
   * Verify Central SHA-256 Merkle Audit Chain Integrity
   */
  public async verifyMerkleChain(): Promise<{ valid: boolean; error?: string }> {
    const resp = await fetch(`${BAP_BASE_URL}/api/v1/control/chain/verify`, {
      method: "GET",
      headers: this.getHeaders(),
    });
    return await resp.json();
  }
}

export const bapApi = new BapApiClient();
```

---

### `src/hooks/useBapTelemetry.ts`
```typescript
import { useState, useEffect, useCallback } from "react";
import { bapApi, BapTelemetryData } from "../services/bapApi";

export function useBapTelemetry(pollIntervalMs = 2000) {
  const [data, setData] = useState<BapTelemetryData | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const telemetry = await bapApi.fetchTelemetry();
      setData(telemetry);
      setError(null);
    } catch (err: any) {
      setError(err.message || "Failed to reach Control Plane");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const interval = setInterval(refresh, pollIntervalMs);
    return () => clearInterval(interval);
  }, [refresh, pollIntervalMs]);

  return { data, loading, error, refresh };
}
```

---

### `src/components/CisoDashboard.tsx` (Sample React View)
```tsx
import React, { useState } from "react";
import { useBapTelemetry } from "../hooks/useBapTelemetry";
import { bapApi } from "../services/bapApi";

export const CisoDashboard: React.FC = () => {
  const { data, loading, error, refresh } = useBapTelemetry(2000);
  const [adminKeyInput, setAdminKeyInput] = useState("");
  const [isUnlocked, setIsUnlocked] = useState(bapApi.isAuthenticated());

  const handleLogin = async () => {
    const ok = await bapApi.login(adminKeyInput);
    if (ok) {
      setIsUnlocked(true);
      refresh();
    } else {
      alert("Invalid Admin Token!");
    }
  };

  const handleToggleKillSwitch = async () => {
    if (!data) return;
    const newState = !data.kill_switch;
    await bapApi.setGlobalKillSwitch(newState);
    refresh();
  };

  if (loading && !data) return <div>Connecting to BAP Control Plane...</div>;
  if (error && !data) return <div className="text-red-500">Error: {error}</div>;

  return (
    <div className="p-6 bg-slate-950 text-white min-h-screen">
      {/* Top Ribbon */}
      <header className="flex justify-between items-center pb-6 border-b border-slate-800">
        <div>
          <h1 className="text-xl font-bold">BAP Enterprise AI Governance</h1>
          <p className="text-xs text-slate-400">Trust Domain: {data?.trust_domain}</p>
        </div>

        {/* Lock / Unlock Admin Auth */}
        <div>
          {isUnlocked ? (
            <span className="px-3 py-1 bg-emerald-500/20 text-emerald-400 rounded-full text-xs font-mono">
              ● CISO Admin Authenticated
            </span>
          ) : (
            <div className="flex gap-2">
              <input
                type="password"
                placeholder="Enter BAP_ADMIN_TOKEN..."
                value={adminKeyInput}
                onChange={(e) => setAdminKeyInput(e.target.value)}
                className="px-2 py-1 text-xs bg-slate-900 border border-slate-700 rounded text-white"
              />
              <button onClick={handleLogin} className="px-3 py-1 bg-cyan-600 hover:bg-cyan-500 rounded text-xs font-bold">
                Unlock
              </button>
            </div>
          )}
        </div>
      </header>

      {/* Global Kill-Switch Banner */}
      <div className="my-6 p-4 rounded-xl border border-slate-800 flex justify-between items-center">
        <div>
          <div className="font-bold">Global Fleet Kill-Switch</div>
          <div className="text-xs text-slate-400">Instantly freeze all AI agents across the company</div>
        </div>
        <button
          onClick={handleToggleKillSwitch}
          disabled={!isUnlocked}
          className={`px-4 py-2 font-bold text-xs rounded-xl ${
            data?.kill_switch ? "bg-rose-600 hover:bg-rose-500" : "bg-slate-800 hover:bg-slate-700"
          }`}
        >
          {data?.kill_switch ? "🚨 KILL-SWITCH ENGAGED (CLICK TO RESTORE)" : "ARM KILL-SWITCH"}
        </button>
      </div>

      {/* Sessions Table with Leadership Developer Prompts */}
      <section className="mt-6">
        <h2 className="text-sm font-bold uppercase text-slate-400 mb-3">Live Developer Workload Telemetry</h2>
        <div className="overflow-x-auto border border-slate-800 rounded-xl">
          <table className="w-full text-left text-xs text-slate-300">
            <thead className="bg-slate-900 text-slate-400 font-mono">
              <tr>
                <th className="p-3">Session ID</th>
                <th className="p-3">Engineer</th>
                <th className="p-3">Agent</th>
                <th className="p-3">Developer Prompt Intent (Admin Only)</th>
                <th className="p-3">Allowed / Denied</th>
                <th className="p-3">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {data?.sessions.map((sess) => (
                <tr key={sess.session_id}>
                  <td className="p-3 font-mono text-cyan-400">{sess.session_id}</td>
                  <td className="p-3 font-mono">{sess.user_email || sess.user_id || "N/A"}</td>
                  <td className="p-3">{sess.app_id}</td>
                  <td className="p-3 font-mono text-slate-300">
                    {sess.user_prompt || <span className="text-slate-500 italic">No prompt captured</span>}
                  </td>
                  <td className="p-3 font-mono">
                    <span className="text-emerald-400">{sess.allowed_count} ALLOW</span> /{" "}
                    <span className="text-rose-400">{sess.denied_count} DENY</span>
                  </td>
                  <td className="p-3">
                    {sess.status === "active" ? (
                      <button
                        disabled={!isUnlocked}
                        onClick={() => bapApi.setAgentIsolation(sess.session_id, "revoke").then(refresh)}
                        className="px-2 py-1 bg-rose-950 text-rose-400 hover:bg-rose-700 hover:text-white rounded text-[10px] font-bold"
                      >
                        Isolate
                      </button>
                    ) : (
                      <span className="text-slate-500 uppercase">{sess.status}</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
};
```

---

## 5. Backend-for-Frontend (BFF) Pattern (Recommended for Banks)

In strict financial institutions where browser JavaScript cannot store shared API keys:
1. **Server B** runs a small Node.js / Express or Go proxy.
2. The user logs into the React Portal using corporate Single Sign-On (SSO / SAML / OIDC).
3. The Node.js proxy attaches the `BAP_ADMIN_TOKEN` from its secure environment and proxies requests to `bapcontrolplane` on Server A over mTLS.
4. The client browser only receives standard HTTP-only session cookies, keeping the master BAP secret 100% server-side.
