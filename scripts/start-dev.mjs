import { resolveGoExecutablePath } from "./process-invocation.mjs";
import { spawn } from "node:child_process";
import fs from "node:fs";
import fsp from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { createBuildCache, createGoInputRegistry, fingerprint, treeInputs, writeIfChanged, goInputTemplate, parseGoInputs } from "./dev-build-cache.mjs";
import { developmentRequest, synchronizeDevelopmentPlugin } from "./development-client.mjs";
import { redactLogLine } from "./log-redaction.mjs";
import { createDevChildOutput, createDevConsole } from "./dev-console.mjs";
import { currentDevelopmentPlugins, prepareStableDevelopmentPlugins, runDevelopmentPluginBatch, startPreparedDevelopmentRuntime } from "./development-pipeline.mjs";
import {
  BUILD_PROFILE,
  LAUNCHER_CONTROL_TOKEN_ENV,
  LAUNCHER_DEV_PROFILE,
  SERVER_RELOAD_WATCH,
  WEB_DEV_BASE_URL,
  WEB_DEV_PORT,
  WEB_DEV_PROFILE,
  classifyWebDevServer,
  createDevelopmentControlEnvironment,
  createDevelopmentServerWatcherEnvironment,
  createDevelopmentServerLease,
  createDevEnvironment,
  createDatedLogWriter,
  createDependencyInstallEnvironment,
  createServerDevelopmentEnvironment,
  createServerReadinessWatch,
  ensureDependencyInstall,
  isProcessRunning,
  parseDevelopmentServerLease,
  requestDevelopmentServerShutdown,
  createTrustedChildEnvironment,
  describeCommandFailure,
  formatUTCLogTimestamp,
  loadStartEnvironmentFile,
  resolveBackendBaseUrl,
  resolveInstallMode,
  resolveCorepackCliPath,
  resolveServerReloadMode,
  resolveStartProfile,
  waitForChildProcessExit,
  waitForDevelopmentServerLease,
} from "./start-dev-support.mjs";
import {
  createDevelopmentReloadQueue,
  currentPluginPlatform,
  loadPluginWorkspace,
  mirrorVueSDK,
  PLUGIN_DEV_OFF,
  PLUGIN_DEV_WATCH,
  preparePluginGoWorkspace,
  resolvePluginDevMode,
  watchPluginWorkspace,
} from "./plugin-dev-workspace.mjs";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(scriptDir, "..");
loadStartEnvironmentFile({ rootDir });
let corepackCliPath = "";
const webDir = path.join(rootDir, "web");
const serverDir = path.join(rootDir, "server");
const serverDistDir = path.join(serverDir, "dist");
const serverBinaryName = process.platform === "win32"
  ? "raylea-server.exe"
  : "raylea-server";
const serverTmpDir = path.join(serverDir, "tmp");
const serverDevBinaryName = process.platform === "win32"
  ? `raylea-server-dev-${process.pid}.exe`
  : `raylea-server-dev-${process.pid}`;
const serverDevBinaryPath = path.join(serverTmpDir, serverDevBinaryName);
const serverDevCandidateBinaryName = process.platform === "win32"
  ? `raylea-server-dev-${process.pid}-next.exe`
  : `raylea-server-dev-${process.pid}-next`;
const serverDevCandidateBinaryPath = path.join(serverTmpDir, serverDevCandidateBinaryName);
const serverDevPreviousBinaryPath = `${serverDevBinaryPath}.previous`;
const serverDevLeasePath = path.join(rootDir, ".tmp", "server-dev-runtime.json");
const serverDevTakeoverTimeoutMs = 30_000;
const serverReloadDebounceMs = 500;
const childGoCacheDir = path.join(rootDir, ".tmp", "gocache");
const childTempDir = path.join(rootDir, ".tmp", "dev-cache", "tmp");
const pluginWorkspacePath = path.resolve(rootDir, process.env.RAYLEA_PLUGIN_WORKSPACE || "plugin-workspace.local.json");
const pluginDevRoot = path.join(rootDir, ".tmp", "plugin-dev");
const pluginDevArtifactRoot = path.join(pluginDevRoot, "artifacts");
const baseChildEnvironment = {
  GOCACHE: childGoCacheDir,
};
const developmentControlEnvironment = createDevelopmentControlEnvironment();
let developmentControlToken = developmentControlEnvironment[LAUNCHER_CONTROL_TOKEN_ENV];
const developmentServerWatcherEnvironment = createDevelopmentServerWatcherEnvironment({
  ownerPid: process.pid,
});
const launcherDir = path.join(rootDir, "launcher");
const developmentLogs = Object.fromEntries(["web", "launcher", "server", "start", "build"].map((type) => [
  type, createDatedLogWriter({ rootDir, scope: "dev", type }),
]));
const terminal = createDevConsole({ record: (message, level) => writeStartLog(`[${level.toUpperCase()}] ${message}\n`) });
const startupStarted = Date.now();
let startupReported = false;
const longRunningChildren = new Set();
const childOutputTails = new WeakMap();
const childHiddenOutputTails = new WeakMap();
const childReadinessWatches = new WeakMap();
const cleanupCallbacks = new Set();
let shuttingDown = false;
let cleanupPromise;
let activeServerDevLeaseId = "";
let startupIdentity = "";
let reusedRuntime = false;
const cacheDir = path.join(rootDir, ".tmp", "dev-cache");
const buildCache = createBuildCache(cacheDir, (message) => log(message, "debug"));
const goInputRegistry = createGoInputRegistry();
const activeGoInputs = goInputRegistry.files;
let onGoInputsChanged = () => {};
let toolIdentity;
const scriptInputs = ["start-dev.mjs", "dev-build-cache.mjs", "plugin-dev-workspace.mjs", "development-pipeline.mjs", "start-dev-support.mjs", "development-client.mjs", "file-content-tracker.mjs", "log-redaction.mjs"].map((name) => path.join(scriptDir, name));
const reportedPluginErrors = new Map();
const startupInputs = [...scriptInputs, path.join(scriptDir, "dev-console.mjs")];

await fsp.mkdir(childGoCacheDir, { recursive: true });
await fsp.mkdir(childTempDir, { recursive: true });

process.once("SIGINT", () => {
  void shutdown(130);
});
process.once("SIGTERM", () => {
  void shutdown(143);
});

try {
  corepackCliPath = resolveCorepackCliPath();
  Object.assign(baseChildEnvironment, createTrustedChildEnvironment({
    nodeExecutablePath: process.execPath,
    goExecutablePath: resolveGoExecutablePath(),
    tempRoot: childTempDir,
  }));
  await main();
  await cleanup();
} catch (error) {
  if (!shuttingDown) {
    terminal.abortStartup();
    log(`${startupReported ? "开发流程" : "启动"}失败：${error?.message ?? error}`, "error");
    log(`启动日志：${relativePath(developmentLogs.start.path)}`, "error");
    process.exitCode = 1;
  }
  await cleanup();
} finally {
  await closeLogs();
}

