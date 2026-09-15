import { test } from 'node:test';
import assert from 'node:assert/strict';
import { presence } from '../src/presence.js';

const now = Date.parse('2026-09-14T12:00:00Z');
test('recent departures leave live view at exactly 30 seconds, without deleting history', () => {
 const agent = { status: 'deregistered', last_heartbeat_at: new Date(now).toISOString() };
 assert.equal(presence(agent, now + 29_999).visible, true);
 assert.equal(presence(agent, now + 30_000).visible, false);
 assert.equal(agent.status, 'deregistered');
});
test('revoked agents never age out and late heartbeat is stale, not deregistered', () => {
 assert.equal(presence({ status: 'revoked', created_at: new Date(now).toISOString() }, now + 99_000).visible, true);
 assert.equal(presence({ status: 'active', last_heartbeat_at: new Date(now).toISOString() }, now + 31_000).status, 'stale');
});
test('future clock skew does not produce negative ages', () => {
 assert.equal(presence({ status: 'active', last_heartbeat_at: new Date(now + 100).toISOString() }, now).age, 0);
});
