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
  SERVER_RELOAD_WATCH,
  WEB_DEV_PROFILE,
  classifyWebDevServer,
  createDevelopmentControlEnvironment,
  createDependencyInstallEnvironment,
  createDevEnvironment,
  createTrustedChildEnvironment,
  formatLocalLogDate,
  loadStartEnvironmentFile,
  parseBackendEndpointFromConfigText,
  resolveDatedLogPath,
  resolveBackendBaseUrl,
  resolveInstallMode,
  resolveServerReloadMode,
  resolveStartProfile,
  shouldInstallDependencies,
} from "../start-dev-support.mjs";

test("loads the optional root environment file", () => {
  const rootDir = path.join("C:", "RayleaBot");
  const loadedPaths = [];
  assert.equal(
    loadStartEnvironmentFile({
      rootDir,
      loadEnvFile: (environmentPath) => loadedPaths.push(environmentPath),
    }),
    path.join(rootDir, ".env"),
  );
  assert.deepEqual(loadedPaths, [path.join(rootDir, ".env")]);

  assert.doesNotThrow(() => loadStartEnvironmentFile({
    rootDir,
    loadEnvFile: () => {
      throw Object.assign(new Error("missing"), { code: "ENOENT" });
    },
  }));
  assert.throws(() => loadStartEnvironmentFile({
    rootDir,
    loadEnvFile: () => {
      throw Object.assign(new Error("denied"), { code: "EACCES" });
    },
  }), /denied/);
});

test("formats local log dates", () => {
  assert.equal(formatLocalLogDate(new Date(2026, 5, 3, 12, 0, 0)), "2026-06-03");
});

test("resolves dated dev log paths by type", () => {
  const rootDir = path.join("C:", "RayleaBot");
  const date = new Date(2026, 5, 13, 12, 0, 0);

  assert.deepEqual(
    ["server", "web", "launcher", "start"].map((type) => resolveDatedLogPath({
      rootDir,
      scope: "dev",
      type,
      date,
    })),
    [
      path.join(rootDir, "logs", "dev", "server", "2026-06-13.log"),
      path.join(rootDir, "logs", "dev", "web", "2026-06-13.log"),
      path.join(rootDir, "logs", "dev", "launcher", "2026-06-13.log"),
      path.join(rootDir, "logs", "dev", "start", "2026-06-13.log"),
    ],
  );
});

test("resolves start profile", () => {
  assert.equal(resolveStartProfile({}), WEB_DEV_PROFILE);
  assert.equal(resolveStartProfile({ RAYLEA_START_PROFILE: BUILD_PROFILE }), BUILD_PROFILE);
  assert.equal(resolveStartProfile({ RAYLEA_START_PROFILE: LAUNCHER_DEV_PROFILE }), LAUNCHER_DEV_PROFILE);
  assert.throws(() => resolveStartProfile({ RAYLEA_START_PROFILE: "unknown" }), /Unsupported/);
});

test("resolves install mode", () => {
  assert.equal(resolveInstallMode({}), "auto");
  assert.equal(resolveInstallMode({ RAYLEA_START_INSTALL: "always" }), "always");
  assert.equal(resolveInstallMode({ RAYLEA_START_INSTALL: "skip" }), "skip");
  assert.throws(() => resolveInstallMode({ RAYLEA_START_INSTALL: "sometimes" }), /Unsupported/);
});

test("resolves server reload mode", () => {
  assert.equal(resolveServerReloadMode({}), "");
  assert.equal(resolveServerReloadMode({ RAYLEA_SERVER_RELOAD: "watch" }), SERVER_RELOAD_WATCH);
  assert.equal(resolveServerReloadMode({ RAYLEA_SERVER_RELOAD: "air" }), SERVER_RELOAD_WATCH);
  assert.equal(resolveServerReloadMode({ RAYLEA_SERVER_RELOAD: " AIR " }), SERVER_RELOAD_WATCH);
  assert.throws(() => resolveServerReloadMode({ RAYLEA_SERVER_RELOAD: "plugin" }), /Unsupported/);
});

test("creates a shared launcher control environment for development processes", () => {
  assert.deepEqual(
    createDevelopmentControlEnvironment({ generateControlToken: () => "dev-control-token" }),
    { RAYLEA_LAUNCHER_CONTROL_TOKEN: "dev-control-token" },
  );
  assert.throws(
    () => createDevelopmentControlEnvironment({ generateControlToken: () => "  " }),
    /control token is required/,
  );
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

test("creates non-interactive dependency install environment", () => {
  assert.deepEqual(createDependencyInstallEnvironment(), { CI: "true" });
});

test("creates a minimal child environment with the managed Node executable", () => {
  const environment = createTrustedChildEnvironment({
    nodeExecutablePath: String.raw`C:\Users\Raylea\.local\opt\node-v24.18.0-win-x64\node.exe`,
    env: {
      SystemRoot: String.raw`C:\Windows`,
      TEMP: String.raw`C:\Users\Raylea\AppData\Local\Temp`,
      PATH: String.raw`C:\untrusted;C:\Program Files\Go\bin`,
    },
    platform: "win32",
  });

  assert.equal(
    environment.PATH,
    [
      String.raw`C:\Users\Raylea\.local\opt\node-v24.18.0-win-x64`,
      String.raw`C:\Windows\System32`,
      String.raw`C:\Windows`,
    ].join(";"),
  );
  assert.equal(environment.ComSpec, String.raw`C:\Windows\System32\cmd.exe`);
  assert.equal(environment.TEMP, String.raw`C:\Users\Raylea\AppData\Local\Temp`);
  assert.equal(environment.PATH.includes("untrusted"), false);
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

test("classifies rayleabot dev server by backend target", async () => {
  const rayleaServer = await listenHttp((request) => {
    if (request.url === "/__rayleabot-dev/status") {
      return JSON.stringify({
        app: "RayleaBot Web",
        backendTarget: "http://127.0.0.1:8080",
      });
    }
    return "<title>RayleaBot Web</title><script type=\"module\" src=\"/src/main.ts\"></script>";
  }, "application/json; charset=utf-8");
  try {
    const port = rayleaServer.address().port;
    assert.equal(
      await classifyWebDevServer({
        url: `http://127.0.0.1:${port}/`,
        port,
        backendBaseUrl: "http://127.0.0.1:8080/",
        timeoutMs: 100,
      }),
      "rayleabot",
    );
    assert.equal(
      await classifyWebDevServer({
        url: `http://127.0.0.1:${port}/`,
        port,
        backendBaseUrl: "http://127.0.0.1:18080",
        timeoutMs: 100,
      }),
      "occupied",
    );
  } finally {
    await closeServer(rayleaServer);
  }
});

test("classifies rayleabot dev server without status as occupied when backend target is required", async () => {
  const rayleaServer = await listenHttp("<title>RayleaBot Web</title><script type=\"module\" src=\"/src/main.ts\"></script>");
  try {
    const port = rayleaServer.address().port;
    assert.equal(
      await classifyWebDevServer({
        url: `http://127.0.0.1:${port}/`,
        port,
        backendBaseUrl: "http://127.0.0.1:8080",
        timeoutMs: 100,
      }),
      "occupied",
    );
  } finally {
    await closeServer(rayleaServer);
  }
});

async function reservePort() {
  const server = net.createServer();
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const port = server.address().port;
  await closeServer(server);
  return port;
}

async function listenHttp(body, contentType = "text/html; charset=utf-8") {
  const server = http.createServer((_, response) => {
    response.writeHead(200, { "content-type": contentType });
    response.end(typeof body === "function" ? body(_) : body);
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
