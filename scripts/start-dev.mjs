import { spawn } from "node:child_process";
import fs from "node:fs";
import fsp from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import {
  BUILD_PROFILE,
  LAUNCHER_DEV_PROFILE,
  SERVER_RELOAD_WATCH,
  WEB_DEV_BASE_URL,
  WEB_DEV_PORT,
  WEB_DEV_PROFILE,
  classifyWebDevServer,
  createDevEnvironment,
  markDependenciesInstalled,
  resolveDatedLogPath,
  resolveBackendBaseUrl,
  resolveInstallMode,
  resolveServerReloadMode,
  resolveStartProfile,
  shouldInstallDependencies,
} from "./start-dev-support.mjs";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(scriptDir, "..");
const webDir = path.join(rootDir, "web");
const serverDir = path.join(rootDir, "server");
const serverDistDir = path.join(serverDir, "dist");
const serverBinaryName = process.platform === "win32"
  ? "raylea-server.exe"
  : "raylea-server";
const serverTmpDir = path.join(serverDir, "tmp");
const serverDevBinaryName = process.platform === "win32"
  ? "raylea-server-dev.exe"
  : "raylea-server-dev";
const serverDevBinaryPath = path.join(serverTmpDir, serverDevBinaryName);
const serverWatchDirs = [path.join(serverDir, "cmd"), path.join(serverDir, "internal")];
const serverWatchExcludedDirs = new Set([".cache", ".gocache", "dist", "logs", "tmp"]);
const serverReloadDebounceMs = 500;
const childGoCacheDir = path.join(rootDir, ".tmp", "gocache");
const baseChildEnvironment = {
  GOCACHE: childGoCacheDir,
  ...(process.platform === "win32"
    ? {
      ComSpec: "C:\\Windows\\System32\\cmd.exe",
      PATHEXT: ".COM;.EXE;.BAT;.CMD",
      SystemRoot: "C:\\Windows",
      WINDIR: "C:\\Windows",
    }
    : {
      LANG: "C.UTF-8",
      PATH: "/usr/local/bin:/usr/bin:/bin",
    }),
};
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
  await cleanup();
  startLog.end();
  process.exitCode = 1;
}

async function main() {
  const profile = resolveStartProfile(process.env);
  const installMode = resolveInstallMode(process.env);
  const serverReloadMode = resolveServerReloadMode(process.env);

  log(`启动配置：profile=${profile} install=${installMode} server_reload=${serverReloadMode || "off"}`);

  if (profile === BUILD_PROFILE) {
    await runBuildProfile({ installMode });
    return;
  }

  const backendBaseUrl = await resolveBackendBaseUrl({ rootDir, env: process.env });
  const devEnvironment = createDevEnvironment({ env: process.env, backendBaseUrl });
  log(`后端地址：${backendBaseUrl}`);

  if (profile === WEB_DEV_PROFILE) {
    await runWebDevProfile({ installMode, devEnvironment, serverReloadMode, backendBaseUrl });
    return;
  }
  if (profile === LAUNCHER_DEV_PROFILE) {
    await runLauncherDevProfile({ installMode, devEnvironment, serverReloadMode, backendBaseUrl });
    return;
  }

  throw new Error(`Unsupported profile: ${profile}`);
}

