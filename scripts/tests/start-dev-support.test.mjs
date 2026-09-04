import assert from "node:assert/strict";
import fs from "node:fs/promises";
import http from "node:http";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import {
  BUILD_PROFILE,
  DEVELOPMENT_SERVER_WATCHER_PID_ENV,
  LAUNCHER_CONTROL_TOKEN_HEADER,
  LAUNCHER_DEV_PROFILE,
  SERVER_RELOAD_WATCH,
  WEB_DEV_PROFILE,
  classifyWebDevServer,
  createDevelopmentControlEnvironment,
  createDevelopmentServerWatcherEnvironment,
  createDevelopmentServerLease,
  createDependencyInstallEnvironment,
  createDevEnvironment,
  createServerDevelopmentEnvironment,
  createTrustedChildEnvironment,
  createLauncherGoArgs,
  describeCommandFailure,
  formatLocalLogDate,
  loadStartEnvironmentFile,
  isProcessRunning,
  parseDevelopmentServerLease,
  parseBackendEndpointFromConfigText,
  resolveDatedLogPath,
  resolveBackendBaseUrl,
  resolveCorepackCliPath,
  resolveInstallMode,
  resolveServerReloadMode,
  resolveStartProfile,
  requestDevelopmentServerShutdown,
  waitForChildProcessExit,
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

test("creates a scoped development watcher environment for the launcher", () => {
  assert.deepEqual(
    createDevelopmentServerWatcherEnvironment({ ownerPid: 12345 }),
    { [DEVELOPMENT_SERVER_WATCHER_PID_ENV]: "12345" },
  );
  assert.throws(
    () => createDevelopmentServerWatcherEnvironment({ ownerPid: 0 }),
    /positive integer/,
  );
});

test("creates and validates a repository-scoped development server lease", async () => {
  const rootDir = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-server-lease-"));
  const serverTmpDir = path.join(rootDir, "server", "tmp");
  const binaryPath = path.join(serverTmpDir, process.platform === "win32"
    ? "raylea-server-dev-123.exe"
    : "raylea-server-dev-123");
  const lease = createDevelopmentServerLease({
    ownerPid: 123,
    rootDir,
    backendBaseUrl: "http://127.0.0.1:1234/",
    binaryPath,
    controlToken: "test-control-token",
    generateLeaseId: () => "test-lease-id",
  });

  assert.deepEqual(
    parseDevelopmentServerLease(JSON.stringify(lease), { rootDir, serverTmpDir }),
    lease,
  );
  assert.throws(
    () => parseDevelopmentServerLease(JSON.stringify(lease), {
      rootDir: path.join(rootDir, "other"),
      serverTmpDir,
    }),
    /another repository/,
  );
  assert.throws(
    () => parseDevelopmentServerLease(JSON.stringify({
      ...lease,
      binary_path: path.join(rootDir, "raylea-server-dev-123"),
    }), { rootDir, serverTmpDir }),
    /outside server\/tmp/,
  );
  assert.throws(
    () => createDevelopmentServerLease({
      ...lease,
      ownerPid: lease.owner_pid,
      rootDir,
      backendBaseUrl: "https://example.com/",
      binaryPath,
      controlToken: lease.control_token,
    }),
    /loopback HTTP/,
  );
});

test("requests graceful shutdown with only the leased control token", async () => {
  const calls = [];
  await requestDevelopmentServerShutdown({
    lease: {
      backend_base_url: "http://127.0.0.1:1234",
      control_token: "test-control-token",
    },
    fetchImpl: async (url, options) => {
      calls.push({ url, options });
      return { status: 202 };
    },
  });

  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, "http://127.0.0.1:1234/api/launcher/shutdown");
  assert.equal(calls[0].options.method, "POST");
  assert.deepEqual(calls[0].options.headers, {
    [LAUNCHER_CONTROL_TOKEN_HEADER]: "test-control-token",
  });
  await assert.rejects(
    requestDevelopmentServerShutdown({
      lease: {
        backend_base_url: "http://127.0.0.1:1234",
        control_token: "test-control-token",
      },
      fetchImpl: async () => ({ status: 403 }),
    }),
    /HTTP 403/,
  );
});

test("classifies development lease owner process state", () => {
  assert.equal(isProcessRunning(42, { kill: () => {} }), true);
  assert.equal(isProcessRunning(42, {
    kill: () => { throw Object.assign(new Error("missing"), { code: "ESRCH" }); },
  }), false);
  assert.equal(isProcessRunning(42, {
    kill: () => { throw Object.assign(new Error("denied"), { code: "EPERM" }); },
  }), true);
});