async function main() {
  const profile = resolveStartProfile(process.env);
  const installMode = resolveInstallMode(process.env);
  const serverReloadMode = resolveServerReloadMode(process.env);
  const pluginDevMode = resolvePluginDevMode(process.env, fs.existsSync(pluginWorkspacePath));
  if (pluginDevMode === PLUGIN_DEV_WATCH && serverReloadMode !== SERVER_RELOAD_WATCH) {
    throw new Error("RAYLEA_PLUGIN_DEV=watch requires RAYLEA_SERVER_RELOAD=watch.");
  }
  const pluginDev = { mode: pluginDevMode, workspacePath: pluginWorkspacePath };
  terminal.intro({ profile, nodeVersion: process.version, serverReload: serverReloadMode, pluginDev: pluginDevMode });
  await fsp.mkdir(cacheDir, { recursive: true });
  toolIdentity = {
    node: process.version, nodePath: process.execPath, platform: process.platform, arch: process.arch,
    go: JSON.parse(await captureCommand("go", ["env", "-json", "GOVERSION", "GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS", "GOAMD64", "GOARM64", "GOROOT", "GOMODCACHE"])),
  };
  startupIdentity = await fingerprint(startupInputs, { profile, serverReloadMode, pluginDevMode, pluginWorkspacePath, toolIdentity });

  log(`启动配置：profile=${profile} install=${installMode} server_reload=${serverReloadMode || "off"} plugin_dev=${pluginDevMode}`, "debug");

  if (profile === BUILD_PROFILE) {
    await runBuildProfile({ installMode, pluginDev });
    return;
  }

  const backendBaseUrl = await resolveBackendBaseUrl({ rootDir, env: process.env });
  const devEnvironment = createDevEnvironment({ env: process.env, backendBaseUrl });
  startupIdentity = await fingerprint([], { startupIdentity, devEnvironment, installMode });
  const serverDevEnvironment = createServerDevelopmentEnvironment({
    devEnvironment,
    controlEnvironment: developmentControlEnvironment,
  });
  await fsp.mkdir(pluginDevArtifactRoot, { recursive: true });
  serverDevEnvironment.RAYLEA_DEV_ARTIFACT_ROOT = pluginDevArtifactRoot;
  log(`后端地址：${backendBaseUrl}`);
  if (serverReloadMode === SERVER_RELOAD_WATCH && await reuseDevelopmentRuntime(backendBaseUrl)) {
    if (pluginDev.mode === "sync") await synchronizeOnlinePlugins(await buildDevelopmentPlugins(pluginDev), backendBaseUrl);
    await prepareLauncher(installMode);
    if (!shouldSkipLaunch()) await launchCachedLauncher(devEnvironment);
    else reportStartup();
    return;
  }

  if (profile === WEB_DEV_PROFILE) {
    await runWebDevProfile({ installMode, devEnvironment, serverDevEnvironment, serverReloadMode, backendBaseUrl, pluginDev });
    return;
  }
  if (profile === LAUNCHER_DEV_PROFILE) {
    await runLauncherDevProfile({ installMode, devEnvironment, serverDevEnvironment, serverReloadMode, backendBaseUrl, pluginDev });
    return;
  }

  throw new Error(`Unsupported profile: ${profile}`);
}

async function runBuildProfile({ installMode, pluginDev }) {
  await terminal.phase("Web 静态资源", async () => {
    await ensureDependencies("Web", webDir, installMode);
    await buildCache.run("web-static", {
      inputs: async () => [...await treeInputs(webDir), ...scriptInputs], identity: toolIdentity,
      outputs: [path.join(webDir, "dist"), path.join(webDir, "dist", "index.html")],
      build: () => runCommand("构建 Web 静态资源", "pnpm", ["run", "build"], { cwd: webDir }),
    });
  });
  await terminal.phase("Server 与开发插件", async () => {
    await buildServer();
    await syncDevelopmentPlugins(pluginDev, path.join(serverDistDir, serverBinaryName));
  });
  await prepareLauncher(installMode);
  if (shouldSkipLaunch()) {
    reportStartup();
    return;
  }
  await launchCachedLauncher({ RAYLEA_WEB_UI_BASE_URL: "" });
}

async function runWebDevProfile({ installMode, devEnvironment, serverDevEnvironment, serverReloadMode, backendBaseUrl, pluginDev }) {
  await terminal.phase("Server 与开发插件", () => ensureServerRuntime({ serverReloadMode, backendBaseUrl, pluginDev, serverDevEnvironment }));
  await terminal.phase("Web 开发服务", async () => {
    await ensureDependencies("Web", webDir, installMode);
    await ensureWebDevServer(devEnvironment);
  });
  await prepareLauncher(installMode);
  if (shouldSkipLaunch()) {
    reportStartup();
    return;
  }
  await launchCachedLauncher(devEnvironment);
}

async function runLauncherDevProfile({ installMode, devEnvironment, serverDevEnvironment, serverReloadMode, backendBaseUrl, pluginDev }) {
  await terminal.phase("Server 与开发插件", () => ensureServerRuntime({ serverReloadMode, backendBaseUrl, pluginDev, serverDevEnvironment }));
  await terminal.phase("Web 开发服务", async () => {
    await ensureDependencies("Web", webDir, installMode);
    await ensureWebDevServer(devEnvironment);
  });
  await terminal.phase("Launcher 依赖", () => ensureDependencies("Launcher", launcherDir, installMode));
  if (shouldSkipLaunch()) {
    reportStartup();
    return;
  }
  await markRuntimeReady();
  await runCommand("启动 Launcher 开发模式", "pnpm", ["run", "dev"], {
    cwd: launcherDir,
    windowsHide: false,
    env: createLauncherToolEnvironment({
      ...devEnvironment,
      ...developmentControlEnvironment,
      ...(serverReloadMode === SERVER_RELOAD_WATCH ? developmentServerWatcherEnvironment : {}),
    }),
    logType: "launcher",
  });
}

async function buildServer() {
  await cachedGoBuild("server-static", { cwd: serverDir, main: "./cmd/raylea-server", output: path.join(serverDistDir, serverBinaryName) });
}

function nativeExecutableSuffix(platform) {
  return platform === "windows-x64" ? ".exe" : "";
}

async function buildDevelopmentPlugins(pluginDev, pluginIDs) {
  return terminal.phase("准备开发插件", (progress) => prepareDevelopmentPlugins(pluginDev, pluginIDs, progress));
}

async function readDevelopmentWorkspace(pluginDev) {
  if (pluginDev?.mode === PLUGIN_DEV_OFF) return { workspaceVersion: "2", plugins: [], invalidPlugins: [] };
  const workspace = await loadPluginWorkspace(pluginDev.workspacePath, { onPluginError: ({ path: source, error }) => {
    if (reportedPluginErrors.get(source) !== error.message) log(`跳过损坏的开发插件 ${source}：${error.message}`, "warn");
    reportedPluginErrors.set(source, error.message);
  } });
  const invalid = new Set(workspace.invalidPlugins?.map((plugin) => plugin.path));
  for (const source of reportedPluginErrors.keys()) if (!invalid.has(source)) reportedPluginErrors.delete(source);
  return workspace;
}

function reportDevelopmentPluginFailure(plugin, error) {
  log(`开发插件 ${plugin.id} 未更新：${error.message}；等待相关源码变化后重试。`, error.code === "DEV_INPUT_CHANGED" ? "debug" : "warn");
}

