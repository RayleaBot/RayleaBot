import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests/production',
  timeout: 30_000,
  workers: 1,
  use: { trace: 'on-first-retry' },
  projects: [{ name: 'real-server', testMatch: '*.real.spec.ts' }],
})