test("passes only the scoped development origins and control token to the server", () => {
  assert.deepEqual(
    createServerDevelopmentEnvironment({
      devEnvironment: {
        VITE_BACKEND_TARGET: "http://127.0.0.1:8080",
        VITE_WS_BASE_URL: "http://127.0.0.1:8080",
        RAYLEA_WEB_UI_BASE_URL: " http://127.0.0.1:4173/ ",
      },
      controlEnvironment: { RAYLEA_LAUNCHER_CONTROL_TOKEN: "dev-control-token" },
    }),
    {
      RAYLEA_LAUNCHER_CONTROL_TOKEN: "dev-control-token",
      RAYLEA_WEB_UI_BASE_URL: "http://127.0.0.1:4173/",
    },
  );
  assert.throws(
    () => createServerDevelopmentEnvironment(),
    /Web UI base URL is required/,
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
  assert.deepEqual(createDependencyInstallEnvironment(), { CI: "true", pnpm_config_verify_deps_before_run: "false" });
  assert.deepEqual(
    createDependencyInstallEnvironment({ VITE_BACKEND_TARGET: "http://127.0.0.1:1234", CI: "false" }),
    { VITE_BACKEND_TARGET: "http://127.0.0.1:1234", CI: "true", pnpm_config_verify_deps_before_run: "false" },
  );
});

test("finds Corepack on PATH when the selected Node runtime contains only node.exe", () => {
  const selectedNode = String.raw`C:\managed-node\node.exe`;
  const fallbackCorepack = String.raw`D:\toolchain\node_modules\corepack\dist\corepack.js`;
  assert.equal(resolveCorepackCliPath({
    nodeExecutablePath: selectedNode,
    env: { PATH: String.raw`C:\managed-node;D:\toolchain` },
    platform: "win32",
    fileExists: (candidate) => candidate === fallbackCorepack,
  }), fallbackCorepack);
});

test("creates a minimal child environment with the selected Node and Go executables", () => {
  const nodeDirectory = String.raw`C:\toolchains\node-v26.7.0-win-x64`;
  const goDirectory = String.raw`D:\toolchains\Go\bin`;
  const appData = String.raw`C:\Profiles\developer\AppData\Roaming`;
  const localAppData = String.raw`C:\Profiles\developer\AppData\Local`;
  const userProfile = String.raw`C:\Profiles\developer`;
  const goPath = String.raw`D:\go-workspace`;
  const goModCache = String.raw`D:\go-modules`;
  const environment = createTrustedChildEnvironment({
    nodeExecutablePath: path.win32.join(nodeDirectory, "node.exe"),
    goExecutablePath: path.win32.join(goDirectory, "go.exe"),
    env: {
      SystemRoot: String.raw`C:\Windows`,
      APPDATA: appData,
      LOCALAPPDATA: localAppData,
      USERPROFILE: userProfile,
      HOMEDRIVE: "C:",
      HOMEPATH: String.raw`\Profiles\developer`,
      GOPATH: goPath,
      GOMODCACHE: goModCache,
      TEMP: path.win32.join(localAppData, "Temp"),
      PATH: String.raw`C:\untrusted;C:\Program Files\Go\bin`,
    },
    platform: "win32",
  });

  assert.equal(
    environment.PATH,
    [
      nodeDirectory,
      goDirectory,
      String.raw`C:\Windows\System32`,
      String.raw`C:\Windows`,
    ].join(";"),
  );
  assert.equal(environment.ComSpec, String.raw`C:\Windows\System32\cmd.exe`);
  assert.equal(environment.APPDATA, appData);
  assert.equal(environment.LOCALAPPDATA, localAppData);
  assert.equal(environment.USERPROFILE, userProfile);
  assert.equal(environment.HOMEDRIVE, "C:");
  assert.equal(environment.HOMEPATH, String.raw`\Profiles\developer`);
  assert.equal(environment.GOPATH, goPath);
  assert.equal(environment.GOMODCACHE, goModCache);
  assert.equal(environment.TEMP, path.win32.join(localAppData, "Temp"));
  assert.equal(environment.PATH.includes("untrusted"), false);
  assert.equal(environment.PATH.includes(String.raw`C:\Program Files\Go\bin`), false);
});

test("preserves the POSIX home and explicit Go module cache roots", () => {
  const environment = createTrustedChildEnvironment({
    nodeExecutablePath: "/opt/raylea/node/bin/node",
    goExecutablePath: "/opt/raylea/go/bin/go",
    env: {
      HOME: "/home/developer",
      GOPATH: "/work/go",
      GOMODCACHE: "/cache/go-modules",
      PATH: "/untrusted/bin",
      TMP: "/tmp",
    },
    platform: "linux",
  });

  assert.equal(environment.HOME, "/home/developer");
  assert.equal(environment.GOPATH, "/work/go");
  assert.equal(environment.GOMODCACHE, "/cache/go-modules");
  assert.equal(environment.PATH, "/opt/raylea/node/bin:/opt/raylea/go/bin:/usr/local/bin:/usr/bin:/bin");
  assert.equal(environment.PATH.includes("/untrusted/bin"), false);
});

test("enables the GTK 3 build tag only for Linux launcher commands", () => {
  assert.deepEqual(createLauncherGoArgs("run", ["."], "linux"), ["run", "-tags", "gtk3", "."]);
  assert.deepEqual(createLauncherGoArgs("run", ["."], "win32"), ["run", "."]);
  assert.deepEqual(createLauncherGoArgs("test", ["./..."], "darwin"), ["test", "./..."]);
});

test("waits for a child process to release its executable", async () => {
  const child = { exitCode: null, signalCode: null };
  let polls = 0;
  await waitForChildProcessExit(child, {
    timeoutMs: 1_000,
    pollIntervalMs: 1,
    sleep: async () => {
      polls += 1;
      child.signalCode = "SIGTERM";
    },
  });
  assert.equal(polls, 1);

  await assert.rejects(
    waitForChildProcessExit(
      { exitCode: null, signalCode: null },
      { timeoutMs: 0, sleep: async () => undefined },
    ),
    /shutdown timeout/,
  );
});

test("classifies web dev server port states", async () => {
  assert.equal(
    await classifyWebDevServer({
      url: "http://127.0.0.1:5174/",
      port: 5174,
      portAvailable: async () => true,
      timeoutMs: 1_000,
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
        portAvailable: async () => false,
        timeoutMs: 1_000,
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
        portAvailable: async () => false,
        timeoutMs: 1_000,
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
        rootDir: path.resolve("fixture-web"),
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
        projectDir: path.resolve("fixture-web"),
        portAvailable: async () => false,
        timeoutMs: 1_000,
      }),
      "rayleabot",
    );
    assert.equal(await classifyWebDevServer({
      url: `http://127.0.0.1:${port}/`, port, backendBaseUrl: "http://127.0.0.1:8080/",
      projectDir: path.resolve("other-checkout-web"), portAvailable: async () => false, timeoutMs: 1_000,
    }), "occupied");
    assert.equal(
      await classifyWebDevServer({
        url: `http://127.0.0.1:${port}/`,
        port,
        backendBaseUrl: "http://127.0.0.1:18080",
        portAvailable: async () => false,
        timeoutMs: 1_000,
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
        portAvailable: async () => false,
        timeoutMs: 1_000,
      }),
      "occupied",
    );
  } finally {
    await closeServer(rayleaServer);
  }
});