async function prepareDevelopmentPlugins(pluginDev, pluginIDs, progress) {
  const workspace = await readDevelopmentWorkspace(pluginDev);
  const platform = currentPluginPlatform();
  const helper = path.join(cacheDir, "raylea-plugin" + nativeExecutableSuffix(platform));
  const plugins = pluginIDs === undefined ? workspace.plugins : workspace.plugins.filter((plugin) => pluginIDs.includes(plugin.id));
  let helperReady;
  const result = await runDevelopmentPluginBatch(plugins, async (plugin) => {
    assertDevelopmentRunning();
    helperReady ??= cachedGoBuild("plugin-builder", { cwd: path.join(rootDir, "sdk", "go"), main: "./cmd/raylea-plugin", output: helper });
    await helperReady;
    // Unrelated plugin modules must not participate in this plugin's Go graph.
    const goWorkPath = await preparePluginGoWorkspace({ directory: path.join(pluginDevRoot, "workspaces"), sdkGoPath: path.join(rootDir, "sdk", "go"), plugin });
    const environment = {
      GOWORK: goWorkPath, CGO_ENABLED: "0", RAYLEA_PLUGIN_BUILD_USE_WORKSPACE: "1",
      RAYLEA_PLUGIN_BUILD_NODE: process.execPath, RAYLEA_PLUGIN_BUILD_COREPACK_CLI: corepackCliPath,
    };
    const uiDir = path.join(plugin.path, "ui");
    const hasUI = fs.existsSync(path.join(uiDir, "package.json"));
    // A plugin whose UI links more than the Vue SDK prepares those links with
    // its own scripts/prepare-ui.mjs, given the Vue SDK directory.
    const prepareUI = path.join(plugin.path, "scripts", "prepare-ui.mjs");
    const hasPrepareUI = hasUI && fs.existsSync(prepareUI);
    if (hasUI) {
      if (hasPrepareUI) await runCommand("准备插件 UI 依赖 " + plugin.id, process.execPath, [prepareUI, path.join(rootDir, "sdk", "vue")], { cwd: plugin.path });
      await mirrorVueSDK({ sdkVuePath: path.join(rootDir, "sdk", "vue"), pluginPath: plugin.path });
      await ensureDependencies(plugin.id + "-ui", uiDir, resolveInstallMode(process.env), [path.join(rootDir, "sdk", "vue", "package.json")]);
      await buildCache.run(plugin.id + "-ui", {
        inputs: async () => [...await treeInputs(uiDir), ...await treeInputs(path.join(rootDir, "sdk", "vue")), ...(hasPrepareUI ? await treeInputs(path.join(plugin.path, ".rayleabot")) : []), ...scriptInputs],
        identity: toolIdentity, outputs: [path.join(uiDir, "dist"), path.join(uiDir, "dist", "index.html")],
        build: () => runCommand("构建插件 UI " + plugin.id, "pnpm", ["build"], { cwd: uiDir }),
      });
    }
    const binary = plugin.hasGoModule
      ? path.join(cacheDir, "plugins", plugin.id, plugin.id + nativeExecutableSuffix(platform))
      : path.join(plugin.path, "dist", "native", platform, plugin.id + nativeExecutableSuffix(platform));
    let backend = ".";
    if (plugin.hasGoModule) {
      backend = JSON.parse(await captureCommand(helper, ["inspect", "--plugin", plugin.path])).backend_package;
      if (!backend) throw new Error("开发插件缺少 Go 入口：" + plugin.id);
      await cachedGoBuild(plugin.id + "-backend", {
        cwd: plugin.path, main: backend, output: binary, env: environment,
        flags: ["-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid="],
      });
    }
    const artifact = path.join(pluginDevArtifactRoot, platform, plugin.id);
    const resourceInputs = async () => {
      const files = [binary, helper, path.join(plugin.path, "info.json"), path.join(plugin.path, "go.mod"), path.join(plugin.path, "go.sum"), ...scriptInputs];
      for (const name of ["assets", "templates", "LICENSES"]) files.push(...await treeInputs(path.join(plugin.path, name), { all: true }));
      for (const name of await fsp.readdir(plugin.path)) if (/^(LICENSE|COPYING|NOTICE|THIRD_PARTY_NOTICES)/.test(name)) {
        const file = path.join(plugin.path, name);
        if ((await fsp.stat(file)).isFile()) files.push(file);
      }
      for (let directory = plugin.path; ; directory = path.dirname(directory)) {
        files.push(path.join(directory, "LICENSE"));
        if (directory === path.dirname(directory)) break;
      }
      if (hasUI) files.push(...await treeInputs(path.join(uiDir, "dist"), { all: true }), path.join(uiDir, "pnpm-lock.yaml"));
      if (plugin.hasGoModule) files.push(...await goInputs(plugin.path, backend, environment));
      return files;
    };
    await buildCache.run(plugin.id + "-artifact", {
      inputs: resourceInputs, identity: { ...toolIdentity, source: await fsp.realpath(plugin.path), platform }, outputs: [artifact],
      build: () => runCommand("组装开发插件 " + plugin.id, helper, [
        plugin.hasGoModule ? "build-go" : "pack", "--plugin", plugin.path,
        ...(plugin.hasGoModule ? ["--backend", backend, "--backend-binary", binary, "--skip-ui-build"] : ["--binary", binary]),
        "--target", platform, "--out", pluginDevArtifactRoot, "--expanded=true", "--archive=false",
      ], { cwd: rootDir, env: environment }),
    });
  }, { onFailure: reportDevelopmentPluginFailure, onProgress: (completed, total) => progress.progress(completed, total) });
  return { workspace, platform, plugins: result.completed.map(({ plugin }) => plugin), failed: result.failed };
}

async function installDevelopmentPlugins(preparedPlugins, serverBinaryPath) {
  if (!preparedPlugins.plugins.length) return { completed: [], failed: [] };
  return terminal.phase("同步开发插件", (progress) => installPreparedPlugins(preparedPlugins, serverBinaryPath, progress));
}

async function installPreparedPlugins(preparedPlugins, serverBinaryPath, progress) {
  const configPath = path.join(rootDir, "config", "user.yaml");
  if (preparedPlugins.plugins.length && !fs.existsSync(configPath)) {
    await runCommand("初始化配置", serverBinaryPath, ["-config", configPath, "config", "init"], { cwd: rootDir });
  }
  const result = await runDevelopmentPluginBatch(preparedPlugins.plugins, async (plugin) => {
    assertDevelopmentRunning();
    const expandedArtifact = path.join(pluginDevArtifactRoot, preparedPlugins.platform, plugin.id);
    await runCommand(`同步开发插件 ${plugin.id}`, serverBinaryPath, [
      "-config",
      configPath,
      "plugin",
      "dev-sync",
      "--artifact",
      expandedArtifact,
      "--source",
      plugin.path,
    ], { cwd: rootDir });
  }, { onFailure: reportDevelopmentPluginFailure, onProgress: (completed, total) => progress.progress(completed, total) });
  log(`开发插件：${result.completed.length} 个已同步${result.failed.length ? `，${result.failed.length} 个失败` : ""}`);
  return result;
}

async function syncDevelopmentPlugins(pluginDev, serverBinaryPath, pluginIDs) {
  const preparedPlugins = await buildDevelopmentPlugins(pluginDev, pluginIDs);
  await installDevelopmentPlugins(preparedPlugins, serverBinaryPath);
  return preparedPlugins.workspace;
}

async function synchronizeOnlinePlugins(prepared, backendBaseUrl) {
  if (!prepared.plugins.length) return { completed: [], failed: [] };
  return terminal.phase("在线同步开发插件", async (progress) => {
    const result = await runDevelopmentPluginBatch(prepared.plugins, async (plugin) => {
      assertDevelopmentRunning();
      const changed = await synchronizeDevelopmentPlugin({
        baseURL: backendBaseUrl, token: developmentControlToken,
        artifact: path.join(pluginDevArtifactRoot, prepared.platform, plugin.id), source: plugin.path,
      });
      log(`${plugin.id}: ${changed ? "已在线同步" : "安装内容未变化"}`, "debug");
      return changed;
    }, { onFailure: reportDevelopmentPluginFailure, onProgress: (completed, total) => progress.progress(completed, total) });
    const updated = result.completed.filter(({ value }) => value).length;
    if (updated) log(`开发插件：${updated} 个已更新，${result.completed.length - updated} 个内容未变化`);
    return result;
  });
}

async function ensureServerRuntime({ serverReloadMode, backendBaseUrl, pluginDev, serverDevEnvironment }) {
  if (serverReloadMode === SERVER_RELOAD_WATCH) {
    await startServerWatch(backendBaseUrl, pluginDev, serverDevEnvironment);
    return;
  }
  await buildServer();
  await syncDevelopmentPlugins(pluginDev, path.join(serverDistDir, serverBinaryName));
}

