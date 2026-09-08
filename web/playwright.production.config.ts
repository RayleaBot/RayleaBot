import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests/production',
  timeout: 30_000,
  workers: 1,
  use: { baseURL: 'http://127.0.0.1:4010', trace: 'on-first-retry' },
  webServer: {
    command: 'corepack pnpm --dir ../examples/plugins/example-config-panel/ui install --frozen-lockfile && corepack pnpm --dir ../examples/plugins/example-config-panel/ui build && node tests/e2e/mock-backend.mjs',
    url: 'http://127.0.0.1:4010/__test/ping',
    reuseExistingServer: false,
    env: { ...process.env, RAYLEA_E2E_WEB_ORIGIN: 'http://127.0.0.1:4010', RAYLEA_E2E_SERVE_WEB_DIST: '1' },
  },
})