test("describeCommandFailure explains an outdated pnpm lockfile with a repair command", () => {
  const hints = describeCommandFailure(
    "[ERR_PNPM_OUTDATED_LOCKFILE] Cannot install with \"frozen-lockfile\" because pnpm-lock.yaml is not up to date with .rayleabot\\sdk\\vue\\package.json",
    { cwd: "C:/workspace/plugin-fortune" },
  );
  assert.ok(hints.some((hint) => hint.includes("corepack pnpm install --no-frozen-lockfile")));
  assert.ok(hints.some((hint) => hint.includes("C:/workspace/plugin-fortune")));
  assert.ok(hints.some((hint) => hint.includes("ui/")));
});

test("describeCommandFailure explains a lockfile config mismatch", () => {
  const hints = describeCommandFailure(
    "[ERR_PNPM_LOCKFILE_CONFIG_MISMATCH] Cannot proceed with the frozen installation.",
    { cwd: "/repo/web" },
  );
  assert.ok(hints.some((hint) => hint.includes("corepack pnpm install --no-frozen-lockfile")));
});

test("describeCommandFailure ignores unrelated or empty output", () => {
  assert.deepEqual(describeCommandFailure("go: unresolved import", { cwd: "/repo" }), []);
  assert.deepEqual(describeCommandFailure("", { cwd: "/repo" }), []);
});

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
