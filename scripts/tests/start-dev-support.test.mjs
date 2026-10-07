import assert from "node:assert/strict";
import fs from "node:fs/promises";
import http from "node:http";
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
  createServerReadinessWatch,
  createTrustedChildEnvironment,
  createLauncherGoArgs,
  describeCommandFailure,
  loadStartEnvironmentFile,
  isDependencyInstallComplete,
  isProcessRunning,
  removeDependencyInstallState,
  parseDevelopmentServerLease,
  parseBackendEndpointFromConfigText,
  resolveBackendBaseUrl,
  resolveCorepackCliPath,
  resolveInstallMode,
  resolveServerReloadMode,
  resolveStartProfile,
  requestDevelopmentServerShutdown,
  waitForChildProcessExit,
  waitForDevelopmentServerLease,
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
  assert.throws(() => resolveServerReloadMode({ RAYLEA_SERVER_RELOAD: "air" }), /Unsupported/);
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

test("development browser entry follows Vite even when an inherited URL points to the backend", () => {
  const environment = createDevEnvironment({
    env: {
      RAYLEA_WEB_UI_BASE_URL: "http://127.0.0.1:12345/",
      VITE_BACKEND_TARGET: "http://127.0.0.1:23456",
    },
    backendBaseUrl: "http://127.0.0.1:12345",
  });
  assert.equal(environment.RAYLEA_WEB_UI_BASE_URL, "http://127.0.0.1:4173/");
  assert.equal(environment.VITE_BACKEND_TARGET, "http://127.0.0.1:23456");
  assert.equal(createServerDevelopmentEnvironment({ devEnvironment: environment }).RAYLEA_WEB_UI_BASE_URL, environment.RAYLEA_WEB_UI_BASE_URL);
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

test("finds user-installed Corepack when the Windows process PATH omits npm", () => {
  const appData = String.raw`C:\Profiles\Developer Name\AppData\Roaming`;
  const corepack = path.win32.join(appData, "npm", "node_modules", "corepack", "dist", "corepack.js");
  assert.equal(resolveCorepackCliPath({
    nodeExecutablePath: String.raw`C:\Program Files\nodejs\node.exe`,
    env: { APPDATA: appData, Path: String.raw`C:\Windows\System32;C:\Program Files\nodejs` },
    platform: "win32",
    fileExists: (candidate) => candidate === corepack,
  }), corepack);
});

test("keeps Node and PATH Corepack installations ahead of the Windows user fallback", () => {
  const nodeDirectory = String.raw`C:\Program Files\nodejs`;
  const pathDirectory = String.raw`D:\tools`;
  const appData = String.raw`C:\Profiles\developer\AppData\Roaming`;
  const candidates = [nodeDirectory, pathDirectory, path.win32.join(appData, "npm")]
    .map((directory) => path.win32.join(directory, "node_modules", "corepack", "dist", "corepack.js"));
  for (const firstAvailable of [0, 1]) {
    assert.equal(resolveCorepackCliPath({
      nodeExecutablePath: path.win32.join(nodeDirectory, "node.exe"),
      env: { APPDATA: appData, PATH: pathDirectory },
      platform: "win32",
      fileExists: (candidate) => candidates.slice(firstAvailable).includes(candidate),
    }), candidates[firstAvailable]);
  }
});

test("does not resolve a Windows user Corepack installation from a relative APPDATA", () => {
  const relativeCorepack = path.win32.join("relative", "npm", "node_modules", "corepack", "dist", "corepack.js");
  assert.throws(() => resolveCorepackCliPath({
    nodeExecutablePath: String.raw`C:\Program Files\nodejs\node.exe`,
    env: { APPDATA: "relative", PATH: "" },
    platform: "win32",
    fileExists: (candidate) => candidate === relativeCorepack,
  }), /Corepack CLI was not found/);
});

test("finds Corepack in the POSIX npm global prefix beside the Node and PATH bin directories", () => {
  const nodeCorepack = "/opt/node/lib/node_modules/corepack/dist/corepack.js";
  assert.equal(resolveCorepackCliPath({
    nodeExecutablePath: "/opt/node/bin/node",
    env: { PATH: "/usr/local/bin:/usr/bin" },
    platform: "linux",
    fileExists: (candidate) => candidate === nodeCorepack,
  }), nodeCorepack);

  const pathCorepack = "/usr/local/lib/node_modules/corepack/dist/corepack.js";
  assert.equal(resolveCorepackCliPath({
    nodeExecutablePath: "/opt/node/bin/node",
    env: { PATH: "/usr/local/bin:/usr/bin" },
    platform: "darwin",
    fileExists: (candidate) => candidate === pathCorepack,
  }), pathCorepack);
});

test("keeps the Windows Corepack lookup beside node_modules only", () => {
  const libCorepack = String.raw`D:\toolchain\lib\node_modules\corepack\dist\corepack.js`;
  assert.throws(() => resolveCorepackCliPath({
    nodeExecutablePath: String.raw`D:\toolchain\bin\node.exe`,
    env: { PATH: "" },
    platform: "win32",
    fileExists: (candidate) => candidate === libCorepack,
  }), /Corepack CLI was not found/);
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
});

test("keeps build temporary files inside the selected project directory", () => {
  for (const [platform, tempRoot, executable] of [
    ["win32", String.raw`C:\project\.tmp\dev-cache\tmp`, String.raw`C:\tools\node.exe`],
    ["linux", "/project/.tmp/dev-cache/tmp", "/tools/node"],
  ]) {
    const environment = createTrustedChildEnvironment({
      nodeExecutablePath: executable, platform, tempRoot,
      env: { TEMP: "unavailable-system-temp", TMP: "unavailable-system-temp" },
    });
    for (const key of ["TEMP", "TMP", "TMPDIR", "GOTMPDIR"]) assert.equal(environment[key], tempRoot);
  }
  assert.throws(() => createTrustedChildEnvironment({ nodeExecutablePath: "/tools/node", platform: "linux", tempRoot: "relative" }), /must be absolute/);
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
});

test("enables the GTK 3 build tag only for Linux launcher commands", () => {
  assert.deepEqual(createLauncherGoArgs("run", ["."], "linux"), ["run", "-tags", "gtk3", "."]);
  assert.deepEqual(createLauncherGoArgs("run", ["."], "win32"), ["run", "."]);
  assert.deepEqual(createLauncherGoArgs("test", ["./..."], "darwin"), ["test", "./..."]);
});

test("detects dependency installs whose pnpm virtual store is missing", async (t) => {
  const projectDir = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-deps-"));
  t.after(() => fs.rm(projectDir, { recursive: true, force: true }));
  const nodeModulesDir = path.join(projectDir, "node_modules");
  await fs.mkdir(nodeModulesDir, { recursive: true });
  await fs.writeFile(path.join(projectDir, "package.json"), "{}");
  await fs.writeFile(path.join(nodeModulesDir, ".package-map.json"), JSON.stringify({ packages: { ".": { url: ".." } } }));

  assert.equal(await isDependencyInstallComplete(projectDir), false);

  await fs.writeFile(path.join(nodeModulesDir, ".modules.yaml"), JSON.stringify({
    nodeLinker: "isolated",
    virtualStoreDir: path.join(nodeModulesDir, ".pnpm"),
  }));
  assert.equal(await isDependencyInstallComplete(projectDir), false);

  await fs.mkdir(path.join(nodeModulesDir, ".pnpm"));
  assert.equal(await isDependencyInstallComplete(projectDir), true);

  await fs.writeFile(path.join(nodeModulesDir, ".modules.yaml"), JSON.stringify({ nodeLinker: "hoisted" }));
  await fs.rm(path.join(nodeModulesDir, ".pnpm"), { recursive: true, force: true });
  assert.equal(await isDependencyInstallComplete(projectDir), true);
});

test("resets pnpm state and generated commands while preserving package contents", async (t) => {
  const projectDir = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-deps-state-"));
  t.after(() => fs.rm(projectDir, { recursive: true, force: true }));
  const nodeModulesDir = path.join(projectDir, "node_modules");
  await fs.mkdir(nodeModulesDir, { recursive: true });
  for (const name of [".modules.yaml", ".package-map.json", ".pnpm-workspace-state-v1.json", "vue"]) {
    await fs.writeFile(path.join(nodeModulesDir, name), "state\n");
  }
  await fs.mkdir(path.join(nodeModulesDir, ".bin"));
  await fs.writeFile(path.join(nodeModulesDir, ".bin", "vite"), "fixture shim\n");

  await removeDependencyInstallState(projectDir);

  for (const name of [".modules.yaml", ".package-map.json", ".pnpm-workspace-state-v1.json", ".bin"]) {
    await assert.rejects(fs.access(path.join(nodeModulesDir, name)), { code: "ENOENT" });
  }
  assert.equal(await fs.readFile(path.join(nodeModulesDir, "vue"), "utf8"), "state\n");
});

test("rejects an existing virtual store with a missing required package", async (t) => {
  const projectDir = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-partial-deps-"));
  t.after(() => fs.rm(projectDir, { recursive: true, force: true }));
  const nodeModulesDir = path.join(projectDir, "node_modules");
  await fs.mkdir(path.join(nodeModulesDir, ".pnpm"), { recursive: true });
  await fs.writeFile(path.join(projectDir, "package.json"), JSON.stringify({ devDependencies: { vite: "8.2.1" } }));
  await fs.writeFile(path.join(nodeModulesDir, ".modules.yaml"), JSON.stringify({ nodeLinker: "isolated" }));
  assert.equal(await isDependencyInstallComplete(projectDir), false);
});

test("does not accept corrupt pnpm metadata as a complete installation", async (t) => {
  const projectDir = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-corrupt-deps-"));
  t.after(() => fs.rm(projectDir, { recursive: true, force: true }));
  await fs.mkdir(path.join(projectDir, "node_modules"));
  await fs.writeFile(path.join(projectDir, "node_modules", ".modules.yaml"), '{"nodeLinker":');
  assert.equal(await isDependencyInstallComplete(projectDir), false);
});

test("checks transitive packages and command entrypoints while ignoring excluded optional packages", async (t) => {
  const projectDir = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-deps-entries-"));
  t.after(() => fs.rm(projectDir, { recursive: true, force: true }));
  const modules = path.join(projectDir, "node_modules");
  const direct = path.join(modules, "fixture-cli"), transitive = path.join(modules, ".pnpm", "transitive");
  const bin = path.join(modules, ".bin", "fixture-cli" + (process.platform === "win32" ? ".cmd" : ""));
  for (const directory of [direct, transitive, path.dirname(bin)]) await fs.mkdir(directory, { recursive: true });
  await fs.writeFile(path.join(projectDir, "package.json"), JSON.stringify({ devDependencies: { "fixture-cli": "1.0.0" } }));
  await fs.writeFile(path.join(modules, ".modules.yaml"), JSON.stringify({ nodeLinker: "isolated", skipped: ["foreign-native@1"] }));
  await fs.writeFile(path.join(modules, ".package-map.json"), JSON.stringify({ packages: {
    ".": { url: ".." }, "fixture-cli@1": { url: "./fixture-cli" },
    "transitive@1": { url: "./.pnpm/transitive" }, "foreign-native@1": { url: "./.pnpm/foreign-native" },
  } }));
  await fs.writeFile(path.join(direct, "package.json"), JSON.stringify({ bin: { "fixture-cli": "cli.js" } }));
  await fs.writeFile(path.join(direct, "cli.js"), "console.log('fixture');\n");
  await fs.writeFile(bin, "fixture shim\n");
  assert.equal(await isDependencyInstallComplete(projectDir), false);
  await fs.writeFile(path.join(transitive, "package.json"), "{}");
  assert.equal(await isDependencyInstallComplete(projectDir), true);
  await fs.rm(bin);
  assert.equal(await isDependencyInstallComplete(projectDir), false);
  await fs.writeFile(bin, "fixture shim\n");
  await fs.rm(path.join(direct, "cli.js"));
  assert.equal(await isDependencyInstallComplete(projectDir), false);
});

test("waits through server runtime preparation but not past a stall or the cap", () => {
  let current = 0;
  const watch = createServerReadinessWatch({
    timeoutMs: 30_000, prepareStallMs: 120_000, prepareMaxMs: 300_000, now: () => current,
  });

  assert.equal(watch.observe("server: preparing"), false);
  assert.equal(watch.isPreparing(), false);
  current = 29_999;
  assert.equal(watch.hasExpired(), false);
  current = 30_000;
  assert.equal(watch.hasExpired(), true);
  current = 0;
  assert.equal(watch.observe('{"component":"runtime_prepare","stage":"download"}'), true);
  assert.equal(watch.isPreparing(), true);

  current = 100_000;
  assert.equal(watch.hasExpired(), false);
  assert.equal(watch.observe('{"component":"runtime_prepare","downloaded_bytes":1}'), true);

  current = 219_999;
  assert.equal(watch.hasExpired(), false);
  current = 220_000;
  assert.equal(watch.hasExpired(), true);

  current = 0;
  const capped = createServerReadinessWatch({
    timeoutMs: 30_000, prepareStallMs: 120_000, prepareMaxMs: 300_000, now: () => current,
  });
  for (let tick = 0; tick <= 500_000; tick += 100_000) {
    current = tick;
    capped.observe(`{"component":"runtime_prepare","downloaded_bytes":${tick}}`);
  }
  current = 299_999;
  assert.equal(capped.hasExpired(), false);
  current = 300_000;
  assert.equal(capped.hasExpired(), true);
});

test("old preparation output and changing log timestamps do not renew a stalled download", () => {
  let current = 0;
  const watch = createServerReadinessWatch({ timeoutMs: 30, prepareStallMs: 100, prepareMaxMs: 500, now: () => current });
  const event = { component: "runtime_prepare", resource_id: "browser", stage: "download", status: "running", downloaded_bytes: 1 };
  const first = JSON.stringify({ ...event, ts: "first" });
  watch.observe(first);
  current = 90;
  watch.observe(first + '\n{"component":"storage","msg":"unrelated activity"}\n');
  watch.observe(JSON.stringify({ ...event, ts: "second" }));
  current = 100;
  assert.equal(watch.hasExpired(), true);
});

test("reusing a starting environment waits through preparation longer than two minutes", async () => {
  let current = 0;
  const lease = { lease_id: "fixture", owner_pid: 123, ready: false };
  const ready = await waitForDevelopmentServerLease({
    lease, now: () => current, sleep: async (ms) => { current += ms; }, isOwnerRunning: () => true,
    readLease: async () => ({ ...lease, ready: current >= 180_000 }),
  });
  assert.equal(ready.ready, true);
  assert.equal(current, 180_000);
});

test("pending reuse stops at its deadline, owner exit or replacement lease", async () => {
  const lease = { lease_id: "fixture", owner_pid: 123, ready: false };
  for (const outcome of ["timeout", "exit", "replaced", "removed"]) {
    let current = 0;
    const result = waitForDevelopmentServerLease({
      lease, timeoutMs: 500, now: () => current, sleep: async (ms) => { current += ms; },
      isOwnerRunning: () => outcome !== "exit" || current === 0,
      readLease: async () => outcome === "removed" ? null : { ...lease, lease_id: outcome === "replaced" ? "another" : lease.lease_id },
    });
    if (outcome === "timeout") await assert.rejects(result);
    else assert.equal(await result, null);
    assert.ok(current <= 500);
  }
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