async function startServerWatch(backendBaseUrl, pluginDev, serverDevEnvironment) {
  let lease;
  let child;
  let timer;
  let rebuilding = true;
  let reloadPromise;
  let pluginWorkspace = await readDevelopmentWorkspace(pluginDev);
  let stopPluginWatching = async () => {};
  const queue = createDevelopmentReloadQueue();
  const expectedExits = new Set();
  const reportError = (error) => {
    if (shuttingDown) return;
    if (error.code === "DEV_INPUT_CHANGED") log("构建期间检测到新改动，将合并后重试。", "debug");
    else log(error.message, "error");
  };
  const schedule = () => {
    clearTimeout(timer);
    if (!shuttingDown) timer = setTimeout(() => { reloadPromise = reconcile(); }, serverReloadDebounceMs);
  };
  const queueServer = (file) => { queue.addServer(file); if (!rebuilding) schedule(); };
  const queueWorkspace = (file) => { queue.addWorkspace(file); if (!rebuilding) schedule(); };
  const queuePlugin = (plugin, file) => { queue.addPlugin(plugin, file); if (!rebuilding) schedule(); };
  const queueAllPlugins = (file, predicate = () => true) => {
    if (pluginDev.mode === PLUGIN_DEV_WATCH) for (const plugin of pluginWorkspace.plugins.filter(predicate)) queuePlugin(plugin, file);
  };
  const rememberFailures = (failed, batch) => {
    for (const { plugin, error } of failed) {
      const source = batch?.pluginChanges.find((entry) => entry.plugin.id === plugin.id)?.sourcePath || plugin.path;
      if (error.code === "DEV_INPUT_CHANGED") queuePlugin(plugin, source);
      else queue.deferPlugin(plugin, source);
    }
  };
  const acceptPluginResults = (result, batch) => {
    for (const { plugin } of result.completed) queue.completePlugin(plugin.id);
    rememberFailures(result.failed, batch);
  };
  const graphWatchers = new Map();
  onGoInputsChanged = () => {
    const excluded = [toolIdentity.go.GOROOT, toolIdentity.go.GOMODCACHE, launcherDir, serverDir, path.join(rootDir, "sdk"), ...pluginWorkspace.plugins.flatMap((plugin) => [plugin.path, ...(plugin.goModules ?? [])])].filter(Boolean);
    const directories = new Set();
    for (const file of activeGoInputs) {
      if (excluded.some((root) => file === root || file.startsWith(root + path.sep))) continue;
      const directory = path.dirname(file);
      if (fs.existsSync(directory)) directories.add(directory);
    }
    for (const [directory, watcher] of graphWatchers) {
      if (!directories.has(directory)) { watcher.close(); graphWatchers.delete(directory); }
    }
    for (const directory of directories) {
      if (graphWatchers.has(directory)) continue;
      const watcher = fs.watch(directory, (event, filename) => {
        if (!filename) return;
        const changed = path.join(directory, filename.toString());
        const owners = new Set(goInputRegistry.ownersForWatchEvent(changed, event));
        if (owners.has(serverDir)) queueServer(changed);
        if (owners.has(path.join(rootDir, "sdk", "go"))) queueAllPlugins(changed, (plugin) => plugin.hasGoModule);
        else if (pluginDev.mode === PLUGIN_DEV_WATCH) for (const plugin of pluginWorkspace.plugins) {
          if (owners.has(plugin.path)) queuePlugin(plugin, changed);
        }
      });
      watcher.on("error", reportError);
      graphWatchers.set(directory, watcher);
    }
  };
  const refreshWorkspaceWatch = async () => {
    const workspace = await readDevelopmentWorkspace(pluginDev);
    const sharedModules = [...new Set(workspace.plugins.flatMap((plugin) => plugin.goModules ?? []))]
      .filter((modulePath) => modulePath !== path.join(rootDir, "sdk", "go"))
      .map((modulePath) => ({ id: "module:" + modulePath, path: modulePath, hasGoModule: true }));
    const queueModuleUsers = (module, file) => {
      for (const plugin of workspace.plugins.filter((item) => item.goModules?.includes(module.path))) queuePlugin(plugin, file);
    };
    const newStop = pluginDev.mode === PLUGIN_DEV_WATCH ? await watchPluginWorkspace(workspace.plugins, queuePlugin, reportError, (file) => activeGoInputs.has(file), { isolateErrors: true }) : async () => {};
    const stopModules = pluginDev.mode === PLUGIN_DEV_WATCH ? await watchPluginWorkspace(sharedModules, queueModuleUsers, reportError, (file) => activeGoInputs.has(file), { isolateErrors: true }) : async () => {};
    const stopInvalid = pluginDev.mode === PLUGIN_DEV_WATCH ? await watchPluginWorkspace(
      (workspace.invalidPlugins || []).map(({ path: source }) => ({ id: source, path: source, hasGoModule: fs.existsSync(path.join(source, "go.mod")) })),
      (_plugin, file) => queueWorkspace(file), reportError, () => false, { isolateErrors: true },
    ) : async () => {};
    await stopPluginWatching();
    stopPluginWatching = async () => { await newStop(); await stopModules(); await stopInvalid(); };
    pluginWorkspace = workspace;
    goInputRegistry.retainRoots([serverDir, launcherDir, path.join(rootDir, "sdk", "go"), ...workspace.plugins.filter((plugin) => plugin.hasGoModule).flatMap((plugin) => [plugin.path, ...(plugin.goModules ?? [])])]);
    onGoInputsChanged();
  };
  const stopServerWatch = await watchServerSources((file) => {
    queueServer(file);
    if (file.startsWith(path.join(rootDir, "sdk", "go") + path.sep)) queueAllPlugins(file, (plugin) => plugin.hasGoModule);
  });
  const stopVueWatch = await watchPluginWorkspace([{ id: "vue-sdk", path: path.join(rootDir, "sdk", "vue"), hasGoModule: true }], (_plugin, file) => queueAllPlugins(file, (plugin) => fs.existsSync(path.join(plugin.path, "ui", "package.json"))), reportError);
  const rootWatcher = fs.watch(rootDir, (_event, name) => {
    if (!name) return;
    const file = path.join(rootDir, name.toString());
    if (file === pluginDev.workspacePath && pluginDev.mode === PLUGIN_DEV_WATCH) {
      queueWorkspace(file);
    } else if (["go.work", "go.work.sum", "go.mod", "go.sum"].includes(name.toString())) {
      queueServer(file);
      queueAllPlugins(file);
    }
  });
  const workspaceWatcher = pluginDev.mode === PLUGIN_DEV_WATCH && path.dirname(pluginDev.workspacePath) !== rootDir && fs.existsSync(path.dirname(pluginDev.workspacePath))
    ? fs.watch(path.dirname(pluginDev.workspacePath), (_event, name) => {
      if (name?.toString() === path.basename(pluginDev.workspacePath)) queueWorkspace(pluginDev.workspacePath);
    }) : null;
  await refreshWorkspaceWatch();

  const monitor = (target) => target.once("exit", (code, signal) => {
    if (expectedExits.delete(target.pid) || shuttingDown) return;
    log("Server 已停止，正在结束开发启动流程。");
    void shutdown(normalizeExitCode(code, signal));
  });
  const stopServer = async (target) => {
    if (!target || target.exitCode !== null || target.signalCode !== null) return;
    expectedExits.add(target.pid);
    try {
      await requestDevelopmentServerShutdown({ lease });
      await waitForChildExit(target, 20_000);
    } catch (error) {
      log("Server 优雅退出未完成，结束当前受管进程。", "error");
      await terminateChild(target);
    }
  };
  const synchronize = (prepared) => synchronizeOnlinePlugins(prepared, backendBaseUrl);
  const startAndVerify = async () => {
    child = startServerDevProcess(serverDevEnvironment);
    await waitForServerProcess(child, backendBaseUrl, null);
    monitor(child);
    await fsp.copyFile(serverDevBinaryPath, path.join(cacheDir, "server-last-good" + nativeExecutableSuffix(currentPluginPlatform())));
  };
  cleanupCallbacks.add(async () => {
    clearTimeout(timer);
    rootWatcher.close();
    workspaceWatcher?.close();
    onGoInputsChanged = () => {};
    for (const watcher of graphWatchers.values()) watcher.close();
    await stopServerWatch();
    await stopVueWatch();
    await stopPluginWatching();
    // Reconciliation owns its candidate until it completes. Stop its build children first.
    if (reloadPromise && rebuilding) {
      await Promise.allSettled([...longRunningChildren].filter((target) => target !== child).map(terminateChild));
      await reloadPromise;
    }
    await stopServer(child);
  });

  async function reconcile() {
    if (rebuilding || !queue.hasChanges() || shuttingDown) return;
    rebuilding = true;
    const batch = queue.take();
    log(`开发变更：${JSON.stringify({ server: batch.serverSourcePath, workspace: batch.workspaceSourcePath, plugins: batch.pluginChanges.map(({ plugin, sourcePath }) => ({ id: plugin.id, path: sourcePath })) })}`, "debug");
    let stopped = false;
    let replaced = false;
    let refresh = false;
    const started = Date.now();
    try {
      refresh = Boolean(batch.workspaceSourcePath);
      if (refresh) await refreshWorkspaceWatch();
      let serverChanged = false;
      if (batch.serverSourcePath) {
        try { serverChanged = await buildServerDevBinaryAt(serverDevCandidateBinaryPath); }
        catch (error) {
          if (error.code === "DEV_SHUTDOWN") throw error;
          reportError(error);
          if (error.code === "DEV_INPUT_CHANGED") queueServer(batch.serverSourcePath);
          else queue.deferServer(batch.serverSourcePath);
        }
      }
      const prepared = batch.pluginChanges.length || refresh
        ? await buildDevelopmentPlugins(pluginDev, refresh ? undefined : batch.pluginChanges.map(({ plugin }) => plugin.id)) : null;
      if (shuttingDown) return;
      if (prepared) rememberFailures(prepared.failed, batch);
      // A newer edit supersedes only its owner. One busy or broken plugin must
      // not prevent already-built peers from reaching the running Server.
      if (queue.hasWorkspaceChanges()) {
        if (batch.serverSourcePath) queue.addServer(batch.serverSourcePath);
        for (const entry of batch.pluginChanges) queue.addPlugin(entry.plugin, entry.sourcePath);
        return;
      }
      if (queue.hasServerChanges()) serverChanged = false;
      if (prepared) prepared.plugins = currentDevelopmentPlugins(prepared.plugins, queue);
      if (serverChanged) {
        await stopServer(child);
        stopped = true;
        if (prepared) acceptPluginResults(await installDevelopmentPlugins(prepared, serverDevCandidateBinaryPath), batch);
        await replaceServerDevBinary(serverDevCandidateBinaryPath);
        replaced = true;
        await startAndVerify();
        stopped = false;
        await fsp.rm(serverDevPreviousBinaryPath, { force: true });
        queue.retryDeferredPlugins();
      }
      let pluginsChanged = 0;
      if (prepared && !serverChanged) {
        const result = await synchronize(prepared);
        acceptPluginResults(result, batch);
        pluginsChanged = result.completed.filter(({ value }) => value).length;
      }
      log(`开发同步完成：Server ${serverChanged ? "已重启" : "保持运行"}，耗时 ${Date.now() - started} ms。`, serverChanged || pluginsChanged ? "info" : "debug");
    } catch (error) {
      reportError(error);
      if (error.code === "DEV_INPUT_CHANGED") {
        if (refresh) queue.addWorkspace(batch.workspaceSourcePath);
        if (batch.serverSourcePath) queue.addServer(batch.serverSourcePath);
        for (const entry of batch.pluginChanges) queue.addPlugin(entry.plugin, entry.sourcePath);
      } else if (error.code !== "DEV_SHUTDOWN") queue.deferBatch(batch);
      if (stopped && !shuttingDown) {
        try {
          await stopServer(child);
          if (replaced) {
            await fsp.rm(serverDevBinaryPath, { force: true });
            await fsp.rename(serverDevPreviousBinaryPath, serverDevBinaryPath);
          }
          await startAndVerify();
          log("Server 已恢复上一个通过健康检查的版本。");
        } catch (recoveryError) { reportError(recoveryError); }
      }
    } finally {
      rebuilding = false;
      if (queue.hasChanges()) schedule();
    }
  }

  try {
    for (;;) {
      try { await buildServerDevBinary(); break; }
      catch (error) {
        if (error.code === "DEV_INPUT_CHANGED" && !shuttingDown) continue;
        if (await isServerHealthy(backendBaseUrl)) throw error;
        const previous = path.join(cacheDir, "server-last-good" + nativeExecutableSuffix(currentPluginPlatform()));
        if (!fs.existsSync(previous) || shuttingDown) throw error;
        await fsp.copyFile(previous, serverDevBinaryPath);
        log("Server 构建失败，使用最近通过健康检查的开发二进制。", "error");
        reportError(error);
        break;
      }
    }
    await startPreparedDevelopmentRuntime({
      prepare: async () => {
        const prepared = await prepareStableDevelopmentPlugins((ids) => buildDevelopmentPlugins(pluginDev, ids));
        rememberFailures(prepared.failed);
        return prepared;
      },
      existingIsHealthy: () => isServerHealthy(backendBaseUrl),
      acquire: async () => {
        assertDevelopmentRunning();
        lease = await acquireDevelopmentServerLease(backendBaseUrl);
        activeServerDevLeaseId = lease.lease_id;
      },
      // Offline installation owns the database only after the old Server exits.
      install: async (prepared) => {
        const result = await installDevelopmentPlugins(prepared, serverDevBinaryPath);
        acceptPluginResults(result);
        return result;
      },
      start: async () => {
        try { await startAndVerify(); }
        catch (error) {
          const previous = path.join(cacheDir, "server-last-good" + nativeExecutableSuffix(currentPluginPlatform()));
          if (!fs.existsSync(previous) || shuttingDown) throw error;
          await stopServer(child);
          await fsp.copyFile(previous, serverDevBinaryPath);
          await startAndVerify();
          log("Server 候选启动失败，已恢复上一个健康版本。", "error");
        }
      },
    });
    log("Server 与开发插件已就绪。", "debug");
  } finally {
    rebuilding = false;
    if (queue.hasChanges()) schedule();
  }
}

