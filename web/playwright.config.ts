import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 30_000,
  fullyParallel: false,
  workers: 1,
  use: {
    baseURL: 'http://127.0.0.1:4173',
    trace: 'on-first-retry',
  },
  webServer: [
    {
      command: 'node tests/e2e/mock-backend.mjs',
      url: 'http://127.0.0.1:4010/__test/ping',
      reuseExistingServer: false,
      cwd: '.',
    },
    {
      command: 'pnpm dev',
      url: 'http://127.0.0.1:4173/login',
      reuseExistingServer: false,
      cwd: '.',
      env: {
        ...process.env,
        VITE_BACKEND_TARGET: 'http://127.0.0.1:4010',
        VITE_WS_BASE_URL: 'ws://127.0.0.1:4010',
      },
    },
  ],
})
