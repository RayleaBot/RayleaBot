import { spawn } from "node:child_process";
import fs from "node:fs";
import fsp from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { createFileContentTracker } from "./file-content-tracker.mjs";
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
  createDependencyInstallEnvironment,
  createServerDevelopmentEnvironment,
  isProcessRunning,
  markDependenciesInstalled,
  parseDevelopmentServerLease,
  requestDevelopmentServerShutdown,
  createTrustedChildEnvironment,
  loadStartEnvironmentFile,
  resolveDatedLogPath,
  resolveBackendBaseUrl,
  resolveInstallMode,
  resolveServerReloadMode,
  resolveStartProfile,
  shouldInstallDependencies,
  waitForChildProcessExit,
} from "./start-dev-support.mjs";
import {
  collectWorkspaceSDKVersions,
  createDevelopmentReloadQueue,
  currentPluginPlatform,
  loadPluginWorkspace,
  mirrorVueSDK,
  PLUGIN_DEV_OFF,
  PLUGIN_DEV_WATCH,
  renderDevelopmentGoWork,
  resolvePluginDevMode,
  selectWorkspacePlugins,
  watchPluginWorkspace,
} from "./plugin-dev-workspace.mjs";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(scriptDir, "..");
loadStartEnvironmentFile({ rootDir });
const corepackCliPath = path.join(path.dirname(process.execPath), "node_modules", "corepack", "dist", "corepack.js");
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
const serverWatchDirs = [path.join(serverDir, "cmd"), path.join(serverDir, "internal")];
const serverWatchExcludedDirs = new Set([".cache", ".gocache", "dist", "logs", "tmp"]);
const serverReloadDebounceMs = 500;
const childGoCacheDir = path.join(rootDir, ".tmp", "gocache");
const pluginWorkspacePath = path.resolve(rootDir, process.env.RAYLEA_PLUGIN_WORKSPACE || "plugin-workspace.local.json");
const pluginDevRoot = path.join(rootDir, ".tmp", "plugin-dev");
const pluginDevGoWorkPath = path.join(pluginDevRoot, "go.work");
const pluginDevArtifactRoot = path.join(pluginDevRoot, "artifacts");
const baseChildEnvironment = {
  ...createTrustedChildEnvironment({ nodeExecutablePath: process.execPath }),
  GOCACHE: childGoCacheDir,
};
const developmentControlEnvironment = createDevelopmentControlEnvironment();
const developmentControlToken = developmentControlEnvironment[LAUNCHER_CONTROL_TOKEN_ENV];
const developmentServerWatcherEnvironment = createDevelopmentServerWatcherEnvironment({
  ownerPid: process.pid,
});
const launcherDir = path.join(rootDir, "launcher");
const logDate = new Date();
const webDevLogPath = resolveDatedLogPath({ rootDir, scope: "dev", type: "web", date: logDate });
const launcherLogPath = resolveDatedLogPath({ rootDir, scope: "dev", type: "launcher", date: logDate });
const serverDevLogPath = resolveDatedLogPath({ rootDir, scope: "dev", type: "server", date: logDate });
const startLogPath = resolveDatedLogPath({ rootDir, scope: "dev", type: "start", date: logDate });
const longRunningChildren = new Set();
const cleanupCallbacks = new Set();
let startLog;
let shuttingDown = false;
let activeServerDevLeaseId = "";

await prepareLogDirectories([webDevLogPath, launcherLogPath, serverDevLogPath, startLogPath]);
await fsp.mkdir(childGoCacheDir, { recursive: true });
startLog = fs.createWriteStream(startLogPath, { flags: "a" });

process.once("SIGINT", () => {
  void shutdown(130);
});
process.once("SIGTERM", () => {
  void shutdown(143);
});