async function reuseDevelopmentRuntime(backendBaseUrl) {
  let lease = await readDevelopmentServerLease();
  if (!lease || !isProcessRunning(lease.owner_pid)) return false;
  if (process.env.RAYLEA_START_RESTART === "1" || lease.startup_identity !== startupIdentity || lease.backend_base_url !== backendBaseUrl) return false;
  lease = await waitForDevelopmentServerLease({ lease, readLease: readDevelopmentServerLease });
  if (!lease) return false;
  if (await classifyWebDevServer({ backendBaseUrl, projectDir: webDir }) !== "rayleabot") throw new Error("已有环境的 Web 开发服务不可用；设置 RAYLEA_START_RESTART=1 后重启。");
  const status = await developmentRequest(backendBaseUrl, lease.control_token, "api/development/status", undefined, { timeoutMs: 2000 });
  if (path.resolve(status.artifact_root) !== await fsp.realpath(pluginDevArtifactRoot)) throw new Error("开发 Server 不属于当前工作区。");
  developmentControlToken = lease.control_token;
  developmentControlEnvironment[LAUNCHER_CONTROL_TOKEN_ENV] = lease.control_token;
  Object.assign(developmentServerWatcherEnvironment, createDevelopmentServerWatcherEnvironment({ ownerPid: lease.owner_pid }));
  reusedRuntime = true;
  log("复用当前健康开发环境，打开 Launcher。");
  return true;
}

