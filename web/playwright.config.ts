import { defineConfig } from '@playwright/test'

const requestedWebPort = Number.parseInt(process.env.RAYLEA_E2E_WEB_PORT ?? '4173', 10)
if (!Number.isInteger(requestedWebPort) || requestedWebPort < 1 || requestedWebPort > 65535) {
  throw new Error('RAYLEA_E2E_WEB_PORT must be a valid TCP port')
}
const webOrigin = `http://127.0.0.1:${requestedWebPort}`

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 30_000,
  fullyParallel: false,
  workers: 1,
  use: {
    baseURL: webOrigin,
    trace: 'on-first-retry',
  },
  webServer: [
    {
      command: 'node tests/e2e/mock-backend.mjs',
      url: 'http://127.0.0.1:4010/__test/ping',
      reuseExistingServer: false,
      cwd: '.',
      env: {
        ...process.env,
        RAYLEA_E2E_WEB_ORIGIN: webOrigin,
      },
    },
    {
      command: `corepack pnpm exec vite --host 127.0.0.1 --port ${requestedWebPort} --strictPort`,
      url: `${webOrigin}/login`,
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
