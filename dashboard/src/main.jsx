import React, { useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { presence, elapsed } from './presence.js';
import './style.css';

const API = '/api/v1';
const protectedText = '[Protected: Leadership Authentication Required]';
function Badge({ status }) { return <span className={`badge ${status || 'neutral'}`}>{(status || 'unknown').replaceAll('_', ' ')}</span>; }

function App() {
  const [data, setData] = useState(null);
	const [controlPlaneURL, setControlPlaneURL] = useState('Loading…');
  const [now, setNow] = useState(Date.now());
  const [connected, setConnected] = useState(false);
  const [lastRefresh, setLastRefresh] = useState(0);
  const [view, setView] = useState('live');
  const [search, setSearch] = useState('');
  const [notice, setNotice] = useState('');
  const [pending, setPending] = useState(false);
  const [action, setAction] = useState(null);
  const [credential, setCredential] = useState('');
  const [sensitive, setSensitive] = useState(null);
  const [revealUntil, setRevealUntil] = useState(0);
  const [selectedAgent, setSelectedAgent] = useState(null);
  const [decisionFilter, setDecisionFilter] = useState('all');
  const dialog = useRef(null);
  const credentialInput = useRef(null);
  const busy = useRef(false);
  const refreshNow = useRef(() => {});

  useEffect(() => {
		fetch('/dashboard-config', { credentials: 'omit', cache: 'no-store' })
			.then(response => response.ok ? response.json() : Promise.reject())
			.then(config => setControlPlaneURL(config.control_plane_url || 'Not reported'))
			.catch(() => setControlPlaneURL('Unavailable'));
	}, []);
	useEffect(() => {
    let disposed = false, timer;
    const controller = new AbortController();
    async function refresh() {
      clearTimeout(timer);
      try {
        const response = await fetch(`${API}/inspector/data`, { credentials: 'omit', cache: 'no-store', signal: AbortSignal.any([controller.signal, AbortSignal.timeout(8000)]) });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const next = await response.json();
        if (!disposed) { setData(next); setConnected(true); setLastRefresh(Date.now()); }
      } catch (error) {
        if (!disposed) { setConnected(false); }
      } finally { if (!disposed) timer = setTimeout(refresh, 2000); }
    }
    refreshNow.current = refresh;
    refresh();
    const tick = setInterval(() => setNow(Date.now()), 1000);
    return () => { disposed = true; controller.abort(); clearTimeout(timer); clearInterval(tick); };
  }, []);
  useEffect(() => {
    if (sensitive && now >= revealUntil) { setSensitive(null); setRevealUntil(0); }
  }, [now, revealUntil, sensitive]);
  useEffect(() => {
    if (action && dialog.current && !dialog.current.open) {
      dialog.current.showModal(); credentialInput.current?.focus();
    }
  }, [action]);

  function closeAction() {
    if (busy.current) return;
    setCredential(''); setAction(null); dialog.current?.close();
  }
  function openAction(next) {
    setNotice(''); setCredential(''); setAction(next);
  }
  async function performAction(event) {
    event.preventDefault();
    if (!credential || busy.current) return;
    if (location.protocol !== 'https:' && !['localhost', '127.0.0.1', '[::1]'].includes(location.hostname)) {
      setNotice('Open this dashboard over HTTPS before sending an administrative credential.'); return;
    }
    busy.current = true; setPending(true);
    const currentAction = action;
    const token = credential;
    setCredential('');
    try {
      const response = await fetch(`${API}${currentAction.path}`, {
        method: currentAction.body ? 'POST' : 'GET',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: currentAction.body ? JSON.stringify(currentAction.body) : undefined,
        credentials: 'omit', cache: 'no-store', redirect: 'error', signal: AbortSignal.timeout(10_000),
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}: action rejected. Check your admin credential and remote-admin configuration.`);
      const result = await response.json();
      if (currentAction.reveal) {
        setSensitive(result); setRevealUntil(Date.now() + 60_000);
		setNotice('Protected telemetry is visible for 60 seconds. No credential has been saved.');
      } else {
		setSensitive(null); setRevealUntil(0);
        setNotice(`${currentAction.title} confirmed by the control plane.`);
        refreshNow.current();
      }
      setAction(null); dialog.current?.close();
    } catch (error) {
      setNotice(`${error.message} No successful outcome has been confirmed.`);
    } finally { busy.current = false; setPending(false); }
  }

  const agents = sensitive?.agents || data?.agents || [];
  const sessions = sensitive?.sessions || data?.sessions || [];
  const events = sensitive?.central_events || data?.central_events || [];
  const serverNow = data?.server_time && lastRefresh ? Date.parse(data.server_time) + now - lastRefresh : now;
  const rows = agents.map(agent => ({ ...agent, presence: presence(agent, serverNow) }));
  const counts = rows.reduce((acc, agent) => { const status = agent.presence.status; if (agent.presence.visible) acc[status] = (acc[status] || 0) + 1; return acc; }, {});
  const filtered = rows.filter(agent => (view === 'history' || agent.presence.visible) && [agent.agent_id, agent.app_id, agent.instance_id, agent.owner_email, agent.hostname].some(value => String(value || '').toLowerCase().includes(search.toLowerCase())));
  const shownEvents = events.filter(event => (!selectedAgent || event.session_id === selectedAgent.sessionID || event.source === selectedAgent.appID && event.spiffe_id === selectedAgent.spiffeID) && (decisionFilter === 'all' || event.decision === decisionFilter)).slice().reverse();
  function promptFor(event) {
    if (!sensitive) return event.user_prompt ? 'Protected · admin reveal required' : 'Not captured';
    const captured = event.user_prompt || sensitive.central_events?.find(item => item.event_id && item.event_id === event.event_id)?.user_prompt;
    const fromSession = sensitive.sessions?.find(item => item.session_id === event.session_id)?.user_prompt;
    return captured && captured !== protectedText ? captured : fromSession && fromSession !== protectedText ? fromSession : 'Not captured';
  }
  function targetFor(agent) {
    return sessions.find(session => session.app_id === agent.app_id && session.instance_id === agent.instance_id && session.status !== 'closed')?.session_id || agent.agent_id;
  }
  const activeAdminState = data?.kill_switch;
  return <>
    <aside className="rail"><a className="brand" href="/dashboard/"><span className="brand-mark">B</span> BAP<span className="brand-caption">DASHBOARD</span></a>
      <p className="nav-label">WORKSPACE</p><a className="nav selected" href="#registry">◉ &nbsp; Agent registry</a><a className="nav" href="#activity">≡ &nbsp; Tool activity</a>
      <div className="rail-foot"><span className="secure-dot"/> Verified HTTPS proxy<br/>Standalone dashboard</div>
    </aside>
    <main>
      <header><div><div className="eyebrow">OBSERVE · UNDERSTAND · CONTROL</div><h1>Agent operations</h1><p className="subtitle">A clear view of your agents and the work they are doing.</p></div><div className="connection"><Badge status={connected ? 'active' : data ? 'stale' : 'connecting'}/><small>{connected ? 'Live connection' : data ? 'Connection lost · showing last known data' : 'Waiting for control plane'}</small></div></header>
      <div className="endpoint-bar"><span>DASHBOARD <strong>{location.origin}</strong> · CONTROL PLANE <strong>{controlPlaneURL}</strong></span><span>{sensitive ? `Admin telemetry reveal · ${Math.max(0, Math.ceil((revealUntil - now) / 1000))}s remaining` : 'Standard view · identity and prompts protected'} · {lastRefresh ? elapsed(now - lastRefresh) : 'Not yet refreshed'}</span></div>
      {notice && <div className="notice" role="status">{notice}</div>}
      {activeAdminState && <div className="freeze-banner"><strong>Fleet freeze is active</strong><span>All connected clients must refresh their policy to observe this state.</span><button disabled={!connected || pending} onClick={() => openAction({ title: 'Restore fleet', path: '/control/kill-switch', body: { enabled: false }, impact: 'Lift the global freeze for all workloads.' })}>Restore fleet · Admin</button></div>}
      <section className="metrics" aria-label="Fleet summary">
        <Metric title="Active agents" value={counts.active || 0} description={`${counts.stale || 0} stale · reporting overdue`}/>
        <Metric title="Offline agents" value={counts.offline || 0} description="Remain visible until explicitly removed"/>
        <Metric title="Revoked agents" value={counts.revoked || 0} description="Require explicit admin restore"/>
        <Metric title="Tool actions" value={events.length} description={`${events.filter(e => e.decision === 'allow').length} allowed · ${events.filter(e => e.decision === 'deny').length} denied`}/>
      </section>
      <section id="registry" className="panel">
        <div className="panel-title"><div><h2>Agent registry <span className="live-dot"/></h2><p>Live presence, identity and administrative status.</p></div><button className="danger-outline" disabled={!connected || pending} onClick={() => openAction({ title: activeAdminState ? 'Restore fleet' : 'Freeze fleet', path: '/control/kill-switch', body: { enabled: !activeAdminState }, impact: activeAdminState ? 'Lift the global freeze for all workloads.' : 'Freeze all workloads when they receive the updated policy. Offline clients may not have received it yet.' })}>{activeAdminState ? 'Restore fleet' : 'Freeze fleet'} · Admin</button></div>
        <div className="toolbar"><div className="tabs" role="group" aria-label="Agent view"><button className={view === 'live' ? 'selected' : ''} onClick={() => setView('live')}>Fleet view</button><button className={view === 'history' ? 'selected' : ''} onClick={() => setView('history')}>All agents & history</button></div><label className="search-label"><span className="sr-only">Search agents</span><input type="search" placeholder="Search agent, user or host…" value={search} onChange={e => setSearch(e.target.value)}/></label></div>
        <div className="table-wrap"><table><thead><tr><th>Agent / instance</th><th>User</th><th>Host</th><th>Status</th><th>Last seen</th><th>Admin action</th></tr></thead><tbody>{filtered.map(agent => <tr key={agent.agent_id}>
          <td><button className="agent-link" onClick={() => { const sessionID = targetFor(agent); setSelectedAgent({ sessionID, appID: agent.app_id, spiffeID: agent.spiffe_id, name: agent.agent_name }); document.getElementById('activity')?.scrollIntoView({ behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth' }); }}>{agent.agent_name || agent.app_id}</button><small>{agent.instance_id || agent.agent_id}</small></td>
          <td>{agent.owner_email || 'Not reported'}</td><td>{agent.hostname || 'Not reported'}</td>
          <td><Badge status={agent.presence.status}/></td>
          <td>{elapsed(agent.presence.age)}</td><td>{['active', 'revoked', 'stale', 'offline'].includes(agent.presence.status) ? <button disabled={!connected || pending} className={agent.status !== 'revoked' ? 'danger-outline' : ''} onClick={() => openAction({ title: agent.status === 'revoked' ? 'Restore authority' : 'Revoke authority', path: '/control/agent/kill', body: { target: agent.agent_id, action: agent.status === 'revoked' ? 'restore' : 'revoke' }, impact: agent.status === 'revoked' ? `Restore governed tool authority for ${agent.agent_name || agent.app_id}.` : `Block future governed tool actions for ${agent.agent_name || agent.app_id}. This does not terminate the Claude process or interrupt text already being generated.` })}>{agent.status === 'revoked' ? 'Restore' : 'Revoke'}</button> : '—'}</td>
        </tr>)}</tbody></table></div>
        {!filtered.length && <div className="empty">{data ? 'No agents in this view. Registered clients appear here as they report activity.' : 'Connecting to your control plane…'}</div>}
        <footer className="panel-foot">Agents remain visible when contact is overdue. Stale and offline are presence signals; revoked is an explicit administrative state.</footer>
      </section>
      <section id="activity" className="panel"><div className="panel-title"><div><h2>Prompts & tool activity</h2><p>Captured intent alongside actual tool actions. No sample events.</p></div><div>{sensitive ? <button onClick={() => { setSensitive(null); setRevealUntil(0); }}>Hide protected data</button> : <button disabled={!connected || pending} onClick={() => openAction({ title: 'Reveal protected telemetry', path: '/admin/inspector/data', reveal: true, impact: 'Show user, session and prompt telemetry in this browser for 60 seconds.' })}>Reveal telemetry · Admin</button>}</div></div>
        <div className="toolbar"><div>{selectedAgent && <button onClick={() => setSelectedAgent(null)}>× {selectedAgent.name} · clear filter</button>}</div><div className="tabs">{['all', 'allow', 'deny'].map(decision => <button key={decision} className={decisionFilter === decision ? 'selected' : ''} onClick={() => setDecisionFilter(decision)}>{decision === 'all' ? 'All actions' : decision === 'allow' ? 'Allowed' : 'Denied'}</button>)}</div></div>
        <div className="table-wrap"><table className="activity-table"><thead><tr><th>Time / agent</th><th>User prompt</th><th>Tool / action</th><th>Decision</th></tr></thead><tbody>{shownEvents.map((event, index) => <tr key={event.event_id || index}><td>{event.timestamp ? new Date(event.timestamp).toLocaleTimeString() : 'Not reported'}<small>{event.source} · {event.user_email || event.user_id || 'User not reported'}</small></td><td>{promptFor(event)}</td><td><strong>{event.executable || 'Tool not reported'}</strong><div>{event.full_command || event.arguments || 'No action details'}</div></td><td><Badge status={event.decision}/><small>{event.reason}</small></td></tr>)}</tbody></table></div>
        {!shownEvents.length && <div className="empty">No tool activity in this view. Clients must send telemetry to this control plane.</div>}
        <footer className="panel-foot">User, session and prompt data requires admin authentication. Reveal expires after 60 seconds; credentials are never saved in browser storage.</footer>
      </section><footer className="page-foot">BAP / Agent operations <span>Live data · Refreshes every 2 seconds</span></footer>
    </main>
    <dialog ref={dialog} className="admin-confirm-dialog" onCancel={event => { event.preventDefault(); closeAction(); }} onClose={() => { if (!busy.current) { setAction(null); setCredential(''); } }}>
      <form onSubmit={performAction}><h2>{action?.title}</h2><p>{action?.impact}</p><label>Admin credential<input ref={credentialInput} type="password" autoComplete="off" value={credential} onChange={event => setCredential(event.target.value)} required disabled={pending}/></label><p>BAP currently uses a shared admin token. Enter it for this action only.</p>{notice && <p role="alert">{notice}</p>}<div><button type="button" onClick={closeAction} disabled={pending}>Cancel</button><button value="confirm" disabled={pending || !credential}>{pending ? 'Confirming…' : 'Confirm action'}</button></div></form>
    </dialog>
  </>;
}
function Metric({ title, value, description }) { return <article><span>{title}</span><strong>{value}</strong><small>{description}</small></article>; }
createRoot(document.getElementById('root')).render(<App/>);
