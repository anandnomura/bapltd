import { test, expect } from '@playwright/test';

const admin = { Authorization: 'Bearer browser-test-admin' };
test.beforeEach(async ({ request }) => {
  await request.post('/api/v1/sessions/reset', { headers: admin });
  await request.post('/api/v1/control/kill-switch', { headers: admin, data: { enabled: false } });
});

test('real session: prompt protection, admin action, failed credential and restore', async ({ page, request }) => {
  await request.post('/api/v1/sessions/start', { data: { session_id: 'sess-browser', app_id: 'code-review', instance_id: 'laptop-alice', user_email: 'alice@example.test', hostname: 'ALICE-MBP', user_prompt: 'Review the payment changes for mistakes.' } });
  await request.post('/api/v1/audit/ingest', { data: [{ event_id: 'browser-event', session_id: 'sess-browser', source: 'code-review', executable: 'git', full_command: 'git diff --stat', decision: 'allow', user_prompt: 'Review the payment changes for mistakes.', timestamp: new Date().toISOString() }] });
  await page.goto('/dashboard/');
  await expect(page.getByRole('heading', { name: 'Agent operations' })).toBeVisible();
  await expect(page.getByText('http://127.0.0.1:18480', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'code-review', exact: true })).toBeVisible();
  await expect(page.getByText('alice@example.test', { exact: true })).toHaveCount(0);
  await expect(page.getByText('Review the payment changes for mistakes.', { exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Reveal telemetry' }).click();
  await page.getByLabel('Admin credential').fill('wrong');
  await page.getByRole('button', { name: 'Confirm action' }).click();
  await expect(page.getByRole('dialog').getByRole('alert')).toContainText('HTTP 403');
  await page.getByLabel('Admin credential').fill('browser-test-admin');
  await page.getByRole('button', { name: 'Confirm action' }).click();
  await expect(page.getByText('Review the payment changes for mistakes.', { exact: true })).toBeVisible();
  await expect(page.getByText('alice@example.test', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Revoke', exact: true }).click();
  await expect(page.getByLabel('Admin credential')).toHaveValue('');
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();
  expect((await (await request.get('/api/v1/sessions/sess-browser')).json()).status).toBe('active');
  await page.getByRole('button', { name: 'Revoke', exact: true }).click();
  await page.getByLabel('Admin credential').fill('browser-test-admin');
  await page.getByRole('button', { name: 'Confirm action' }).click();
  await expect(page.getByRole('button', { name: 'Restore', exact: true })).toBeVisible();
  expect(await page.evaluate(() => [localStorage.length, sessionStorage.length, document.cookie])).toEqual([0, 0, '']);
  await expect(page.getByText('Review the payment changes for mistakes.', { exact: true })).toHaveCount(0);
});

test('offline agents remain visible and untrusted names render as text', async ({ page }) => {
  const agent = { agent_id: 'history-agent', agent_name: '<img src=x onerror=alert(1)>', status: 'deregistered', last_heartbeat_at: new Date(Date.now() - 31_000).toISOString() };
  await page.route('**/api/v1/inspector/data', route => route.fulfill({ json: { agents: [agent], sessions: [], central_events: [], server_time: new Date().toISOString() } }));
  await page.goto('/dashboard/');
  await expect(page.getByRole('button', { name: agent.agent_name, exact: true })).toBeVisible();
  await expect(page.locator('img')).toHaveCount(0);
});

test('light dashboard screenshot with real API fixtures and narrow layout', async ({ page, request }) => {
  for (const [name, app, host] of [['Maya Chen', 'payments-review', 'MAYA-MBP'], ['James Wilson', 'infra-assistant', 'DEVBOX-07'], ['Sofia Patel', 'analytics-worker', 'BUILD-03']]) {
    const id = app + '-showcase';
    await request.post('/api/v1/sessions/start', { data: { session_id: id, app_id: app, instance_id: host.toLowerCase(), user_email: name.toLowerCase().replace(' ', '.') + '@example.test', hostname: host } });
    await request.post('/api/v1/audit/ingest', { data: [{ event_id: id, session_id: id, source: app, executable: 'git', full_command: 'git status --short', decision: 'allow', timestamp: new Date().toISOString() }] });
  }
  await page.goto('/dashboard/');
  await expect(page.getByRole('button', { name: 'payments-review', exact: true })).toBeVisible();
  await page.screenshot({ path: 'dashboard-desktop.png', fullPage: true, animations: 'disabled' });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole('heading', { name: 'Agent operations' })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: 'dashboard-mobile.png', fullPage: true, animations: 'disabled' });
});