async function markRuntimeReady() {
  if (!activeServerDevLeaseId || reusedRuntime) return;
  const lease = await readDevelopmentServerLease();
  if (lease?.lease_id !== activeServerDevLeaseId) throw new Error("开发运行时租约已变化。");
  lease.ready = true;
  const temporary = serverDevLeasePath + "." + process.pid;
  await fsp.writeFile(temporary, JSON.stringify(lease), { mode: 0o600 });
  await fsp.rename(temporary, serverDevLeasePath);
}

async function acquireDevelopmentServerLease(backendBaseUrl) {
  await fsp.mkdir(path.dirname(serverDevLeasePath), { recursive: true });
  for (let attempt = 0; attempt < 3; attempt += 1) {
    const existingLease = await readDevelopmentServerLease();
    if (existingLease) {
      if (process.env.RAYLEA_START_RESTART !== "1" && existingLease.startup_identity === startupIdentity && isProcessRunning(existingLease.owner_pid)) {
        throw new Error("同一工作区已有开发流程正在启动或重载，请稍后重新打开。");
      }
      await retireDevelopmentServerLease(existingLease);
    } else if (await isServerHealthy(backendBaseUrl)) {
      throw new Error(
        "检测到未由当前开发 watcher 管理的 Server。请先通过 Launcher 停止现有服务，再重新启动。",
      );
    }

    const lease = createDevelopmentServerLease({
      ownerPid: process.pid,
      rootDir,
      backendBaseUrl,
      binaryPath: serverDevBinaryPath,
      controlToken: developmentControlToken,
    });
    lease.startup_identity = startupIdentity;
    lease.ready = false;
    try {
      await fsp.writeFile(serverDevLeasePath, `${JSON.stringify(lease, null, 2)}\n`, {
        encoding: "utf8",
        flag: "wx",
        mode: 0o600,
      });
      return lease;
    } catch (error) {
      if (error?.code !== "EEXIST") {
        throw error;
      }
    }
  }
  throw new Error("另一个开发启动流程正在接管 Server，请稍后重试。");
}

async function retireDevelopmentServerLease(lease) {
  const takeoverDeadline = Date.now() + serverDevTakeoverTimeoutMs;
  let ownerRunning = isProcessRunning(lease.owner_pid);
  let serverHealthy = await isServerHealthy(lease.backend_base_url);
  if (ownerRunning && !serverHealthy) {
    log("检测到另一个开发启动流程正在准备 Server，等待其进入可接管状态。");
    while (ownerRunning && !serverHealthy && Date.now() < takeoverDeadline) {
      await delay(250);
      ownerRunning = isProcessRunning(lease.owner_pid);
      serverHealthy = await isServerHealthy(lease.backend_base_url);
    }
  }

  if (serverHealthy) {
    log("检测到上一次开发启动流程，正在平滑停止旧 Server。");
    try {
      await requestDevelopmentServerShutdown({ lease });
    } catch (error) {
      if (await isServerHealthy(lease.backend_base_url)) {
        throw new Error(`无法停止上一次开发 Server：${error?.message ?? error}`);
      }
    }
  }

  while (Date.now() < takeoverDeadline) {
    ownerRunning = isProcessRunning(lease.owner_pid);
    serverHealthy = await isServerHealthy(lease.backend_base_url);
    if (!ownerRunning && !serverHealthy) {
      await removeDevelopmentServerLeaseIfOwned(lease.lease_id);
      await fsp.rm(lease.binary_path, { force: true });
      return;
    }
    await delay(250);
  }
  throw new Error("上一次开发启动流程未在 30 秒内退出，请关闭旧启动窗口后重试。");
}

async function readDevelopmentServerLease() {
  try {
    const text = await fsp.readFile(serverDevLeasePath, "utf8");
    return parseDevelopmentServerLease(text, { rootDir, serverTmpDir });
  } catch (error) {
    if (error?.code === "ENOENT") {
      return null;
    }
    throw new Error(`读取开发 Server 租约失败：${error?.message ?? error}`);
  }
}

async function removeDevelopmentServerLeaseIfOwned(leaseId) {
  const currentLease = await readDevelopmentServerLease();
  if (currentLease?.lease_id === leaseId) {
    await fsp.rm(serverDevLeasePath, { force: true });
  }
}

async function releaseActiveDevelopmentServerLease() {
  const leaseId = activeServerDevLeaseId;
  activeServerDevLeaseId = "";
  if (!leaseId) {
    return;
  }
  await removeDevelopmentServerLeaseIfOwned(leaseId);
}

async function buildServerDevBinary() {
  await buildServerDevBinaryAt(serverDevBinaryPath);
}

async function buildServerDevBinaryAt(outputPath) {
  await fsp.mkdir(serverTmpDir, { recursive: true });
  const cached = path.join(cacheDir, serverBinaryName);
  await cachedGoBuild("server", { cwd: serverDir, main: "./cmd/raylea-server", output: cached });
  let changed = true;
  try { changed = !(await fsp.readFile(cached)).equals(await fsp.readFile(serverDevBinaryPath)); }
  catch (error) { if (error.code !== "ENOENT") throw error; }
  if (outputPath !== serverDevBinaryPath || changed) await fsp.copyFile(cached, outputPath);
  return changed;
}

async function replaceServerDevBinary(candidatePath) {
  await fsp.rm(serverDevPreviousBinaryPath, { force: true });
  await fsp.rename(serverDevBinaryPath, serverDevPreviousBinaryPath);
  try {
    await fsp.rename(candidatePath, serverDevBinaryPath);
  } catch (error) {
    await fsp.rename(serverDevPreviousBinaryPath, serverDevBinaryPath).catch(() => undefined);
    throw error;
  }
}

function startServerDevProcess(serverDevEnvironment) {
  return spawnManaged(serverDevBinaryPath, [
    "-config",
    "../config/user.yaml",
    "-config-schema",
    "../contracts/config.user.schema.json",
  ], {
    cwd: serverDir,
    env: serverDevEnvironment,
    logType: "server",
  });
}

async function waitForServerProcess(child, backendBaseUrl, readyMessage) {
  const readiness = childReadinessWatches.get(child);
  let reportedPreparation = false;
  while (!readiness.hasExpired()) {
    if (child.exitCode !== null || child.signalCode !== null) {
      throw new Error("Server 热重载进程已退出。");
    }
    try {
      if (await isServerHealthy(backendBaseUrl)) {
        await developmentRequest(backendBaseUrl, developmentControlToken, "api/development/status", undefined, { timeoutMs: 2000 });
        if (readyMessage) {
          log(readyMessage);
        }
        return;
      }
    } catch (error) {
      if (error?.code !== "ENOENT") {
        throw error;
      }
    }
    if (readiness.isPreparing() && !reportedPreparation) {
      reportedPreparation = true;
      log("Server 正在准备运行环境，完成后继续启动。");
    }
    await delay(500);
  }
  const reason = readiness.isPreparing()
    ? "Server 运行环境准备超时"
    : "Server 未在 30 秒内就绪";
  throw new Error(`${reason}，日志见 ${relativePath(developmentLogs.server.path)}。`);
}

async function watchServerSources(onChange) {
  return watchPluginWorkspace([
    { id: "server", path: serverDir, hasGoModule: true, ignoredDirectories: ["tmp", ".tmp", ".cache", ".gocache", "logs"] },
    { id: "sdk", path: path.join(rootDir, "sdk", "go"), hasGoModule: true },
  ], (_plugin, source) => onChange(source), (error) => log(error.message, "error"), (file) => activeGoInputs.has(file));
}

async function isServerHealthy(backendBaseUrl) {
  try {
    const response = await fetchWithTimeout(new URL("healthz", ensureTrailingSlash(backendBaseUrl)).toString(), 800);
    return response.ok;
  } catch {
    return false;
  }
}

async function fetchWithTimeout(url, timeoutMs) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(url, { signal: controller.signal });
  } finally {
    clearTimeout(timeout);
  }
}

function ensureTrailingSlash(value) {
  return value.endsWith("/") ? value : `${value}/`;
}

