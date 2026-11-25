import { defineConfig, devices } from '@playwright/test';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

// One fresh data dir per run, shared by the server, global setup and workers.
process.env.PB_E2E_DATA ??= fs.mkdtempSync(path.join(os.tmpdir(), 'pb-e2e-'));

export const serverEnv = {
  PB_ADDR: ':8099',
  PB_BASE_URL: 'http://localhost:8099',
  PB_DATA_DIR: process.env.PB_E2E_DATA,
};

export default defineConfig({
  testDir: '.',
  testMatch: '*.spec.ts',
  workers: 1,
  retries: 0,
  globalSetup: './global-setup.ts',
  reporter: [['list']],
  use: {
    baseURL: 'http://localhost:8099',
    locale: 'pt-BR',
    extraHTTPHeaders: { 'X-PB-CSRF': '1' },
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'], locale: 'pt-BR' } }],
  webServer: {
    command: 'make web && go run ./cmd/patchboard serve',
    cwd: '..',
    url: 'http://localhost:8099/healthz',
    timeout: 240_000,
    reuseExistingServer: false,
    stdout: 'ignore',
    stderr: 'pipe',
    env: { ...(process.env as Record<string, string>), ...serverEnv } as Record<string, string>,
  },
});
