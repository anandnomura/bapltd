import { test } from 'node:test';
import assert from 'node:assert/strict';
import { presence } from '../src/presence.js';

const now = Date.parse('2026-09-14T12:00:00Z');
test('deregistered and offline agents are hidden on active fleet screen', () => {
 const agent = { status: 'deregistered', last_heartbeat_at: new Date(now).toISOString() };
 assert.deepEqual(presence(agent, now + 120_000), { status: 'offline', age: 120_000, visible: false });
 assert.equal(agent.status, 'deregistered');
});
test('revoked agents never age out and remain visible for restoration', () => {
 assert.equal(presence({ status: 'revoked', created_at: new Date(now).toISOString() }, now + 99_000).visible, true);
 assert.equal(presence({ status: 'active', last_heartbeat_at: new Date(now).toISOString() }, now + 31_000).status, 'offline');
 assert.equal(presence({ status: 'active', last_heartbeat_at: new Date(now).toISOString() }, now + 31_000).visible, false);
});
test('future clock skew does not produce negative ages', () => {
 assert.equal(presence({ status: 'active', last_heartbeat_at: new Date(now + 100).toISOString() }, now).age, 0);
});
