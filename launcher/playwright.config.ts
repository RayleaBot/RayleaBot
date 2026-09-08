import { defineConfig } from "@playwright/test";

const port = Number.parseInt(process.env.RAYLEA_LAUNCHER_E2E_PORT ?? "5194", 10);
if (!Number.isInteger(port) || port < 1 || port > 65535) {
  throw new Error("RAYLEA_LAUNCHER_E2E_PORT must be a valid TCP port");
}
const origin = `http://127.0.0.1:${port}`;

export default defineConfig({
  testDir: "./tests/e2e",
  workers: 1,
  use: { baseURL: origin, trace: "retain-on-failure" },
  webServer: {
    command: `corepack pnpm exec vite --host 127.0.0.1 --port ${port} --strictPort`,
    url: origin,
    reuseExistingServer: false,
  },
});
