import { test, expect } from '@playwright/test';

const admin = { Authorization: 'Bearer browser-test-admin' };

test.beforeEach(async ({ request }) => {
  await request.post('/api/v1/sessions/reset', { headers: admin });
  await request.post('/api/v1/control/kill-switch', { headers: admin, data: { enabled: false } });
});

test('privileged prompt reveal and closed-loop revoke use real telemetry', async ({ page, request }) => {
  await request.post('/api/v1/sessions/start', { data: {
    session_id: 'sess-browser', app_id: 'code-review', instance_id: 'laptop-alice',
    user_email: 'alice@example.test', hostname: 'ALICE-MBP',
    user_prompt: 'Review the payment changes for mistakes.',
    intent: { primary: 'BUG_FIX', secondary: ['DATABASE_CHANGE'], tags: ['DATABASE'], confidence: 0.96, classifier_version: 'bap-intent-rules-v1', source: 'claude-user-prompt-submit', prompt_captured: true },
  } });
  await request.post('/api/v1/audit/ingest', { data: [{
    event_id: 'browser-event', session_id: 'sess-browser', source: 'code-review', executable: 'git',
    full_command: 'git diff --stat', decision: 'allow', user_prompt: 'Review the payment changes for mistakes.',
    timestamp: new Date().toISOString(),
  }] });

  await page.goto('/dashboard/');
  await expect(page.getByRole('heading', { name: 'Agent operations' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'code-review', exact: true })).toBeVisible();
  await expect(page.getByText('Bug Fix', { exact: true }).first()).toBeVisible();
  await expect(page.getByText('Review the payment changes for mistakes.', { exact: true })).toHaveCount(0);

  await page.getByRole('button', { name: 'Unlock prompts' }).click();
  await page.getByLabel('Admin credential').fill('wrong');
  await page.getByRole('button', { name: 'Unlock telemetry' }).click();
  await expect(page.getByRole('status')).toContainText('HTTP 403');
  await page.getByLabel('Admin credential').fill('browser-test-admin');
  await page.getByRole('button', { name: 'Unlock telemetry' }).click();
  await expect(page.getByText('Review the payment changes for mistakes.', { exact: true })).toBeVisible();
  await expect(page.getByText('alice@example.test', { exact: true }).first()).toBeVisible();
  expect(await page.evaluate(() => [localStorage.length, sessionStorage.length, document.cookie])).toEqual([0, 0, '']);

  await page.getByRole('button', { name: 'Revoke', exact: true }).click();
  await page.getByRole('button', { name: 'Revoke authority' }).click();
  await expect(page.getByRole('button', { name: 'Restore', exact: true })).toBeVisible();
  expect((await (await request.get('/api/v1/sessions/sess-browser')).json()).status).toBe('revoked');
});

test('production mode hides destructive demo controls and fails closed on unknown audit state', async ({ page }) => {
  await page.route('**/dashboard-config', (route) => route.fulfill({ json: { environment: 'production', demo_mode: false } }));
  await page.route('**/api/v1/inspector/data', (route) => route.fulfill({ json: {
    agents: [], sessions: [], central_events: [], kill_switch: false, server_time: new Date().toISOString(),
  } }));
  await page.goto('/dashboard/');
  await expect(page.getByText('PRODUCTION', { exact: true })).toBeVisible();
  await expect(page.getByText('Unverified', { exact: true })).toBeVisible();
  await expect(page.getByText('tamper-evident chain', { exact: false })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Start Demo' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Trigger Incident' })).toHaveCount(0);
});

test('25-agent orchestration, incident promotion, stop isolation, and Global Freeze', async ({ page }) => {
  await page.goto('/dashboard/');
  await page.getByRole('button', { name: 'Start Demo' }).click();
  await page.getByLabel('Admin credential').fill('browser-test-admin');
  await page.getByRole('button', { name: 'Start demo', exact: true }).click();
  await expect(page.locator('.agent-tile')).toHaveCount(25);
  await expect(page.locator('.metric.current strong')).toHaveText('25');

  await page.getByRole('button', { name: 'Trigger Incident' }).click();
  await page.getByRole('button', { name: 'Trigger incident', exact: true }).click();
  await expect(page.getByRole('complementary', { name: 'Incident panel' }).getByText('Credential Audit Probe')).toBeVisible();
  await expect(page.getByRole('complementary', { name: 'Incident panel' }).getByText('CRITICAL')).toBeVisible();
  await page.screenshot({ path: 'dashboard-desktop.png', fullPage: true, animations: 'disabled' });

  await page.getByRole('button', { name: 'Frontend Squad · Alice', exact: true }).click();
  await page.getByRole('button', { name: 'Stop', exact: true }).click();
  await page.getByRole('button', { name: 'Stop session' }).click();
  await expect(page.locator('.metric.current strong')).toHaveText('25'); // incident session remains active
  await expect(page.getByText('closed', { exact: true }).first()).toBeVisible();

  await page.getByRole('button', { name: 'Global Freeze' }).click();
  await page.getByRole('button', { name: 'Freeze entire fleet' }).click();
  await expect(page.getByText('GLOBAL FREEZE ACTIVE', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Release Freeze' })).toBeVisible();
});

test('500-agent fleet remains searchable, filterable, paged, and responsive', async ({ page }) => {
  const now = new Date().toISOString();
  const sessions = Array.from({ length: 500 }, (_, index) => ({
    session_id: `fleet-session-${index}`, instance_id: `worker-${index}`, app_id: `service-${index % 12}`,
    agent_name: `Worker ${String(index).padStart(3, '0')}`, user_email: `operator-${index}@example.test`,
    hostname: `NODE-${String(index).padStart(3, '0')}`, status: 'active', last_active_at: now,
    allowed_count: index % 9, denied_count: index === 497 ? 3 : index === 498 ? 1 : 0,
    total_events: index % 12, user_prompt: `Process governed workload ${index}`,
    intent: { primary: index % 2 ? 'BUG_FIX' : 'FEATURE_ENHANCEMENT', confidence: 0.9, classifier_version: 'bap-intent-rules-v1', source: 'claude-user-prompt-submit', prompt_captured: true },
  }));
  await page.route('**/api/v1/inspector/data', (route) => route.fulfill({ json: {
    agents: [], sessions, central_events: [], revoked_sessions: [], revoked_users: [],
    chain_status: 'valid', policy_version: 'fleet-test', kill_switch: false, server_time: now,
  } }));

  await page.goto('/dashboard/');
  await expect(page.locator('.agent-tile')).toHaveCount(25);
  await expect(page.locator('.pager')).toContainText('1 / 20');
  await expect(page.getByRole('region', { name: 'Live mission intent mix' })).toContainText('Bug Fix');
  await expect(page.getByRole('complementary', { name: 'Incident panel' }).getByText('Worker 497')).toBeVisible();

  await page.getByLabel('Search fleet').fill('Worker 499');
  await expect(page.locator('.agent-tile')).toHaveCount(1);
  await expect(page.getByRole('button', { name: 'Worker 499', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'At risk' }).click();
  await expect(page.locator('.agent-tile')).toHaveCount(0);

  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: 'dashboard-mobile.png', fullPage: true, animations: 'disabled' });
});