try {
  await main();
  await cleanup();
  startLog.end();
} catch (error) {
  log(`启动失败：${error?.message ?? error}`, "error");
  log(`启动日志：${relativePath(startLogPath)}`, "error");
  await cleanup();
  startLog.end();
  process.exitCode = 1;
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

  log(`启动配置：profile=${profile} install=${installMode} server_reload=${serverReloadMode || "off"} plugin_dev=${pluginDevMode}`);

  if (profile === BUILD_PROFILE) {
    await runBuildProfile({ installMode, pluginDev });
    return;
  }

  const backendBaseUrl = await resolveBackendBaseUrl({ rootDir, env: process.env });
  const devEnvironment = createDevEnvironment({ env: process.env, backendBaseUrl });
  const serverDevEnvironment = createServerDevelopmentEnvironment({
    devEnvironment,
    controlEnvironment: developmentControlEnvironment,
  });
  log(`后端地址：${backendBaseUrl}`);

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
  await ensureDependencies("Web", webDir, installMode);
  await runCommand("构建 Web 静态资源", "pnpm", ["run", "build"], { cwd: webDir });
  await buildServer();
  await syncDevelopmentPlugins(pluginDev, path.join(serverDistDir, serverBinaryName));
  await ensureDependencies("Launcher", launcherDir, installMode);
  await buildLauncherApp();
  if (shouldSkipLaunch()) {
    log("已跳过 Launcher 启动。");
    return;
  }
  await runCommand("启动 Launcher", "pnpm", ["exec", "electron", "."], {
    cwd: launcherDir,
    env: { RAYLEA_WEB_UI_BASE_URL: "" },
    logPath: launcherLogPath,
  });
}

async function runWebDevProfile({ installMode, devEnvironment, serverDevEnvironment, serverReloadMode, backendBaseUrl, pluginDev }) {
  await ensureDependencies("Web", webDir, installMode);
  await ensureServerRuntime({ serverReloadMode, backendBaseUrl, pluginDev, serverDevEnvironment });
  await ensureWebDevServer(devEnvironment);
  await ensureDependencies("Launcher", launcherDir, installMode);
  await buildLauncherApp();
  if (shouldSkipLaunch()) {
    log("已跳过 Launcher 启动。");
    return;
  }
  await runCommand("启动 Launcher", "pnpm", ["exec", "electron", "."], {
    cwd: launcherDir,
    env: {
      ...devEnvironment,
      ...developmentControlEnvironment,
      ...(serverReloadMode === SERVER_RELOAD_WATCH ? developmentServerWatcherEnvironment : {}),
    },
    logPath: launcherLogPath,
  });
}

async function runLauncherDevProfile({ installMode, devEnvironment, serverDevEnvironment, serverReloadMode, backendBaseUrl, pluginDev }) {
  await ensureDependencies("Web", webDir, installMode);
  await ensureServerRuntime({ serverReloadMode, backendBaseUrl, pluginDev, serverDevEnvironment });
  await ensureWebDevServer(devEnvironment);
  await ensureDependencies("Launcher", launcherDir, installMode);
  if (shouldSkipLaunch()) {
    log("已跳过 Launcher 启动。");
    return;
  }
  await runCommand("启动 Launcher 开发模式", "pnpm", ["run", "dev"], {
    cwd: launcherDir,
    env: {
      ...devEnvironment,
      ...developmentControlEnvironment,
      ...(serverReloadMode === SERVER_RELOAD_WATCH ? developmentServerWatcherEnvironment : {}),
    },
    logPath: launcherLogPath,
  });
}

async function buildServer() {
  await fsp.mkdir(serverDistDir, { recursive: true });
  await runCommand(
    "构建 Server",
    "go",
    ["build", "-o", path.join("dist", serverBinaryName), "./cmd/raylea-server"],
    { cwd: serverDir },
  );
}