async function buildLauncherApp() {
  const bindings = path.join(launcherDir, "src", "renderer", "bindings");
  await buildCache.run("launcher-bindings", {
    inputs: async () => [...(await goInputs(launcherDir, ".", { GOWORK: "off" }, process.platform === "linux" ? ["-tags", "gtk3"] : [])).filter((file) => file.endsWith(".go") || /go\.(mod|sum)$/.test(file)), ...await treeInputs(path.join(launcherDir, "scripts")), ...scriptInputs],
    identity: toolIdentity, outputs: [bindings],
    build: () => runCommand("生成 Launcher bindings", "pnpm", ["run", "generate:wails"], { cwd: launcherDir, env: createLauncherToolEnvironment() }),
  });
  await buildCache.run("launcher-ui", {
    inputs: async () => [...await treeInputs(path.join(launcherDir, "src")), ...dependencyInputs(launcherDir), path.join(launcherDir, "vite.config.ts"), ...await treeInputs(path.join(launcherDir, "scripts")), ...scriptInputs],
    identity: toolIdentity, outputs: [path.join(launcherDir, "internal", "frontend", "dist", "index.html"), path.join(launcherDir, "internal", "frontend", "dist")],
    build: () => runCommand("构建 Launcher UI", process.execPath, [path.join(launcherDir, "node_modules", "vite", "bin", "vite.js"), "build"], { cwd: launcherDir }),
  });
}

async function prepareLauncher(installMode) {
  await terminal.phase("Launcher 构建", async () => {
    await ensureDependencies("Launcher", launcherDir, installMode);
    if (process.platform === "win32") {
      await runCommand("生成 Launcher Windows 图标资源", process.execPath, ["scripts/generate-windows-resources.mjs"], {
        cwd: launcherDir, env: createLauncherToolEnvironment(),
      });
    }
    await buildLauncherApp();
    if (!shouldSkipLaunch()) {
      const output = path.join(cacheDir, "raylea-launcher" + (process.platform === "win32" ? ".exe" : ""));
      await cachedGoBuild("launcher", { cwd: launcherDir, main: ".", output, env: { GOWORK: "off" }, flags: process.platform === "linux" ? ["-tags", "gtk3"] : [] });
    }
  });
}

async function launchCachedLauncher(environment) {
  const output = path.join(cacheDir, "raylea-launcher" + (process.platform === "win32" ? ".exe" : ""));
  await markRuntimeReady();
  const executable = path.join(cacheDir, `launcher-run-${process.pid}` + (process.platform === "win32" ? ".exe" : ""));
  await fsp.copyFile(output, executable);
  try {
    await runCommand("启动 Launcher", executable, [], {
      cwd: launcherDir, env: { ...environment, ...developmentControlEnvironment, ...(activeServerDevLeaseId || reusedRuntime ? developmentServerWatcherEnvironment : {}), GOWORK: "off" }, logType: "launcher",
      // SW_HIDE overrides the first ShowWindow call, leaving the interactive UI invisible.
      windowsHide: false,
    });
  } finally { await removeFileWithRetry(executable); }
}

async function captureCommand(command, args, { cwd = rootDir, env = {} } = {}) {
  assertDevelopmentRunning();
  const spec = createSpawnSpec(command, args);
  const child = spawn(spec.command, spec.args, { cwd, env: createChildEnvironment(env), windowsHide: true, stdio: ["ignore", "pipe", "pipe"] });
  longRunningChildren.add(child);
  let output = "", errors = "";
  child.stdout.on("data", (chunk) => { output += chunk; });
  child.stderr.on("data", (chunk) => { errors += chunk; });
  try {
    const exit = await waitForChild(child);
    assertDevelopmentRunning();
    if (exit.code !== 0) throw new Error(`${command} failed: ${errors.trim()}`);
    return output.trim();
  } finally { longRunningChildren.delete(child); }
}

async function goInputs(cwd, main, env = {}, flags = []) {
  const output = await captureCommand("go", ["list", ...flags, "-deps", "-f", goInputTemplate, main], { cwd, env });
  const workspace = env.GOWORK === "off" ? [] : [env.GOWORK || path.join(rootDir, "go.work"), (env.GOWORK || path.join(rootDir, "go.work")) + ".sum"];
  const inputs = [...parseGoInputs(output), ...workspace];
  goInputRegistry.update(cwd, inputs);
  onGoInputsChanged();
  return [...inputs, ...scriptInputs];
}

async function cachedGoBuild(name, { cwd, main, output, env = {}, flags = [] }) {
  if (!flags.some((flag) => flag.startsWith("-buildvcs="))) flags = ["-buildvcs=false", ...flags];
  env = { ...env, GOOS: { win32: "windows", darwin: "darwin", linux: "linux" }[process.platform], GOARCH: { x64: "amd64", arm64: "arm64" }[process.arch] };
  await fsp.mkdir(path.dirname(output), { recursive: true });
  const listFlags = flags.includes("-tags") ? flags.slice(flags.indexOf("-tags"), flags.indexOf("-tags") + 2) : [];
  return buildCache.run(name, {
    inputs: () => goInputs(cwd, main, env, listFlags), identity: { ...toolIdentity, env, flags }, outputs: [output],
    build: () => runCommand("构建 " + name, "go", ["build", ...flags, "-o", output, main], { cwd, env }),
  });
}

function dependencyInputs(projectDir) {
  return ["package.json", "pnpm-lock.yaml", "pnpm-workspace.yaml", ".npmrc"].map((name) => path.join(projectDir, name));
}

function createLauncherToolEnvironment(environment = {}) {
  return {
    ...environment,
    RAYLEA_GO_EXECUTABLE: resolveGoExecutablePath(),
  };
}

async function ensureDependencies(label, projectDir, installMode, extraInputs = []) {
  if (installMode === "skip") return;
  const identity = { ...toolIdentity, platformOnly: true, ...(installMode === "always" ? { force: Date.now() } : {}) };
  const name = "deps-" + (await fingerprint([], { projectDir })).slice(0, 16);
  const install = () => runCommand("安装 " + label + " 依赖", "pnpm", [
    "install", "--frozen-lockfile", "--os=" + process.platform, "--cpu=" + process.arch,
    ...(process.platform === "linux" ? ["--libc=" + (process.report.getReport().header.glibcVersionRuntime ? "glibc" : "musl")] : []),
  ], { cwd: projectDir, env: createDependencyInstallEnvironment() });
  await ensureDependencyInstall({
    projectDir, cache: buildCache, name,
    inputs: async () => [...dependencyInputs(projectDir), ...extraInputs], identity,
    install,
    onRepair: () => log(`检测到 ${label} 依赖安装不完整，正在重新安装。`, "warn"),
  });
}

async function ensureWebDevServer(devEnvironment) {
  const state = await classifyWebDevServer({ backendBaseUrl: devEnvironment.VITE_BACKEND_TARGET, projectDir: webDir });
  if (state === "rayleabot") {
    log("已复用现有 Web 开发服务");
    return;
  }
  if (state === "occupied") {
    throw new Error(`端口 ${WEB_DEV_PORT} 已被其他程序占用。请关闭占用程序，或使用 RAYLEA_START_PROFILE=build。`);
  }

  log(`启动 Web 开发服务器：${WEB_DEV_BASE_URL}`, "debug");
  const child = spawnManaged("pnpm", ["dev"], {
    cwd: webDir,
    env: devEnvironment,
    logType: "web",
  });

  await waitForWebDevServer(child, devEnvironment.VITE_BACKEND_TARGET);
}

async function waitForWebDevServer(child, backendBaseUrl) {
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    if (child.exitCode !== null || child.signalCode !== null) {
      throw new Error("Web 开发服务器已退出。");
    }
    const state = await classifyWebDevServer({ backendBaseUrl, projectDir: webDir, timeoutMs: 800 });
    if (state === "rayleabot") {
      log(`Web 开发服务器已就绪：${WEB_DEV_BASE_URL}`, "debug");
      return;
    }
    await delay(500);
  }
  throw new Error(`Web 开发服务器未在 30 秒内就绪，日志见 ${relativePath(developmentLogs.web.path)}。`);
}