async function runBuildProfile({ installMode }) {
  await ensureDependencies("Web", webDir, installMode);
  await runCommand("构建 Web 静态资源", "pnpm", ["run", "build"], { cwd: webDir });
  await buildServer();
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

async function runWebDevProfile({ installMode, devEnvironment, serverReloadMode, backendBaseUrl }) {
  await ensureDependencies("Web", webDir, installMode);
  await ensureServerRuntime({ serverReloadMode, backendBaseUrl });
  await ensureWebDevServer(devEnvironment);
  await ensureDependencies("Launcher", launcherDir, installMode);
  await buildLauncherApp();
  if (shouldSkipLaunch()) {
    log("已跳过 Launcher 启动。");
    return;
  }
  await runCommand("启动 Launcher", "pnpm", ["exec", "electron", "."], {
    cwd: launcherDir,
    env: devEnvironment,
    logPath: launcherLogPath,
  });
}

async function runLauncherDevProfile({ installMode, devEnvironment, serverReloadMode, backendBaseUrl }) {
  await ensureDependencies("Web", webDir, installMode);
  await ensureServerRuntime({ serverReloadMode, backendBaseUrl });
  await ensureWebDevServer(devEnvironment);
  await ensureDependencies("Launcher", launcherDir, installMode);
  if (shouldSkipLaunch()) {
    log("已跳过 Launcher 启动。");
    return;
  }
  await runCommand("启动 Launcher 开发模式", "pnpm", ["run", "dev"], {
    cwd: launcherDir,
    env: devEnvironment,
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

async function ensureServerRuntime({ serverReloadMode, backendBaseUrl }) {
  if (serverReloadMode === SERVER_RELOAD_WATCH) {
    await startServerWatch(backendBaseUrl);
    return;
  }
  await buildServer();
}

async function startServerWatch(backendBaseUrl) {
  log("启动 Server 热重载：内置 watcher");
  await fsp.rm(serverDevBinaryPath, { force: true });
  await buildServerDevBinary();
  let child = startServerDevProcess();
  await waitForServerProcess(child, backendBaseUrl, "Server 热重载已启动。");

  let timer;
  let rebuilding = false;
  let pending = false;

  const scheduleReload = (sourcePath) => {
    if (shuttingDown) {
      return;
    }
    clearTimeout(timer);
    timer = setTimeout(() => {
      void rebuildAndRestart(sourcePath);
    }, serverReloadDebounceMs);
  };

  const rebuildAndRestart = async (sourcePath) => {
    if (rebuilding) {
      pending = true;
      return;
    }
    rebuilding = true;
    try {
      log(`检测到 Server 源码变更：${relativePath(sourcePath)}`);
      await buildServerDevBinary();
      await terminateChild(child);
      child = startServerDevProcess();
      await waitForServerProcess(child, backendBaseUrl, "Server 热重载已重启。");
    } catch (error) {
      log(`Server 热重载失败：${error?.message ?? error}`, "error");
    } finally {
      rebuilding = false;
      if (pending) {
        pending = false;
        scheduleReload(sourcePath);
      }
    }
  };

  const stopWatching = await watchServerSources(scheduleReload);
  cleanupCallbacks.add(async () => {
    clearTimeout(timer);
    cleanupCallbacks.delete(stopWatching);
    await stopWatching();
  });
}

async function buildServerDevBinary() {
  await fsp.mkdir(serverTmpDir, { recursive: true });
  await runCommand(
    "构建 Server 热重载二进制",
    "go",
    ["build", "-o", path.relative(serverDir, serverDevBinaryPath), "./cmd/raylea-server"],
    { cwd: serverDir, logPath: serverDevLogPath },
  );
}

function startServerDevProcess() {
  return spawnManaged("tmp/" + serverDevBinaryName, [
    "-config",
    "../config/user.yaml",
    "-config-schema",
    "../contracts/config.user.schema.json",
  ], {
    cwd: serverDir,
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
        log(readyMessage);
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
  for (const watchRoot of serverWatchDirs) {
    await watchServerDirectory(watchRoot, onChange, watchers);
  }
  return async () => {
    for (const watcher of watchers) {
      watcher.close();
    }
  };
}

async function watchServerDirectory(directory, onChange, watchers) {
  const entries = await fsp.readdir(directory, { withFileTypes: true });
  const watcher = fs.watch(directory, (eventType, filename) => {
    if (!filename) {
      return;
    }
    const sourcePath = path.join(directory, filename.toString());
    if (isWatchedGoSource(sourcePath)) {
      onChange(sourcePath);
    }
    if (eventType === "rename") {
      void watchNewDirectory(sourcePath, onChange, watchers);
    }
  });
  watchers.push(watcher);

  await Promise.all(entries
    .filter((entry) => entry.isDirectory() && !serverWatchExcludedDirs.has(entry.name))
    .map((entry) => watchServerDirectory(path.join(directory, entry.name), onChange, watchers)));
}

async function watchNewDirectory(directory, onChange, watchers) {
  try {
    const stat = await fsp.stat(directory);
    if (stat.isDirectory() && !serverWatchExcludedDirs.has(path.basename(directory))) {
      await watchServerDirectory(directory, onChange, watchers);
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
  await runCommand(`安装 ${label} 依赖`, "pnpm", ["install", "--frozen-lockfile"], { cwd: projectDir });
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
  const child = spawn(spawnSpec.command, spawnSpec.args, {
    cwd,
    env: createChildEnvironment(env),
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
  if (process.platform !== "win32") {
    return { command, args };
  }
  return {
    command: "cmd.exe",
    args: ["/d", "/s", "/c", [command, ...args].map(quoteCmdArg).join(" ")],
  };
}

function quoteCmdArg(value) {
  const text = String(value);
  if (/^[A-Za-z0-9_./:\\=-]+$/.test(text)) {
    return text;
  }
  return `"${text.replaceAll('"', '""')}"`;
}

function waitForChild(child) {
  return new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("exit", (code, signal) => {
      resolve({ code: normalizeExitCode(code, signal), signal });
    });
  });
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
  await Promise.all(children.map((child) => terminateChild(child)));
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
  if (child.exitCode !== null || child.killed || !child.pid) {
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
    return;
  }
  child.kill("SIGTERM");
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