async function buildDevelopmentPlugins(pluginDev, pluginIDs) {
  if (!pluginDev || pluginDev.mode === PLUGIN_DEV_OFF) {
    return {
      workspace: { workspaceVersion: "1", plugins: [] },
      platform: currentPluginPlatform(),
      plugins: [],
    };
  }
  const workspace = await loadPluginWorkspace(pluginDev.workspacePath);
  if (workspace.plugins.length === 0) {
    log(`开发插件工作区为空：${relativePath(pluginDev.workspacePath)}`);
    return { workspace, platform: currentPluginPlatform(), plugins: [] };
  }
  const platform = currentPluginPlatform();
  const sdkGoVersions = await collectWorkspaceSDKVersions(workspace.plugins);
  await fsp.mkdir(pluginDevRoot, { recursive: true });
  await fsp.writeFile(pluginDevGoWorkPath, renderDevelopmentGoWork({
    sdkGoPath: path.join(rootDir, "sdk", "go"),
    sdkGoVersions,
    plugins: workspace.plugins,
  }), "utf8");
  await fsp.mkdir(pluginDevArtifactRoot, { recursive: true });

  const pluginsToSync = selectWorkspacePlugins(workspace.plugins, pluginIDs);
  for (const plugin of pluginsToSync) {
    const buildEntry = path.join(plugin.path, "tools", "build");
    if (!fs.existsSync(path.join(plugin.path, "info.json")) || !fs.existsSync(buildEntry)) {
      throw new Error(`开发插件 ${plugin.id} 缺少 info.json 或 tools/build：${plugin.path}`);
    }
    await mirrorVueSDK({ sdkVuePath: path.join(rootDir, "sdk", "vue"), pluginPath: plugin.path });
    await runCommand(`构建开发插件 ${plugin.id}`, "go", [
      "run",
      "./tools/build",
      "-target",
      platform,
      "-out",
      pluginDevArtifactRoot,
    ], {
      cwd: plugin.path,
      env: {
        ...createDependencyInstallEnvironment(),
        GOWORK: pluginDevGoWorkPath,
        RAYLEA_PLUGIN_BUILD_USE_WORKSPACE: "1",
        RAYLEA_PLUGIN_BUILD_NODE: process.execPath,
        RAYLEA_PLUGIN_BUILD_COREPACK_CLI: corepackCliPath,
      },
    });
  }
  return { workspace, platform, plugins: pluginsToSync };
}

async function installDevelopmentPlugins(preparedPlugins, serverBinaryPath) {
  for (const plugin of preparedPlugins.plugins) {
    const expandedArtifact = path.join(pluginDevArtifactRoot, preparedPlugins.platform, plugin.id);
    await runCommand(`同步开发插件 ${plugin.id}`, serverBinaryPath, [
      "-config",
      path.join(rootDir, "config", "user.yaml"),
      "plugin",
      "dev-sync",
      "--artifact",
      expandedArtifact,
      "--source",
      plugin.path,
      "--plugin-id",
      plugin.id,
    ], { cwd: rootDir });
  }
}

