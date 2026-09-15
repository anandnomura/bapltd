import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './browser-tests', workers: 1, fullyParallel: false,
  use: { baseURL: 'https://127.0.0.1:18481', ignoreHTTPSErrors: true, viewport: { width: 1440, height: 1050 } },
  webServer: { command: 'node browser-tests/server.mjs', url: 'https://127.0.0.1:18481/dashboard-health', ignoreHTTPSErrors: true, reuseExistingServer: false, timeout: 120_000 },
});
