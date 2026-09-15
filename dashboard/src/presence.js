export const DEPARTURE_MS = 30_000;
export const STALE_MS = 30_000;

export function presence(agent, now) {
  const last = Date.parse(agent.last_heartbeat_at || agent.enrolled_at || agent.created_at);
  const age = Number.isFinite(last) ? Math.max(0, now - last) : Infinity;
  if (['deregistered', 'closed', 'inactive'].includes(agent.status)) {
    return { status: 'deregistered', age, visible: age < DEPARTURE_MS, remaining: Math.max(0, Math.ceil((DEPARTURE_MS - age) / 1000)) };
  }
  return { status: agent.status === 'active' && age >= STALE_MS ? 'stale' : agent.status, age, visible: true };
}

export function elapsed(ms) {
  if (!Number.isFinite(ms)) return 'Not reported';
  if (ms < 1000) return 'Just now';
  if (ms < 60_000) return `${Math.floor(ms / 1000)}s ago`;
  if (ms < 3_600_000) return `${Math.floor(ms / 60_000)}m ago`;
  return `${Math.floor(ms / 3_600_000)}h ago`;
}