async function syncDevelopmentPlugins(pluginDev, serverBinaryPath, pluginIDs) {
  const preparedPlugins = await buildDevelopmentPlugins(pluginDev, pluginIDs);
  await installDevelopmentPlugins(preparedPlugins, serverBinaryPath);
  return preparedPlugins.workspace;
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
  log("启动 Server 热重载：内置 watcher");
  const lease = await acquireDevelopmentServerLease(backendBaseUrl);
  activeServerDevLeaseId = lease.lease_id;
  await fsp.rm(serverDevBinaryPath, { force: true });
  await buildServerDevBinary();
  const pluginWorkspace = await syncDevelopmentPlugins(pluginDev, serverDevBinaryPath);
  let child = startServerDevProcess(serverDevEnvironment);
  await waitForServerProcess(child, backendBaseUrl, "Server 热重载已启动。");

  let timer;
  let rebuilding = false;
  const reloadQueue = createDevelopmentReloadQueue();
  const expectedServerExits = new Set();

  const expectServerExit = (target) => {
    if (target?.pid) {
      expectedServerExits.add(target.pid);
    }
  };

  const monitorServerExit = (target) => {
    const pid = target.pid;
    target.once("exit", (code, signal) => {
      if (expectedServerExits.delete(pid) || shuttingDown) {
        return;
      }
      const exitCode = normalizeExitCode(code, signal);
      log(
        exitCode === 0
          ? `Server 已在 watcher 之外停止（watcher PID ${process.pid}，Server PID ${pid ?? "unknown"}），正在结束当前开发启动流程。`
          : `Server 进程意外退出（watcher PID ${process.pid}，Server PID ${pid ?? "unknown"}，退出码 ${exitCode}，信号 ${signal ?? "none"}）。`,
        exitCode === 0 ? "info" : "error",
      );
      void shutdown(exitCode);
    });
  };
  monitorServerExit(child);

  const scheduleReload = () => {
    if (shuttingDown) {
      return;
    }
    clearTimeout(timer);
    timer = setTimeout(() => {
      void rebuildAndRestart();
    }, serverReloadDebounceMs);
  };

  const queueServerReload = (sourcePath) => {
    reloadQueue.addServer(sourcePath);
    if (!rebuilding) {
      scheduleReload();
    }
  };

  const queuePluginReload = (plugin, sourcePath) => {
    reloadQueue.addPlugin(plugin, sourcePath);
    if (!rebuilding) {
      scheduleReload();
    }
  };

  const rebuildAndRestart = async () => {
    if (rebuilding || !reloadQueue.hasChanges()) {
      return;
    }
    const { serverSourcePath, pluginChanges } = reloadQueue.take();
    rebuilding = true;
    let serverStopped = false;
    const reloadStartedAt = Date.now();
    const previousServerPid = child.pid ?? null;
    let downtimeStartedAt = null;
    try {
      if (serverSourcePath) {
        log(`检测到 Server 源码变更：${relativePath(serverSourcePath)}`);
      }
      for (const { plugin, sourcePath } of pluginChanges) {
        log(`检测到开发插件源码变更：${plugin.id} (${sourcePath})`);
      }
      log(
        `Server 热重载准备中：watcher PID ${process.pid}，当前 Server PID ${previousServerPid ?? "unknown"}，`
        + `Server 源码=${serverSourcePath ? "是" : "否"}，开发插件=${pluginChanges.length}。`,
      );
      if (serverSourcePath) {
        await fsp.rm(serverDevCandidateBinaryPath, { force: true });
        await buildServerDevBinaryAt(serverDevCandidateBinaryPath);
      }
      const preparedPlugins = pluginChanges.length > 0
        ? await buildDevelopmentPlugins(
          pluginDev,
          pluginChanges.map(({ plugin }) => plugin.id),
        )
        : null;
      if (serverSourcePath || pluginChanges.length > 0) {
        downtimeStartedAt = Date.now();
        expectServerExit(child);
        await terminateChild(child);
        serverStopped = true;
        log(
          `旧 Server 已停止（PID ${previousServerPid ?? "unknown"}）；预构建产物已就绪，正在切换运行时。`,
        );
      }
      if (serverSourcePath) {
        await replaceServerDevBinary(serverDevCandidateBinaryPath);
      }
      if (preparedPlugins) {
        await installDevelopmentPlugins(preparedPlugins, serverDevBinaryPath);
      }
      child = startServerDevProcess(serverDevEnvironment);
      await waitForServerProcess(child, backendBaseUrl, null);
      monitorServerExit(child);
      serverStopped = false;
      log(
        `Server 热重载完成：watcher PID ${process.pid}，Server PID ${previousServerPid ?? "unknown"} -> ${child.pid ?? "unknown"}，`
        + `服务切换 ${downtimeStartedAt === null ? 0 : Date.now() - downtimeStartedAt} ms，总耗时 ${Date.now() - reloadStartedAt} ms。`,
      );
    } catch (error) {
      log(
        `Server 热重载失败：watcher PID ${process.pid}，原 Server PID ${previousServerPid ?? "unknown"}，`
        + `已耗时 ${Date.now() - reloadStartedAt} ms，错误：${error?.message ?? error}`,
        "error",
      );
      if (serverStopped && !shuttingDown) {
        try {
          log("开发插件更新未生效，正在使用已安装的上一个可用产物恢复 Server。");
          child = startServerDevProcess(serverDevEnvironment);
          await waitForServerProcess(child, backendBaseUrl, "Server 已使用上一个可用插件产物恢复。");
          monitorServerExit(child);
        } catch (recoveryError) {
          log(`Server 恢复失败：${recoveryError?.message ?? recoveryError}`, "error");
        }
      }
    } finally {
      rebuilding = false;
      if (reloadQueue.hasChanges()) {
        scheduleReload();
      }
    }
  };

  const stopWatching = await watchServerSources(queueServerReload);
  const stopPluginWatching = pluginDev.mode === PLUGIN_DEV_WATCH
    ? await watchPluginWorkspace(pluginWorkspace.plugins, queuePluginReload)
    : async () => {};
  cleanupCallbacks.add(async () => {
    clearTimeout(timer);
    expectServerExit(child);
    cleanupCallbacks.delete(stopWatching);
    await stopWatching();
    await stopPluginWatching();
  });
}