async function runCommand(label, command, args, { cwd, env = {}, logType, windowsHide = true } = {}) {
  assertDevelopmentRunning();
  const task = terminal.start(label);
  let child;
  try {
    child = spawnManaged(command, args, { cwd, env, logType, windowsHide });
    if (logType === "launcher") child.once("spawn", () => {
      if (shuttingDown) return;
      task.finish("进程已启动");
      reportStartup();
    });
    const exit = await waitForChild(child);
    assertDevelopmentRunning();
    if (exit.code !== 0) {
      const output = childOutputTails.get(child)?.() ?? "";
      const hints = describeCommandFailure(output, { cwd: cwd ?? rootDir });
      const detail = hints.map((hint) => `提示：${hint}`).join("\n");
      const hidden = childHiddenOutputTails.get(child)?.().trim().split("\n").slice(-12).join("\n");
      if (hidden) terminal.write(hidden, { scope: logType || "build", level: "error", persist: false });
      throw new Error(`${label}失败，退出码 ${exit.code}。日志：${relativePath(developmentLogs[logType || "build"].path)}${detail ? `\n${detail}` : ""}`);
    }
    task.finish();
  } catch (error) {
    if (shuttingDown) task.cancel();
    else task.fail();
    throw error;
  }
}

function spawnManaged(command, args, { cwd, env = {}, logType = "build", windowsHide = true } = {}) {
  assertDevelopmentRunning();
  const commandText = [command, ...args].join(" ");
  const childLog = developmentLogs[logType];
  writeStartLog(`$ ${commandText} → ${relativePath(childLog.path)}\n`);
  childLog.write(`\n[${formatUTCLogTimestamp()}] $ ${redactLogLine(commandText)}\n`);
  const spawnSpec = createSpawnSpec(command, args);
  const childOverrides = command === "pnpm"
    ? createDependencyInstallEnvironment(env)
    : env;
  const child = spawn(spawnSpec.command, spawnSpec.args, {
    cwd,
    env: createChildEnvironment(childOverrides),
    windowsHide,
    stdio: ["ignore", "pipe", "pipe"],
  });

  const readiness = logType === "server" ? createServerReadinessWatch() : null;
  if (readiness) childReadinessWatches.set(child, readiness);
  const output = createDevChildOutput({ terminal, scope: logType, writeLog: (text) => childLog.write(text), onLine: (line) => readiness?.observe(line) });
  childOutputTails.set(child, output.tail);
  childHiddenOutputTails.set(child, output.hiddenTail);
  child.stdout.on("data", (chunk) => output.stdout.write(chunk));
  child.stderr.on("data", (chunk) => output.stderr.write(chunk));
  longRunningChildren.add(child);
  child.once("close", () => {
    output.end();
    longRunningChildren.delete(child);
    childLog.write(`[${formatUTCLogTimestamp()}] 进程结束：exit=${child.exitCode} signal=${child.signalCode || "none"}\n`);
  });
  return child;
}

function createChildEnvironment(extraEnv = {}) {
  const childEnv = { ...baseChildEnvironment };
  for (const [key, value] of Object.entries(extraEnv)) {
    if (value !== undefined) {
      childEnv[key] = String(value);
    }
  }
  return childEnv;
}

function createSpawnSpec(command, args) {
  if (command === "pnpm") {
    return { command: process.execPath, args: [corepackCliPath, "pnpm", "--config.verify-deps-before-run=false", ...args] };
  }
  if (command === "go") {
    return { command: resolveGoExecutablePath(), args };
  }
  if (path.isAbsolute(command) && fs.existsSync(command)) {
    return { command, args };
  }
  throw new Error(`Unsupported child command: ${command}`);
}

function waitForChild(child) {
  return new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("close", (code, signal) => {
      resolve({ code: normalizeExitCode(code, signal), signal });
    });
  });
}

async function waitForChildExit(child, timeoutMs = 5_000) {
  await waitForChildProcessExit(child, { timeoutMs });
}

function normalizeExitCode(code, signal) {
  if (typeof code === "number") {
    return code;
  }
  return signal ? 1 : 0;
}

function assertDevelopmentRunning() {
  if (shuttingDown) throw Object.assign(new Error("开发流程正在关闭"), { code: "DEV_SHUTDOWN" });
}

function cleanup() {
  cleanupPromise ??= cleanupDevelopmentProcesses();
  return cleanupPromise;
}

async function cleanupDevelopmentProcesses() {
  shuttingDown = true;
  const callbacks = [...cleanupCallbacks];
  cleanupCallbacks.clear();
  const results = await Promise.allSettled(callbacks.map((callback) => callback()));
  for (const result of results) if (result.status === "rejected") log(`开发清理失败：${result.reason?.message ?? result.reason}`, "error");
  const children = [...longRunningChildren];
  longRunningChildren.clear();
  const terminationResults = await Promise.allSettled(children.map((child) => terminateChild(child)));
  for (const result of terminationResults) {
    if (result.status === "rejected") {
      log(`停止开发子进程失败：${result.reason?.message ?? result.reason}`, "error");
    }
  }
  for (const binaryPath of [serverDevBinaryPath, serverDevCandidateBinaryPath, serverDevPreviousBinaryPath]) {
    try {
      await removeFileWithRetry(binaryPath);
    } catch (error) {
      log(`清理开发 Server 二进制失败（${relativePath(binaryPath)}）：${error?.message ?? error}`, "error");
    }
  }
  try {
    await releaseActiveDevelopmentServerLease();
  } catch (error) {
    log(`释放开发 Server 租约失败：${error?.message ?? error}`, "error");
  }
}

async function shutdown(code) {
  if (shuttingDown) {
    return;
  }
  shuttingDown = true;
  log("正在关闭开发进程。");
  await cleanup();
  await closeLogs();
  process.exit(code);
}

async function terminateChild(child) {
  if (child.exitCode !== null || child.signalCode !== null || !child.pid) {
    return;
  }
  if (process.platform === "win32") {
    await new Promise((resolve) => {
      const killer = spawn("taskkill", ["/pid", String(child.pid), "/T", "/F"], {
        stdio: "ignore",
        windowsHide: true,
      });
      killer.once("exit", resolve);
      killer.once("error", resolve);
    });
    await waitForChildExit(child);
    return;
  }
  child.kill("SIGTERM");
  await waitForChildExit(child);
}

async function removeFileWithRetry(targetPath, attempts = 10, retryDelayMs = 100) {
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    try {
      await fsp.rm(targetPath, { force: true });
      return;
    } catch (error) {
      const retryable = error?.code === "EPERM" || error?.code === "EBUSY";
      if (!retryable || attempt === attempts) {
        throw error;
      }
      await delay(retryDelayMs);
    }
  }
}

function log(message, level = "info") {
  terminal.write(message, { level });
}

function reportStartup() {
  if (startupReported) return;
  startupReported = true;
  terminal.summary({
    elapsedMs: Date.now() - startupStarted,
    webURL: !shouldSkipLaunch() && resolveStartProfile(process.env) !== BUILD_PROFILE ? WEB_DEV_BASE_URL : undefined,
    skipLaunch: shouldSkipLaunch(),
  });
}

function writeStartLog(chunk) {
  developmentLogs.start.write(`[${formatUTCLogTimestamp()}] ${redactLogLine(chunk)}`);
}

async function closeLogs() {
  terminal.close();
  await Promise.all(Object.values(developmentLogs).map((writer) => writer.end()));
}

function shouldSkipLaunch() {
  return process.env.RAYLEA_START_SKIP_LAUNCH === "1";
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function relativePath(targetPath) {
  return path.relative(rootDir, targetPath).replaceAll(path.sep, "/");
}
