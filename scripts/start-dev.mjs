import { spawn } from "node:child_process";
import fs from "node:fs";
import fsp from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import {
  BUILD_PROFILE,
  LAUNCHER_DEV_PROFILE,
  WEB_DEV_BASE_URL,
  WEB_DEV_PORT,
  WEB_DEV_PROFILE,
  classifyWebDevServer,
  createDevEnvironment,
  markDependenciesInstalled,
  resolveBackendBaseUrl,
  resolveInstallMode,
  resolveStartProfile,
  shouldInstallDependencies,
} from "./start-dev-support.mjs";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(scriptDir, "..");
const webDir = path.join(rootDir, "web");
const serverDir = path.join(rootDir, "server");
const launcherDir = path.join(rootDir, "launcher");
const logDir = path.join(rootDir, "logs", "dev");
const webDevLogPath = path.join(logDir, "web-dev.log");
const launcherLogPath = path.join(logDir, "launcher.log");
const startLogPath = path.join(logDir, "start.log");
const longRunningChildren = new Set();
let startLog;
let shuttingDown = false;

await fsp.mkdir(logDir, { recursive: true });
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

  log(`启动配置：profile=${profile} install=${installMode}`);

  if (profile === BUILD_PROFILE) {
    await runBuildProfile({ installMode });
    return;
  }

  const backendBaseUrl = await resolveBackendBaseUrl({ rootDir, env: process.env });
  const devEnvironment = createDevEnvironment({ env: process.env, backendBaseUrl });
  log(`后端地址：${backendBaseUrl}`);

  if (profile === WEB_DEV_PROFILE) {
    await runWebDevProfile({ installMode, devEnvironment });
    return;
  }
  if (profile === LAUNCHER_DEV_PROFILE) {
    await runLauncherDevProfile({ installMode, devEnvironment });
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

async function runWebDevProfile({ installMode, devEnvironment }) {
  await ensureDependencies("Web", webDir, installMode);
  await buildServer();
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

async function runLauncherDevProfile({ installMode, devEnvironment }) {
  await ensureDependencies("Web", webDir, installMode);
  await buildServer();
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
  await runCommand("构建 Server", "go", ["build", "-o", "raylea-server.exe", "./cmd/raylea-server"], {
    cwd: serverDir,
  });
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
  const state = await classifyWebDevServer();
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

  await waitForWebDevServer(child);
}

async function waitForWebDevServer(child) {
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      throw new Error("Web 开发服务器已退出。");
    }
    const state = await classifyWebDevServer({ timeoutMs: 800 });
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
    env: { ...process.env, ...env },
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

function shouldSkipLaunch() {
  return process.env.RAYLEA_START_SKIP_LAUNCH === "1";
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function relativePath(targetPath) {
  return path.relative(rootDir, targetPath).replaceAll(path.sep, "/");
}