async function acquireDevelopmentServerLease(backendBaseUrl) {
  await fsp.mkdir(path.dirname(serverDevLeasePath), { recursive: true });
  for (let attempt = 0; attempt < 3; attempt += 1) {
    const existingLease = await readDevelopmentServerLease();
    if (existingLease) {
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
  await runCommand(
    "构建 Server 热重载二进制",
    "go",
    ["build", "-o", path.relative(serverDir, outputPath), "./cmd/raylea-server"],
    { cwd: serverDir, logPath: serverDevLogPath },
  );
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
  await fsp.rm(serverDevPreviousBinaryPath, { force: true });
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
    logPath: serverDevLogPath,
  });
}

async function waitForServerProcess(child, backendBaseUrl, readyMessage) {
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      throw new Error("Server 热重载进程已退出。");
    }
    try {
      if (await isServerHealthy(backendBaseUrl)) {
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
    await delay(500);
  }
  throw new Error(`Server 热重载未在 30 秒内完成首次构建，日志见 ${relativePath(serverDevLogPath)}。`);
}

async function watchServerSources(onChange) {
  const watchers = [];
  const contentTracker = createFileContentTracker();
  for (const watchRoot of serverWatchDirs) {
    await watchServerDirectory(watchRoot, onChange, watchers, contentTracker);
  }
  return async () => {
    for (const watcher of watchers) {
      watcher.close();
    }
  };
}

async function watchServerDirectory(directory, onChange, watchers, contentTracker) {
  const entries = await fsp.readdir(directory, { withFileTypes: true });
  await Promise.all(entries
    .filter((entry) => !entry.isDirectory())
    .map((entry) => path.join(directory, entry.name))
    .filter(isWatchedGoSource)
    .map((sourcePath) => contentTracker.prime(sourcePath)));
  const watcher = fs.watch(directory, (eventType, filename) => {
    if (!filename) {
      return;
    }
    const sourcePath = path.join(directory, filename.toString());
    if (isWatchedGoSource(sourcePath)) {
      void contentTracker.hasChanged(sourcePath)
        .then((changed) => {
          if (changed) {
            onChange(sourcePath);
          }
        })
        .catch((error) => {
          log(`Server 热重载校验源码变更失败：${error?.message ?? error}`, "error");
        });
    }
    if (eventType === "rename") {
      void watchNewDirectory(sourcePath, onChange, watchers, contentTracker);
    }
  });
  watchers.push(watcher);

  await Promise.all(entries
    .filter((entry) => entry.isDirectory() && !serverWatchExcludedDirs.has(entry.name))
    .map((entry) => watchServerDirectory(
      path.join(directory, entry.name),
      onChange,
      watchers,
      contentTracker,
    )));
}

async function watchNewDirectory(directory, onChange, watchers, contentTracker) {
  try {
    const stat = await fsp.stat(directory);
    if (stat.isDirectory() && !serverWatchExcludedDirs.has(path.basename(directory))) {
      await watchServerDirectory(directory, onChange, watchers, contentTracker);
    }
  } catch (error) {
    if (error?.code !== "ENOENT") {
      log(`Server 热重载监听新目录失败：${error?.message ?? error}`, "error");
    }
  }
}

function isWatchedGoSource(sourcePath) {
  return sourcePath.endsWith(".go") && !sourcePath.endsWith("_test.go");
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
  await runCommand("构建 Launcher App", "pnpm", ["run", "build:app"], { cwd: launcherDir });
}

async function ensureDependencies(label, projectDir, installMode) {
  const shouldInstall = await shouldInstallDependencies({ projectDir, mode: installMode });
  if (!shouldInstall) {
    log(`${label} 依赖可用。`);
    return;
  }
  await runCommand(`安装 ${label} 依赖`, "pnpm", ["install", "--frozen-lockfile"], {
    cwd: projectDir,
    env: createDependencyInstallEnvironment(),
  });
  await markDependenciesInstalled({ projectDir });
}

async function ensureWebDevServer(devEnvironment) {
  const state = await classifyWebDevServer({ backendBaseUrl: devEnvironment.VITE_BACKEND_TARGET });
  if (state === "rayleabot") {
    log(`复用 Web 开发服务器：${WEB_DEV_BASE_URL}`);
    return;
  }
  if (state === "occupied") {
    throw new Error(`端口 ${WEB_DEV_PORT} 已被其他程序占用。请关闭占用程序，或使用 RAYLEA_START_PROFILE=build。`);
  }

  log(`启动 Web 开发服务器：${WEB_DEV_BASE_URL}`);
  const child = spawnManaged("pnpm", ["dev"], {
    cwd: webDir,
    env: devEnvironment,
    logPath: webDevLogPath,
  });

  await waitForWebDevServer(child, devEnvironment.VITE_BACKEND_TARGET);
}

