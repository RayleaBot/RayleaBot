import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests/production',
  timeout: 30_000,
  workers: 1,
  // Desktop verification runs at the 16:9 1920x1080 baseline.
  use: { viewport: { width: 1920, height: 1080 } },
  projects: [{ name: 'real-server', testMatch: '*.real.spec.ts' }],
})
