import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './browser-tests', workers: 1, fullyParallel: false,
  use: { baseURL: 'http://127.0.0.1:18480', viewport: { width: 1440, height: 1050 } },
  webServer: { command: 'node browser-tests/server.mjs', url: 'http://127.0.0.1:18480/api/v1/health', reuseExistingServer: false, timeout: 120_000 },
});