async function waitForWebDevServer(child, backendBaseUrl) {
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      throw new Error("Web 开发服务器已退出。");
    }
    const state = await classifyWebDevServer({ backendBaseUrl, timeoutMs: 800 });
    if (state === "rayleabot") {
      log(`Web 开发服务器已就绪：${WEB_DEV_BASE_URL}`);
      return;
    }
    await delay(500);
  }
  throw new Error(`Web 开发服务器未在 30 秒内就绪，日志见 ${relativePath(webDevLogPath)}。`);
}

async function runCommand(label, command, args, { cwd, env = {}, logPath } = {}) {
  log(`${label}...`);
  const child = spawnManaged(command, args, { cwd, env, logPath });
  const exit = await waitForChild(child);
  if (exit.code !== 0) {
    throw new Error(`${label}失败，退出码 ${exit.code}。`);
  }
}

function spawnManaged(command, args, { cwd, env = {}, logPath } = {}) {
  const commandText = [command, ...args].join(" ");
  writeStartLog(`$ ${commandText}\n`);
  const childLog = logPath ? fs.createWriteStream(logPath, { flags: "a" }) : null;
  const spawnSpec = createSpawnSpec(command, args);
  const childOverrides = command === "pnpm"
    ? createDependencyInstallEnvironment(env)
    : env;
  const child = spawn(spawnSpec.command, spawnSpec.args, {
    cwd,
    env: createChildEnvironment(childOverrides),
    windowsHide: false,
    stdio: ["ignore", "pipe", "pipe"],
  });

  child.stdout.on("data", (chunk) => writeChildOutput(chunk, process.stdout, childLog));
  child.stderr.on("data", (chunk) => writeChildOutput(chunk, process.stderr, childLog));
  longRunningChildren.add(child);
  child.once("exit", () => {
    childLog?.end();
    longRunningChildren.delete(child);
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
    return { command: process.execPath, args: [corepackCliPath, "pnpm", ...args] };
  }
  if (command === "go") {
    return { command: resolveGoExecutablePath(), args };
  }
  if (path.isAbsolute(command) && fs.existsSync(command)) {
    return { command, args };
  }
  throw new Error(`Unsupported child command: ${command}`);
}

function resolveGoExecutablePath() {
  const executableName = process.platform === "win32" ? "go.exe" : "go";
  const candidates = String(process.env.PATH ?? "")
    .split(path.delimiter)
    .map((directory) => directory.trim().replace(/^"|"$/g, ""))
    .filter(Boolean)
    .map((directory) => path.join(directory, executableName));

  if (process.platform === "win32") {
    const programFiles = process.env.ProgramFiles?.trim();
    if (programFiles) {
      candidates.push(path.join(programFiles, "Go", "bin", executableName));
    }
  }

  const executablePath = candidates.find((candidate) => fs.existsSync(candidate));
  if (!executablePath) {
    throw new Error("Go executable was not found. Run python scripts/check-toolchain.py for installation guidance.");
  }
  return executablePath;
}

function waitForChild(child) {
  return new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("exit", (code, signal) => {
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

async function cleanup() {
  const callbacks = [...cleanupCallbacks];
  cleanupCallbacks.clear();
  await Promise.all(callbacks.map((callback) => callback()));
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
  startLog.end();
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

function writeChildOutput(chunk, output, childLog) {
  output.write(chunk);
  childLog?.write(chunk);
  writeStartLog(chunk);
}

function log(message, level = "info") {
  const prefix = level === "error" ? "[RayleaBot] " : "[RayleaBot] ";
  const line = `${prefix}${message}`;
  if (level === "error") {
    console.error(line);
  } else {
    console.log(line);
  }
  writeStartLog(`${line}\n`);
}

function writeStartLog(chunk) {
  startLog?.write(`[${new Date().toISOString()}] ${chunk}`);
}

async function prepareLogDirectories(paths) {
  await Promise.all([...new Set(paths.map((targetPath) => path.dirname(targetPath)))].map((directory) => {
    return fsp.mkdir(directory, { recursive: true });
  }));
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
