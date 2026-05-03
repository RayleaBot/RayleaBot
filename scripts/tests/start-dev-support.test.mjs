import assert from "node:assert/strict";
import fs from "node:fs/promises";
import http from "node:http";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import {
  BUILD_PROFILE,
  LAUNCHER_DEV_PROFILE,
  WEB_DEV_PROFILE,
  classifyWebDevServer,
  createDevEnvironment,
  parseBackendEndpointFromConfigText,
  resolveBackendBaseUrl,
  resolveInstallMode,
  resolveStartProfile,
  shouldInstallDependencies,
} from "../start-dev-support.mjs";

test("resolves start profile and legacy web mode", () => {
  assert.equal(resolveStartProfile({}), WEB_DEV_PROFILE);
  assert.equal(resolveStartProfile({ RAYLEA_START_WEB_MODE: "build" }), BUILD_PROFILE);
  assert.equal(resolveStartProfile({ RAYLEA_START_WEB_MODE: "dev" }), WEB_DEV_PROFILE);
  assert.equal(resolveStartProfile({ RAYLEA_START_PROFILE: LAUNCHER_DEV_PROFILE }), LAUNCHER_DEV_PROFILE);
  assert.throws(() => resolveStartProfile({ RAYLEA_START_PROFILE: "unknown" }), /Unsupported/);
  assert.throws(() => resolveStartProfile({ RAYLEA_START_WEB_MODE: "preview" }), /Unsupported/);
});

test("resolves install mode", () => {
  assert.equal(resolveInstallMode({}), "auto");
  assert.equal(resolveInstallMode({ RAYLEA_START_INSTALL: "always" }), "always");
  assert.equal(resolveInstallMode({ RAYLEA_START_INSTALL: "skip" }), "skip");
  assert.throws(() => resolveInstallMode({ RAYLEA_START_INSTALL: "sometimes" }), /Unsupported/);
});

test("parses backend endpoint from user config", () => {
  const endpoint = parseBackendEndpointFromConfigText(`
server:
  host: 0.0.0.0
  port: "18080"
web:
  exposure_mode: localhost_only
`);

  assert.deepEqual(endpoint, { host: "0.0.0.0", port: 18080 });
});

test("resolves backend base url from env or config", async () => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-start-config-"));
  await fs.mkdir(path.join(root, "config"));
  await fs.writeFile(
    path.join(root, "config", "user.yaml"),
    ["server:", "  host: ::1", "  port: 18081", ""].join("\n"),
    "utf8",
  );

  assert.equal(await resolveBackendBaseUrl({ rootDir: root, env: {} }), "http://[::1]:18081");
  assert.equal(
    await resolveBackendBaseUrl({ rootDir: root, env: { VITE_BACKEND_TARGET: "http://127.0.0.1:28080/" } }),
    "http://127.0.0.1:28080",
  );
});

test("creates dev server environment", () => {
  assert.deepEqual(
    createDevEnvironment({
      env: {},
      backendBaseUrl: "http://127.0.0.1:8080",
    }),
    {
      VITE_BACKEND_TARGET: "http://127.0.0.1:8080",
      VITE_WS_BASE_URL: "http://127.0.0.1:8080",
      RAYLEA_WEB_UI_BASE_URL: "http://127.0.0.1:4173/",
    },
  );

  assert.equal(
    createDevEnvironment({
      env: { VITE_WS_BASE_URL: "ws://127.0.0.1:9000" },
      backendBaseUrl: "http://127.0.0.1:8080",
    }).VITE_WS_BASE_URL,
    "ws://127.0.0.1:9000",
  );
});

test("detects install need from node_modules and lockfile marker", async () => {
  const projectDir = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-start-install-"));
  await fs.writeFile(path.join(projectDir, "pnpm-lock.yaml"), "lockfileVersion: '9.0'\n", "utf8");

  assert.equal(await shouldInstallDependencies({ projectDir, mode: "auto" }), true);

  const nodeModulesDir = path.join(projectDir, "node_modules");
  await fs.mkdir(nodeModulesDir);
  assert.equal(await shouldInstallDependencies({ projectDir, mode: "auto" }), true);

  const markerPath = path.join(nodeModulesDir, ".rayleabot-start-install.stamp");
  await fs.writeFile(markerPath, "installed\n", "utf8");
  const oldTime = new Date("2026-01-01T00:00:00.000Z");
  const newTime = new Date("2026-01-02T00:00:00.000Z");
  await fs.utimes(markerPath, newTime, newTime);
  await fs.utimes(path.join(projectDir, "pnpm-lock.yaml"), oldTime, oldTime);
  assert.equal(await shouldInstallDependencies({ projectDir, mode: "auto" }), false);

  await fs.utimes(path.join(projectDir, "pnpm-lock.yaml"), new Date("2026-01-03T00:00:00.000Z"), new Date("2026-01-03T00:00:00.000Z"));
  assert.equal(await shouldInstallDependencies({ projectDir, mode: "auto" }), true);
  assert.equal(await shouldInstallDependencies({ projectDir, mode: "skip" }), false);
  assert.equal(await shouldInstallDependencies({ projectDir, mode: "always" }), true);
});

test("classifies web dev server port states", async () => {
  const availablePort = await reservePort();
  assert.equal(
    await classifyWebDevServer({
      url: `http://127.0.0.1:${availablePort}/`,
      port: availablePort,
      timeoutMs: 100,
    }),
    "available",
  );

  const rayleaServer = await listenHttp("<title>RayleaBot Web</title><script type=\"module\" src=\"/src/main.ts\"></script>");
  try {
    const port = rayleaServer.address().port;
    assert.equal(
      await classifyWebDevServer({
        url: `http://127.0.0.1:${port}/`,
        port,
        timeoutMs: 100,
      }),
      "rayleabot",
    );
  } finally {
    await closeServer(rayleaServer);
  }

  const unknownServer = await listenHttp("<title>Other App</title>");
  try {
    const port = unknownServer.address().port;
    assert.equal(
      await classifyWebDevServer({
        url: `http://127.0.0.1:${port}/`,
        port,
        timeoutMs: 100,
      }),
      "occupied",
    );
  } finally {
    await closeServer(unknownServer);
  }
});

async function reservePort() {
  const server = net.createServer();
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const port = server.address().port;
  await closeServer(server);
  return port;
}

async function listenHttp(body) {
  const server = http.createServer((_, response) => {
    response.writeHead(200, { "content-type": "text/html; charset=utf-8" });
    response.end(body);
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  return server;
}

async function closeServer(server) {
  await new Promise((resolve, reject) => {
    server.close((error) => {
      if (error) {
        reject(error);
      } else {
        resolve();
      }
    });
  });
}
