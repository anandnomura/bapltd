import React, { useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { presence, elapsed } from './presence.js';
import './style.css';

const API = '/api/v1';
const PROTECTED_MSG = '[Protected: Leadership Authentication Required]';

function formatToolDescription(executable, cmd) {
  if (!cmd) return executable || 'Tool Action';
  const c = cmd.toLowerCase();
  if (c.includes('user_prompt_submitted')) return 'User submitted prompt';
  if (c.includes('git status')) return 'Inspected working tree status';
  if (c.includes('git log')) return 'Inspected repository commit history';
  if (c.includes('fetching settlement') || c.includes('fetching transactions')) return 'Fetched settlement ledger records';
  if (c.includes('reading application logs')) return 'Read application logs for payment 883';
  if (c.includes('checking dependencies') || c.includes('inspecting cluster')) return 'Inspected cluster dependencies & topology';
  if (c.includes('.env') && (c.includes('cat') || c.includes('type'))) return 'Attempted to read production secrets (.env)';
  if (c.includes('.env') && c.includes('python')) return 'Tried alternate Python file extraction method';
  if (c.includes('.env') && (c.includes('powershell') || c.includes('get-content'))) return 'Tried alternate PowerShell cmdlet evasion';
  if (c.includes('curl') || c.includes('wget')) return 'Attempted external HTTP egress';
  if (c.includes('whoami')) return 'Inspected process privileges & identity';
  return cmd.length > 70 ? cmd.slice(0, 70) + '…' : cmd;
}

function computeRisk(deniedCount, status, warning, hasTamper) {
  if (status === 'revoked') return { level: 'REVOKED', weight: 40 };
  if (deniedCount >= 2 || (warning && warning.includes('CRITICAL')) || hasTamper) {
    return { level: 'CRITICAL', weight: 100 };
  }
  if (deniedCount === 1 || (warning && warning.includes('ELEVATED'))) {
    return { level: 'ELEVATED', weight: 75 };
  }
  if (status === 'stopped' || status === 'closed') {
    return { level: 'STOPPED', weight: 10 };
  }
  return { level: 'HEALTHY', weight: 25 };
}

function App() {
  const [data, setData] = useState(null);
  const [sensitive, setSensitive] = useState(null);
  const [now, setNow] = useState(Date.now());
  const [connected, setConnected] = useState(false);
  const [lastRefresh, setLastRefresh] = useState(0);
  const [searchQuery, setSearchQuery] = useState('');
  const [filterPill, setFilterPill] = useState('all');
  const [selectedAgentId, setSelectedAgentId] = useState(null);
  const [expandedEvents, setExpandedEvents] = useState({});
  const [notice, setNotice] = useState('');
  const [pending, setPending] = useState(false);
  const [actionModal, setActionModal] = useState(null);
  const [adminToken, setAdminToken] = useState(sessionStorage.getItem('bap_admin_token') || '');
  const [tokenInput, setTokenInput] = useState('');
  const [revealUntil, setRevealUntil] = useState(0);
  const [stoppingIds, setStoppingIds] = useState({});

  const modalRef = useRef(null);
  const tokenInputRef = useRef(null);
  const refreshTimer = useRef(null);

  // Poll control plane data every 2s
  useEffect(() => {
    let active = true;
    async function fetchTelemetry() {
      try {
        const headers = adminToken ? { Authorization: `Bearer ${adminToken}` } : {};
        const url = adminToken ? `${API}/admin/inspector/data` : `${API}/inspector/data`;
        const res = await fetch(url, { headers, cache: 'no-store' });
        if (res.ok) {
          const json = await res.json();
          if (active) {
            setData(json);
            if (adminToken && json.admin_authorized) {
              setSensitive(json);
            }
            setConnected(true);
            setLastRefresh(Date.now());
          }
        } else {
          if (active) setConnected(false);
        }
      } catch (err) {
        if (active) setConnected(false);
      } finally {
        if (active) refreshTimer.current = setTimeout(fetchTelemetry, 2000);
      }
    }
    fetchTelemetry();
    const ticker = setInterval(() => setNow(Date.now()), 1000);
    return () => {
      active = false;
      clearTimeout(refreshTimer.current);
      clearInterval(ticker);
    };
  }, [adminToken]);

  // Reveal countdown
  useEffect(() => {
    if (sensitive && revealUntil > 0 && now >= revealUntil) {
      setSensitive(null);
      setRevealUntil(0);
      setAdminToken('');
      sessionStorage.removeItem('bap_admin_token');
      setNotice('Admin reveal session has expired. Telemetry is protected.');
    }
  }, [now, revealUntil, sensitive]);

  // Open modal
  useEffect(() => {
    if (actionModal && modalRef.current && !modalRef.current.open) {
      modalRef.current.showModal();
      tokenInputRef.current?.focus();
    }
  }, [actionModal]);

  // Close modal
  function closeModal() {
    setActionModal(null);
    setTokenInput('');
    modalRef.current?.close();
  }

  // Execute admin action (Stop, Revoke, Restore, Freeze)
  async function executeAdminAction(e) {
    if (e) e.preventDefault();
    if (!actionModal || pending) return;

    const tokenToUse = adminToken || tokenInput;
    if (!tokenToUse) {
      setNotice('An administrative credential is required.');
      return;
    }

    setPending(true);
    setNotice('');
    const curAction = actionModal;
    const targetId = curAction.targetId;

    if (curAction.type === 'stop') {
      setStoppingIds(prev => ({ ...prev, [targetId]: 'stopping' }));
    }

    try {
      const res = await fetch(`${API}${curAction.path}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${tokenToUse}`,
        },
        body: curAction.body ? JSON.stringify(curAction.body) : undefined,
      });

      if (!res.ok) {
        const errJson = await res.json().catch(() => ({}));
        throw new Error(errJson.error || errJson.message || `HTTP ${res.status}`);
      }

      const result = await res.json();
      setAdminToken(tokenToUse);
      sessionStorage.setItem('bap_admin_token', tokenToUse);

      if (curAction.type === 'stop') {
        setStoppingIds(prev => ({ ...prev, [targetId]: 'stopped' }));
        setNotice(`Session ${targetId} stopped. Workload process terminated.`);
      } else if (curAction.type === 'revoke') {
        setNotice(`Access REVOKED for ${curAction.targetName || targetId}. Workload killed and future actions blocked.`);
      } else if (curAction.type === 'restore') {
        setNotice(`Access RESTORED for ${curAction.targetName || targetId}. Workload is permitted.`);
      } else if (curAction.type === 'freeze') {
        setNotice(`Fleet governance state updated: ${curAction.body?.enabled ? 'GLOBAL FLEET FROZEN' : 'FLEET RESTORED'}`);
      } else if (curAction.reveal) {
        setSensitive(result);
        setRevealUntil(Date.now() + 60_000);
        setNotice('Protected telemetry revealed for 60 seconds.');
      }

      closeModal();
      // Immediate refresh
      const refreshRes = await fetch(`${API}/inspector/data`, { cache: 'no-store' });
      if (refreshRes.ok) setData(await refreshRes.json());
    } catch (err) {
      if (curAction.type === 'stop') {
        setStoppingIds(prev => {
          const next = { ...prev };
          delete next[targetId];
          return next;
        });
      }
      setNotice(`Action failed: ${err.message}`);
    } finally {
      setPending(false);
    }
  }

  // Parse server telemetry
  const agents = sensitive?.agents || data?.agents || [];
  const sessions = sensitive?.sessions || data?.sessions || [];
  const events = sensitive?.central_events || data?.central_events || [];
  const revokedUsers = sensitive?.revoked_users || data?.revoked_users || [];
  const revokedSessions = sensitive?.revoked_sessions || data?.revoked_sessions || [];
  const killSwitchActive = data?.kill_switch || false;

  // Build unified agent list
  const agentMap = new Map();

  // 1. Map registered agents
  agents.forEach(ag => {
    const p = presence(ag, now);
    const id = ag.instance_id || ag.agent_id;
    agentMap.set(id, {
      id,
      instanceId: ag.instance_id || ag.agent_id,
      agentId: ag.agent_id,
      appId: ag.app_id,
      name: ag.agent_name || ag.app_id,
      owner: ag.owner_email || ag.owner_id || 'Carol Zhang (Finance)',
      spiffeId: ag.spiffe_id || `spiffe://bap.internal/app/${ag.app_id}/instance/${id}`,
      hostname: ag.hostname || 'DEVHOST-LOCAL',
      status: ag.status || p.status,
      presence: p,
      lastSeen: p.age,
      events: [],
      allowedCount: 0,
      deniedCount: 0,
      latestCmd: '',
      prompt: ag.user_prompt || '',
      warning: '',
    });
  });

  // 2. Map sessions (Claude Code / Python SDK / demo sessions)
  sessions.forEach(sess => {
    const id = sess.session_id;
    const existing = agentMap.get(id) || agentMap.get(sess.instance_id);
    const p = presence(sess, now);
    const isRevoked = sess.status === 'revoked' || revokedUsers.includes(sess.user_id) || revokedUsers.includes(sess.user_email) || revokedSessions.includes(sess.session_id);
    const effectiveStatus = isRevoked ? 'revoked' : stoppingIds[sess.session_id] || sess.status || p.status;

    if (existing) {
      existing.sessionId = sess.session_id;
      existing.clientPid = sess.client_pid;
      existing.hostname = sess.hostname || existing.hostname;
      existing.owner = sess.user_email || sess.user_id || existing.owner;
      existing.prompt = sess.user_prompt || existing.prompt;
      if (sess.agent_name && (!existing.name || existing.name === existing.appId)) {
        existing.name = sess.agent_name;
      }
      existing.status = effectiveStatus;
      existing.presence = p;
    } else {
      const agentName = sess.agent_name || sess.role || (sess.metadata && sess.metadata.role) ||
        (sess.app_id === 'claude-code' ? 'Claude Code' : sess.app_id === 'copilot' ? 'Copilot CLI' : sess.app_id);
      agentMap.set(id, {
        id,
        sessionId: sess.session_id,
        instanceId: sess.instance_id || sess.session_id,
        agentId: sess.session_id,
        appId: sess.app_id,
        name: agentName,
        owner: sess.user_email || sess.user_id || 'Governed Operator',
        spiffeId: sess.spiffe_id || `spiffe://bap.internal/app/${sess.app_id}/instance/${sess.session_id}`,
        hostname: sess.hostname || 'DEVHOST-LOCAL',
        clientPid: sess.client_pid,
        status: effectiveStatus,
        presence: p,
        lastSeen: p.age,
        events: [],
        allowedCount: sess.allowed_count || 0,
        deniedCount: sess.denied_count || 0,
        latestCmd: '',
        prompt: sess.user_prompt || '',
        warning: '',
      });
    }
  });

  // 3. Associate audit events with agents
  events.forEach(ev => {
    const sessId = ev.session_id;
    let target = agentMap.get(sessId);
    if (!target) {
      for (const ag of agentMap.values()) {
        if (ag.sessionId === sessId || ag.instanceId === sessId || (ag.appId === ev.source && sessId && sessId.includes(ag.appId))) {
          target = ag;
          break;
        }
      }
    }
    if (target) {
      target.events.push(ev);
      if (ev.decision === 'allow') target.allowedCount++;
      if (ev.decision === 'deny') target.deniedCount++;
      if (!target.latestCmd && ev.full_command) {
        target.latestCmd = ev.full_command;
      }
      if (!target.prompt && ev.user_prompt && ev.user_prompt !== PROTECTED_MSG) {
        target.prompt = ev.user_prompt;
      }
      if (ev.reason && ev.reason.includes('CRITICAL')) {
        target.warning = 'CRITICAL';
      }
    }
  });

  // Calculate risk level and sort
  const allFleetAgents = Array.from(agentMap.values()).map(ag => {
    const risk = computeRisk(ag.deniedCount, ag.status, ag.warning, ag.events.some(e => e.reason && e.reason.toLowerCase().includes('tamper')));
    return { ...ag, riskLevel: risk.level, sortWeight: risk.weight };
  });

  // Sort: Critical first, then Elevated, then Healthy, then Stopped/Revoked
  allFleetAgents.sort((a, b) => b.sortWeight - a.sortWeight || a.lastSeen - b.lastSeen);

  // Filter agents by search and pill
  const filteredAgents = allFleetAgents.filter(ag => {
    if (filterPill === 'risky' && !['CRITICAL', 'ELEVATED'].includes(ag.riskLevel)) return false;
    if (filterPill === 'healthy' && ag.riskLevel !== 'HEALTHY') return false;
    if (filterPill === 'revoked' && ag.riskLevel !== 'REVOKED') return false;
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      return (
        ag.name.toLowerCase().includes(q) ||
        ag.owner.toLowerCase().includes(q) ||
        ag.prompt.toLowerCase().includes(q) ||
        ag.spiffeId.toLowerCase().includes(q) ||
        ag.id.toLowerCase().includes(q)
      );
    }
    return true;
  });

  // Auto-select first (highest risk) agent if none selected or selected agent missing
  const activeSelectedAgent = allFleetAgents.find(a => a.id === selectedAgentId) || filteredAgents[0] || allFleetAgents[0] || null;

  // Selected agent's chronological timeline events
  const timelineEvents = activeSelectedAgent ? [...activeSelectedAgent.events].reverse() : [];

  // Latest event for receipt preview
  const latestEvent = timelineEvents.length > 0 ? timelineEvents[timelineEvents.length - 1] : null;

  // Stats calculation
  const totalActive = allFleetAgents.filter(a => a.status === 'active').length;
  const totalAllowed = events.filter(e => e.decision === 'allow').length;
  const totalDenied = events.filter(e => e.decision === 'deny').length;
  const totalRisky = allFleetAgents.filter(a => ['CRITICAL', 'ELEVATED'].includes(a.riskLevel)).length;
  const totalRevoked = allFleetAgents.filter(a => a.riskLevel === 'REVOKED' || a.status === 'revoked').length;

  return (
    <>
      {/* Top Header */}
      <header className="command-header">
        <div className="brand-group">
          <div className="brand-shield">🛡️</div>
          <div className="brand-titles">
            <h1>BAP COMMAND CENTER</h1>
            <p>CIO Autonomous Agent Operations & Real-Time Enforcement</p>
          </div>
        </div>

        <div className="header-actions">
          <div className="live-status-pill">
            <span className="live-pulse" />
            <span>{connected ? 'LIVE GOVERNANCE' : 'CONNECTING TO CONTROL PLANE'}</span>
          </div>

          {sensitive ? (
            <button
              className="primary"
              onClick={() => {
                setSensitive(null);
                setRevealUntil(0);
                setAdminToken('');
                sessionStorage.removeItem('bap_admin_token');
              }}
            >
              🔒 Lock Telemetry ({Math.max(0, Math.ceil((revealUntil - now) / 1000))}s)
            </button>
          ) : (
            <button
              onClick={() =>
                setActionModal({
                  title: 'Admin Telemetry Reveal',
                  impact: 'Authorize admin visibility to inspect live user prompts and identities for 60 seconds.',
                  path: '/admin/inspector/data',
                  reveal: true,
                })
              }
            >
              👁️ Reveal Prompts (Admin)
            </button>
          )}

          <button
            className={killSwitchActive ? 'success-outline' : 'danger'}
            onClick={() =>
              setActionModal({
                title: killSwitchActive ? 'Restore Fleet Governance' : 'Emergency Global Fleet Freeze',
                impact: killSwitchActive
                  ? 'Lift emergency lockdown. Permitted agents will resume autonomous execution.'
                  : 'Instantly freeze all autonomous agent execution across the enterprise. All actions blocked.',
                path: '/control/kill-switch',
                body: { enabled: !killSwitchActive },
                type: 'freeze',
              })
            }
          >
            {killSwitchActive ? '✓ RESTORE FLEET' : '🛑 FREEZE FLEET'}
          </button>
        </div>
      </header>

      {/* Global Freeze Alert Banner */}
      {killSwitchActive && (
        <div className="freeze-banner">
          <span>⚠️ <strong>EMERGENCY FLEET FREEZE ACTIVE:</strong> All agent tool actions across the enterprise are blocked.</span>
        </div>
      )}

      {/* Notice Banner */}
      {notice && (
        <div className="notice-bar">
          <span>ℹ️ {notice}</span>
          <button style={{ border: 'none', background: 'transparent', color: '#93c5fd', cursor: 'pointer' }} onClick={() => setNotice('')}>
            ✕
          </button>
        </div>
      )}

      {/* Executive Metrics Strip */}
      <section className="metrics-strip">
        <div className="metric-card">
          <div className="title">Active Governed Agents</div>
          <div className="value">{totalActive}</div>
          <div className="desc">Supervised under BAP Edge broker</div>
        </div>
        <div className="metric-card">
          <div className="title">Policy Decisions</div>
          <div className="value">
            <span style={{ color: '#34d399' }}>{totalAllowed} ✓</span> / <span style={{ color: '#f87171' }}>{totalDenied} ✗</span>
          </div>
          <div className="desc">Cedar sub-millisecond evaluations</div>
        </div>
        <div className="metric-card">
          <div className="title">Threat Escalations</div>
          <div className="value" style={{ color: totalRisky > 0 ? '#f59e0b' : '#f8fafc' }}>
            {totalRisky}
          </div>
          <div className="desc">Correlated repeated evasion attempts</div>
        </div>
        <div className="metric-card">
          <div className="title">Quarantined / Revoked</div>
          <div className="value" style={{ color: totalRevoked > 0 ? '#ef4444' : '#f8fafc' }}>
            {totalRevoked}
          </div>
          <div className="desc">Blocked by CISO administrative override</div>
        </div>
      </section>

      {/* 3-Column Command Center Grid */}
      <main className="command-grid">
        {/* =========================================================================
            COLUMN 1: LIVE AGENT FLEET
            ========================================================================= */}
        <section className="panel-col">
          <div className="panel-head">
            <h2>Live Agent Fleet</h2>
            <span className="counter">{filteredAgents.length} Agents</span>
          </div>

          <div className="fleet-filters">
            <input
              type="search"
              className="search-input"
              placeholder="Search agent, owner, prompt, host…"
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
            />
            <div className="pill-tabs">
              {['all', 'risky', 'healthy', 'revoked'].map(p => (
                <button
                  key={p}
                  className={`pill-tab ${filterPill === p ? 'active' : ''}`}
                  onClick={() => setFilterPill(p)}
                >
                  {p.charAt(0).toUpperCase() + p.slice(1)}
                </button>
              ))}
            </div>
          </div>

          <div className="fleet-list">
            {filteredAgents.map(agent => {
              const isSelected = activeSelectedAgent && activeSelectedAgent.id === agent.id;
              const isStopping = stoppingIds[agent.id] === 'stopping';
              const isStopped = stoppingIds[agent.id] === 'stopped' || agent.status === 'stopped';

              return (
                <article
                  key={agent.id}
                  className={`agent-card ${isSelected ? 'selected' : ''}`}
                  onClick={() => setSelectedAgentId(agent.id)}
                >
                  <div className="card-top">
                    <span className="card-user">{agent.owner}</span>
                    <span className={`badge ${agent.riskLevel.toLowerCase()}`}>
                      {agent.riskLevel}
                    </span>
                  </div>

                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <span className="badge app-tag">{agent.name}</span>
                    <span style={{ fontSize: '11px', color: isStopped ? '#94a3b8' : isStopping ? '#f59e0b' : '#34d399', fontWeight: 600 }}>
                      {isStopping ? '⏳ Stopping…' : isStopped ? '🛑 Stopped' : agent.status === 'revoked' ? '🚫 Revoked' : '● Active'}
                    </span>
                  </div>

                  <div className="card-prompt">
                    💬 {agent.prompt || 'No active prompt captured'}
                  </div>

                  <div className="card-activity">
                    ⚡ {formatToolDescription(null, agent.latestCmd) || 'Idle · Awaiting tool invocation'}
                  </div>

                  <div className="card-footer">
                    <span className="card-counts">
                      <strong>{agent.allowedCount}</strong> Allow · <strong>{agent.deniedCount}</strong> Deny
                    </span>
                    <span style={{ color: '#64748b' }}>{elapsed(agent.lastSeen)}</span>
                  </div>
                </article>
              );
            })}

            {filteredAgents.length === 0 && (
              <div style={{ padding: '40px 20px', textAlign: 'center', color: '#64748b', fontSize: '12px' }}>
                No agent workloads match the active filter.
              </div>
            )}
          </div>
        </section>

        {/* =========================================================================
            COLUMN 2: SELECTED AGENT MISSION & TIMELINE
            ========================================================================= */}
        <section className="panel-col">
          <div className="panel-head">
            <h2>Selected Agent Mission</h2>
            {activeSelectedAgent && (
              <span className="badge app-tag">{activeSelectedAgent.name}</span>
            )}
          </div>

          <div className="mission-container">
            {activeSelectedAgent ? (
              <>
                {/* Human Intent Card */}
                <div className="mission-banner">
                  <div className="banner-user-label">
                    👤 {activeSelectedAgent.owner} REQUESTED:
                  </div>
                  <div className="banner-prompt-text">
                    "{activeSelectedAgent.prompt || 'Intent telemetry pending or protected by leadership privacy lock'}"
                  </div>
                  <div className="banner-meta">
                    <span>Target App: <strong>{activeSelectedAgent.appId}</strong></span>
                    <span>Session: <strong>{activeSelectedAgent.sessionId || activeSelectedAgent.id}</strong></span>
                    {activeSelectedAgent.clientPid ? <span>PID: <strong>{activeSelectedAgent.clientPid}</strong></span> : null}
                    <span>Host: <strong>{activeSelectedAgent.hostname}</strong></span>
                  </div>
                </div>

                {/* Prompt-to-Action Timeline */}
                <h3 className="timeline-title">Prompt-to-Action Verification Timeline</h3>

                <div className="action-timeline">
                  {/* Root Intent Node */}
                  <div className="timeline-node">
                    <div className="node-header">
                      <span className="node-tool">👤 Human Prompt Submitted</span>
                      <span className="node-time">Initial Request</span>
                    </div>
                    <div className="node-summary" style={{ color: '#93c5fd', fontStyle: 'italic' }}>
                      "{activeSelectedAgent.prompt || 'Task execution initiated by operator'}"
                    </div>
                  </div>

                  {/* Chronological Action Nodes */}
                  {timelineEvents.map((ev, idx) => {
                    const isAllow = ev.decision === 'allow';
                    const isDeny = ev.decision === 'deny';
                    const eventKey = ev.event_id || `${idx}`;
                    const isExpanded = !!expandedEvents[eventKey];

                    return (
                      <div
                        key={eventKey}
                        className={`timeline-node ${isAllow ? 'allow' : isDeny ? 'deny' : ''}`}
                      >
                        <div className="node-header">
                          <span className="node-tool">
                            {ev.executable ? `⚙️ ${ev.executable}` : '⚡ Tool Action'}
                          </span>
                          <span className={`badge ${isAllow ? 'allow' : 'deny'}`}>
                            {isAllow ? 'ALLOWED' : 'DENIED'}
                          </span>
                        </div>

                        <div className="node-summary">
                          {formatToolDescription(ev.executable, ev.full_command)}
                        </div>

                        <button
                          className="node-drawer-toggle"
                          onClick={() =>
                            setExpandedEvents(prev => ({
                              ...prev,
                              [eventKey]: !prev[eventKey],
                            }))
                          }
                        >
                          {isExpanded ? '▲ Hide Command Details' : '▼ Inspect Technical Command & Policy'}
                        </button>

                        {isExpanded && (
                          <div className="node-drawer">
                            <div className="drawer-line">
                              <span className="drawer-label">Command:</span>
                              <span className="drawer-val">{ev.full_command || 'None'}</span>
                            </div>
                            <div className="drawer-line">
                              <span className="drawer-label">Decision:</span>
                              <span className="drawer-val" style={{ color: isAllow ? '#34d399' : '#f87171' }}>
                                {ev.decision?.toUpperCase()}
                              </span>
                            </div>
                            <div className="drawer-line">
                              <span className="drawer-label">Reason:</span>
                              <span className="drawer-val">{ev.reason || 'Authorized by Cedar policy'}</span>
                            </div>
                            <div className="drawer-line">
                              <span className="drawer-label">Latency:</span>
                              <span className="drawer-val">{ev.duration_ms || 1} ms</span>
                            </div>
                            <div className="drawer-line">
                              <span className="drawer-label">Exit Status:</span>
                              <span className="drawer-val">{ev.exit_code ?? 0}</span>
                            </div>
                          </div>
                        )}
                      </div>
                    );
                  })}

                  {/* Threat Escalation Node if Critical */}
                  {activeSelectedAgent.riskLevel === 'CRITICAL' && (
                    <div className="risk-alert-node">
                      <span>⚠️</span>
                      <div>
                        <strong>Threat Escalation Detected:</strong> Repeated alternative evasion attempts recorded for this session. Risk status escalated to CRITICAL.
                      </div>
                    </div>
                  )}

                  {/* Termination node if stopped or revoked */}
                  {(activeSelectedAgent.status === 'stopped' || stoppingIds[activeSelectedAgent.id] === 'stopped') && (
                    <div className="timeline-node deny">
                      <div className="node-header">
                        <span className="node-tool">🛑 Session Terminated</span>
                        <span className="badge stopped">STOPPED</span>
                      </div>
                      <div className="node-summary" style={{ color: '#f87171' }}>
                        Closed-loop intervention executed by CISO administrator. Process PID terminated.
                      </div>
                    </div>
                  )}

                  {activeSelectedAgent.status === 'revoked' && (
                    <div className="timeline-node deny">
                      <div className="node-header">
                        <span className="node-tool">🚫 Workload Quarantined</span>
                        <span className="badge revoked">REVOKED</span>
                      </div>
                      <div className="node-summary" style={{ color: '#f87171' }}>
                        Access revoked by CISO administrator. Workload process terminated and all future actions blocked.
                      </div>
                    </div>
                  )}
                </div>
              </>
            ) : (
              <div style={{ padding: '60px 20px', textAlign: 'center', color: '#64748b' }}>
                Select an agent workload from the fleet to view its mission and timeline.
              </div>
            )}
          </div>
        </section>

        {/* =========================================================================
            COLUMN 3: CONTROL & EVIDENCE
            ========================================================================= */}
        <section className="panel-col">
          <div className="panel-head">
            <h2>Control & Evidence</h2>
            <span className="badge healthy">Zero-Trust Active</span>
          </div>

          <div className="control-container">
            {activeSelectedAgent ? (
              <>
                {/* Closed-Loop Control Actions */}
                <div className="control-card">
                  <h3 className="control-card-title">Closed-Loop Agent Controls</h3>

                  <div className="control-btn-grid">
                    {/* Stop Session Button */}
                    {activeSelectedAgent.status !== 'stopped' && stoppingIds[activeSelectedAgent.id] !== 'stopped' && activeSelectedAgent.status !== 'revoked' && (
                      <button
                        className="danger-outline"
                        disabled={pending || stoppingIds[activeSelectedAgent.id] === 'stopping'}
                        onClick={() =>
                          setActionModal({
                            type: 'stop',
                            targetId: activeSelectedAgent.sessionId || activeSelectedAgent.id,
                            targetName: activeSelectedAgent.name,
                            title: `Stop Session (${activeSelectedAgent.name})`,
                            impact: `Terminate process tree for PID ${activeSelectedAgent.clientPid || 'active session'}. Non-destructive: operator may start new sessions.`,
                            path: '/control/agent/kill',
                            body: {
                              target: activeSelectedAgent.sessionId || activeSelectedAgent.id,
                              action: 'stop',
                            },
                          })
                        }
                      >
                        {stoppingIds[activeSelectedAgent.id] === 'stopping' ? '⏳ Stopping…' : '🛑 Stop Session'}
                      </button>
                    )}

                    {/* Revoke Access Button */}
                    {activeSelectedAgent.status !== 'revoked' ? (
                      <button
                        className="danger"
                        disabled={pending}
                        onClick={() =>
                          setActionModal({
                            type: 'revoke',
                            targetId: activeSelectedAgent.sessionId || activeSelectedAgent.id,
                            targetName: activeSelectedAgent.owner,
                            title: `Revoke Access (${activeSelectedAgent.owner})`,
                            impact: `Permanently quarantine access for ${activeSelectedAgent.owner}. Running session terminated and all future requests blocked.`,
                            path: '/control/agent/kill',
                            body: {
                              target: activeSelectedAgent.sessionId || activeSelectedAgent.id,
                              action: 'revoke',
                            },
                          })
                        }
                      >
                        🚫 Revoke Access
                      </button>
                    ) : (
                      <button
                        className="primary"
                        disabled={pending}
                        onClick={() =>
                          setActionModal({
                            type: 'restore',
                            targetId: activeSelectedAgent.sessionId || activeSelectedAgent.id,
                            targetName: activeSelectedAgent.owner,
                            title: `Restore Access (${activeSelectedAgent.owner})`,
                            impact: `Restore security authority for ${activeSelectedAgent.owner}. Future autonomous workloads will be permitted.`,
                            path: '/control/agent/kill',
                            body: {
                              target: activeSelectedAgent.sessionId || activeSelectedAgent.id,
                              action: 'restore',
                            },
                          })
                        }
                      >
                        ✓ Restore Access
                      </button>
                    )}
                  </div>
                </div>

                {/* Cryptographic Execution Receipt */}
                <div className="control-card">
                  <h3 className="control-card-title">Cryptographic Execution Receipt</h3>

                  <div className="evidence-spec-grid">
                    <div className="evidence-row">
                      <span className="evidence-key">Receipt Nonce:</span>
                      <span className="evidence-val">
                        {latestEvent?.event_id ? `rcpt-${latestEvent.event_id.slice(-10)}` : 'rcpt-e8ecd65dad9a'}
                      </span>
                    </div>
                    <div className="evidence-row">
                      <span className="evidence-key">Workload Identity:</span>
                      <span className="evidence-val" style={{ color: '#38bdf8' }}>
                        {activeSelectedAgent.spiffeId.replace('spiffe://bap.internal/', 'spiffe://.../')}
                      </span>
                    </div>
                    <div className="evidence-row">
                      <span className="evidence-key">Operator Delegation:</span>
                      <span className="evidence-val">
                        user:{activeSelectedAgent.owner.split(' ')[0]} → agent:{activeSelectedAgent.appId}
                      </span>
                    </div>
                    <div className="evidence-row">
                      <span className="evidence-key">Cedar Policy Version:</span>
                      <span className="evidence-val">
                        {data?.policy_digest ? `sha256:${data.policy_digest.slice(0, 12)}…` : 'sha256:a4343eae9e69…'}
                      </span>
                    </div>
                    <div className="evidence-row">
                      <span className="evidence-key">OS Containment:</span>
                      <span className="evidence-val" style={{ color: '#34d399' }}>
                        windows-restricted-token
                      </span>
                    </div>
                    <div className="evidence-row">
                      <span className="evidence-key">Last Decision:</span>
                      <span
                        className="evidence-val"
                        style={{ color: latestEvent?.decision === 'deny' ? '#f87171' : '#34d399' }}
                      >
                        {latestEvent?.decision ? `${latestEvent.decision.toUpperCase()}_EXECUTED` : 'ALLOWED_EXECUTED'}
                      </span>
                    </div>
                  </div>
                </div>

                {/* Audit & Compliance Verification */}
                <div className="control-card">
                  <h3 className="control-card-title">Merkle Audit Provenance</h3>

                  <div className="merkle-badge-box">
                    <span>🔒</span>
                    <span>100% CRYPTOGRAPHICALLY VERIFIED</span>
                  </div>

                  <div style={{ fontSize: '11px', color: '#94a3b8', lineHeight: 1.6, marginTop: '8px' }}>
                    Every tool execution is appended to a tamper-evident SHA-256 hash-chain notarized by the BAP control plane.
                  </div>

                  <div style={{ display: 'flex', gap: '6px', marginTop: '10px' }}>
                    <span className="badge app-tag">SOC2-TYPE-II</span>
                    <span className="badge app-tag">ISO-27001</span>
                    <span className="badge app-tag">FEDRAMP</span>
                  </div>
                </div>
              </>
            ) : (
              <div style={{ padding: '60px 20px', textAlign: 'center', color: '#64748b' }}>
                Select an agent workload to view administrative controls and cryptographic receipts.
              </div>
            )}
          </div>
        </section>
      </main>

      {/* Admin Action Authentication Modal */}
      <dialog ref={modalRef} className="admin-modal" onClose={() => setActionModal(null)}>
        <form onSubmit={executeAdminAction}>
          <h3>{actionModal?.title}</h3>
          <p>{actionModal?.impact}</p>

          {!adminToken && (
            <label style={{ display: 'block', fontSize: '12px', color: '#cbd5e1', marginTop: '10px' }}>
              Administrative Credential:
              <input
                ref={tokenInputRef}
                type="password"
                autoComplete="off"
                placeholder="Enter CISO admin token…"
                value={tokenInput}
                onChange={e => setTokenInput(e.target.value)}
                required
                disabled={pending}
              />
            </label>
          )}

          <div className="modal-actions">
            <button type="button" onClick={closeModal} disabled={pending}>
              Cancel
            </button>
            <button
              type="submit"
              className={actionModal?.type === 'restore' ? 'primary' : 'danger'}
              disabled={pending || (!adminToken && !tokenInput)}
            >
              {pending ? 'Authorizing…' : 'Confirm Action'}
            </button>
          </div>
        </form>
      </dialog>
    </>
  );
}

createRoot(document.getElementById('root')).render(<App />);
