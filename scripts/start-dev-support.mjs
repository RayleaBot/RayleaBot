import { randomBytes } from "node:crypto";
import { existsSync } from "node:fs";
import fs from "node:fs/promises";
import net from "node:net";
import path from "node:path";

export const WEB_DEV_PROFILE = "web-dev";
export const BUILD_PROFILE = "build";
export const LAUNCHER_DEV_PROFILE = "launcher-dev";
export const SERVER_RELOAD_WATCH = "watch";
export const WEB_DEV_PORT = 4173;
export const WEB_DEV_BASE_URL = `http://127.0.0.1:${WEB_DEV_PORT}/`;
export const WEB_DEV_STATUS_PATH = "/__rayleabot-dev/status";
export const LAUNCHER_CONTROL_TOKEN_ENV = "RAYLEA_LAUNCHER_CONTROL_TOKEN";
export const LAUNCHER_CONTROL_TOKEN_HEADER = "X-Raylea-Launcher-Control";
export const DEVELOPMENT_SERVER_WATCHER_PID_ENV = "RAYLEA_DEV_SERVER_WATCHER_PID";
export const DEVELOPMENT_SERVER_LEASE_VERSION = 1;

const VALID_PROFILES = new Set([WEB_DEV_PROFILE, BUILD_PROFILE, LAUNCHER_DEV_PROFILE]);
const VALID_INSTALL_MODES = new Set(["auto", "always", "skip"]);
const LEGACY_SERVER_RELOAD_AIR = "air";
const VALID_SERVER_RELOAD_MODES = new Set(["", SERVER_RELOAD_WATCH, LEGACY_SERVER_RELOAD_AIR]);
const WILDCARD_HOSTS = new Set(["", "*", "0.0.0.0", "::", "[::]"]);

export function loadStartEnvironmentFile({
  rootDir,
  loadEnvFile = process.loadEnvFile,
} = {}) {
  if (!rootDir) {
    throw new Error("rootDir is required");
  }
  const environmentPath = path.join(rootDir, ".env");
  try {
    loadEnvFile(environmentPath);
  } catch (error) {
    if (error?.code !== "ENOENT") {
      throw error;
    }
  }
  return environmentPath;
}

export function formatLocalLogDate(date = new Date()) {
  if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
    throw new Error("date must be a valid Date");
  }
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function resolveDatedLogPath({ rootDir, scope = "", type, date = new Date() } = {}) {
  if (!rootDir) {
    throw new Error("rootDir is required");
  }
  if (!type) {
    throw new Error("type is required");
  }
  const segments = [rootDir, "logs"];
  if (scope) {
    segments.push(scope);
  }
  segments.push(type, `${formatLocalLogDate(date)}.log`);
  return path.join(...segments);
}

export function resolveStartProfile(env = process.env) {
  const explicitProfile = env.RAYLEA_START_PROFILE?.trim();
  if (explicitProfile) {
    return assertKnownProfile(explicitProfile);
  }
  return WEB_DEV_PROFILE;
}

export function resolveInstallMode(env = process.env) {
  const mode = env.RAYLEA_START_INSTALL?.trim().toLowerCase() || "auto";
  if (!VALID_INSTALL_MODES.has(mode)) {
    throw new Error(`Unsupported RAYLEA_START_INSTALL: ${env.RAYLEA_START_INSTALL}`);
  }
  return mode;
}

export function resolveServerReloadMode(env = process.env) {
  const mode = env.RAYLEA_SERVER_RELOAD?.trim().toLowerCase() || "";
  if (!VALID_SERVER_RELOAD_MODES.has(mode)) {
    throw new Error(`Unsupported RAYLEA_SERVER_RELOAD: ${env.RAYLEA_SERVER_RELOAD}`);
  }
  return mode === LEGACY_SERVER_RELOAD_AIR ? SERVER_RELOAD_WATCH : mode;
}

export function createDevelopmentServerWatcherEnvironment({
  ownerPid = process.pid,
} = {}) {
  if (!Number.isSafeInteger(ownerPid) || ownerPid <= 0) {
    throw new Error("development server watcher PID must be a positive integer");
  }
  return { [DEVELOPMENT_SERVER_WATCHER_PID_ENV]: String(ownerPid) };
}

export function normalizeBackendHost(host) {
  const normalized = stripQuotes(String(host ?? "").trim());
  return WILDCARD_HOSTS.has(normalized) ? "127.0.0.1" : normalized;
}

export function formatUrlHost(host) {
  const normalized = normalizeBackendHost(host);
  if (normalized.startsWith("[") && normalized.endsWith("]")) {
    return normalized;
  }
  return normalized.includes(":") ? `[${normalized}]` : normalized;
}

export function parseBackendEndpointFromConfigText(text) {
  let inServerBlock = false;
  let serverIndent = -1;
  let host;
  let port;

  for (const rawLine of String(text ?? "").split(/\r?\n/)) {
    const lineWithoutComment = stripYamlComment(rawLine);
    if (!lineWithoutComment.trim()) {
      continue;
    }

    const indent = leadingWhitespace(lineWithoutComment);
    if (!inServerBlock) {
      if (/^\s*server\s*:\s*$/.test(lineWithoutComment)) {
        inServerBlock = true;
        serverIndent = indent;
      }
      continue;
    }

    if (indent <= serverIndent) {
      break;
    }

    const match = lineWithoutComment.match(/^\s*(host|port)\s*:\s*(.*?)\s*$/);
    if (!match) {
      continue;
    }

    if (match[1] === "host") {
      host = stripQuotes(match[2].trim());
    }
    if (match[1] === "port") {
      const parsedPort = Number.parseInt(stripQuotes(match[2].trim()), 10);
      if (Number.isFinite(parsedPort) && parsedPort > 0) {
        port = parsedPort;
      }
    }
  }

  return { host, port };
}

export async function resolveBackendBaseUrl({ rootDir, env = process.env, readFile = fs.readFile } = {}) {
  const configuredTarget = env.VITE_BACKEND_TARGET?.trim();
  if (configuredTarget) {
    return trimTrailingSlash(new URL(configuredTarget).toString());
  }

  let endpoint = {};
  if (rootDir) {
    try {
      const configText = await readFile(path.join(rootDir, "config", "user.yaml"), "utf8");
      endpoint = parseBackendEndpointFromConfigText(configText);
    } catch (error) {
      if (error?.code !== "ENOENT") {
        throw error;
      }
    }
  }

  const host = formatUrlHost(endpoint.host ?? "127.0.0.1");
  const port = endpoint.port ?? 8080;
  return `http://${host}:${port}`;
}

export function createDevEnvironment({ env = process.env, backendBaseUrl, webBaseUrl = WEB_DEV_BASE_URL } = {}) {
  return {
    VITE_BACKEND_TARGET: env.VITE_BACKEND_TARGET?.trim() || backendBaseUrl,
    VITE_WS_BASE_URL: env.VITE_WS_BASE_URL?.trim() || backendBaseUrl,
    // The browser must follow the managed Vite server, independently of the API target.
    RAYLEA_WEB_UI_BASE_URL: webBaseUrl,
  };
}

export function createDevelopmentControlEnvironment({
  generateControlToken = () => randomBytes(32).toString("base64url"),
} = {}) {
  const controlToken = String(generateControlToken()).trim();
  if (!controlToken) {
    throw new Error("development launcher control token is required");
  }
  return { [LAUNCHER_CONTROL_TOKEN_ENV]: controlToken };
}

export function createDevelopmentServerLease({
  ownerPid,
  rootDir,
  backendBaseUrl,
  binaryPath,
  controlToken,
  generateLeaseId = () => randomBytes(16).toString("base64url"),
} = {}) {
  const normalizedOwnerPid = Number(ownerPid);
  const normalizedRootDir = String(rootDir ?? "").trim();
  const normalizedBinaryPath = String(binaryPath ?? "").trim();
  const normalizedControlToken = String(controlToken ?? "").trim();
  const leaseId = String(generateLeaseId()).trim();
  if (!Number.isSafeInteger(normalizedOwnerPid) || normalizedOwnerPid <= 0) {
    throw new Error("development server lease owner PID is required");
  }
  if (!normalizedControlToken) {
    throw new Error("development server lease control token is required");
  }
  if (!leaseId) {
    throw new Error("development server lease ID is required");
  }
  if (!normalizedRootDir || !normalizedBinaryPath) {
    throw new Error("development server lease paths are required");
  }

  return {
    version: DEVELOPMENT_SERVER_LEASE_VERSION,
    lease_id: leaseId,
    owner_pid: normalizedOwnerPid,
    root_dir: path.resolve(normalizedRootDir),
    backend_base_url: normalizeLoopbackHTTPURL(backendBaseUrl),
    binary_path: path.resolve(normalizedBinaryPath),
    control_token: normalizedControlToken,
  };
}

export function parseDevelopmentServerLease(text, {
  rootDir,
  serverTmpDir,
  platform = process.platform,
} = {}) {
  if (!String(rootDir ?? "").trim() || !String(serverTmpDir ?? "").trim()) {
    throw new Error("development server lease validation paths are required");
  }
  let value;
  try {
    value = JSON.parse(String(text ?? ""));
  } catch {
    throw new Error("development server lease is not valid JSON");
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error("development server lease must be an object");
  }
  if (value.version !== DEVELOPMENT_SERVER_LEASE_VERSION) {
    throw new Error("development server lease version is unsupported");
  }

  const lease = createDevelopmentServerLease({
    ownerPid: value.owner_pid,
    rootDir: value.root_dir,
    backendBaseUrl: value.backend_base_url,
    binaryPath: value.binary_path,
    controlToken: value.control_token,
    generateLeaseId: () => value.lease_id,
  });
  if (normalizeComparablePath(lease.root_dir, platform) !== normalizeComparablePath(rootDir, platform)) {
    throw new Error("development server lease belongs to another repository");
  }

  const normalizedTmpDir = normalizeComparablePath(serverTmpDir, platform);
  const normalizedBinaryPath = normalizeComparablePath(lease.binary_path, platform);
  const relativeBinaryPath = path.relative(normalizedTmpDir, normalizedBinaryPath);
  if (
    !relativeBinaryPath
    || relativeBinaryPath.startsWith("..")
    || path.isAbsolute(relativeBinaryPath)
    || !path.basename(normalizedBinaryPath).startsWith("raylea-server-dev-")
  ) {
    throw new Error("development server lease binary path is outside server/tmp");
  }
  if (typeof value.startup_identity === "string") lease.startup_identity = value.startup_identity;
  if (typeof value.ready === "boolean") lease.ready = value.ready;
  return lease;
}

export async function requestDevelopmentServerShutdown({
  lease,
  fetchImpl = globalThis.fetch,
  timeoutMs = 2_000,
} = {}) {
  if (!lease?.control_token || !lease?.backend_base_url) {
    throw new Error("development server lease is required for shutdown");
  }
  const response = await fetchWithTimeout(
    fetchImpl,
    new URL("api/launcher/shutdown", `${lease.backend_base_url}/`).toString(),
    timeoutMs,
    {
      method: "POST",
      headers: { [LAUNCHER_CONTROL_TOKEN_HEADER]: lease.control_token },
    },
  );
  if (response.status !== 202) {
    throw new Error(`development server shutdown was rejected with HTTP ${response.status}`);
  }
}

export function isProcessRunning(pid, { kill = process.kill } = {}) {
  const normalizedPID = Number(pid);
  if (!Number.isSafeInteger(normalizedPID) || normalizedPID <= 0) {
    return false;
  }
  try {
    kill(normalizedPID, 0);
    return true;
  } catch (error) {
    if (error?.code === "ESRCH") {
      return false;
    }
    if (error?.code === "EPERM") {
      return true;
    }
    throw error;
  }
}

export function createServerDevelopmentEnvironment({
  devEnvironment = {},
  controlEnvironment = {},
} = {}) {
  const webUIBaseURL = devEnvironment.RAYLEA_WEB_UI_BASE_URL?.trim();
  if (!webUIBaseURL) {
    throw new Error("development Web UI base URL is required for the server");
  }
  return {
    ...controlEnvironment,
    RAYLEA_WEB_UI_BASE_URL: webUIBaseURL,
  };
}

export function createDependencyInstallEnvironment(environment = {}) {
  return {
    ...environment,
    CI: "true",
    pnpm_config_verify_deps_before_run: "false",
  };
}

const commandFailureHintRules = [
  {
    pattern: /ERR_PNPM_(?:OUTDATED_LOCKFILE|LOCKFILE_CONFIG_MISMATCH)/,
    hints: ({ cwd }) => [
      "pnpm 锁文件与 package.json 不一致，通常发生在 SDK 或依赖基线升级之后。",
      `修复：在 ${cwd} 执行 corepack pnpm install --no-frozen-lockfile 更新锁文件。`,
      "若是开发插件 UI 构建失败，在插件目录的 ui/ 子目录执行同一命令后重新启动。",
    ],
  },
];

export function describeCommandFailure(output, { cwd = process.cwd() } = {}) {
  if (!output) {
    return [];
  }
  return commandFailureHintRules
    .filter((rule) => rule.pattern.test(output))
    .flatMap((rule) => rule.hints({ cwd }));
}

export function resolveCorepackCliPath({
  nodeExecutablePath = process.execPath,
  env = process.env,
  platform = process.platform,
  fileExists = existsSync,
} = {}) {
  const pathApi = platform === "win32" ? path.win32 : path.posix;
  const delimiter = platform === "win32" ? ";" : ":";
  const searchDirectories = [
    pathApi.dirname(nodeExecutablePath),
    ...String(env.PATH ?? env.Path ?? "")
      .split(delimiter)
      .map((directory) => stripQuotes(directory.trim()))
      .filter(Boolean),
  ];
  const seen = new Set();
  for (const directory of searchDirectories) {
    const key = platform === "win32" ? directory.toLowerCase() : directory;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    const candidate = pathApi.join(directory, "node_modules", "corepack", "dist", "corepack.js");
    if (fileExists(candidate)) {
      return candidate;
    }
  }

  throw new Error("Corepack CLI was not found next to Node.js or in a Node.js directory on PATH.");
}

export function createTrustedChildEnvironment({
  nodeExecutablePath,
  goExecutablePath,
  env = process.env,
  platform = process.platform,
} = {}) {
  if (!nodeExecutablePath) {
    throw new Error("nodeExecutablePath is required");
  }

  const isWindows = platform === "win32";
  const pathApi = isWindows ? path.win32 : path.posix;
  const delimiter = isWindows ? ";" : ":";
  const pathEntries = [pathApi.dirname(nodeExecutablePath)];
  if (goExecutablePath) {
    pathEntries.push(pathApi.dirname(goExecutablePath));
  }
  const childEnvironment = {};

  if (isWindows) {
    const systemRoot = env.SystemRoot?.trim() || env.WINDIR?.trim();
    if (systemRoot) {
      pathEntries.push(pathApi.join(systemRoot, "System32"), systemRoot);
      childEnvironment.SystemRoot = systemRoot;
      childEnvironment.ComSpec = pathApi.join(systemRoot, "System32", "cmd.exe");
    }
    childEnvironment.PATHEXT = ".COM;.EXE;.BAT;.CMD";
  } else {
    pathEntries.push("/usr/local/bin", "/usr/bin", "/bin");
  }

  const inheritedPathKeys = isWindows
    ? [
        "APPDATA",
        "LOCALAPPDATA",
        "USERPROFILE",
        "HOMEDRIVE",
        "HOMEPATH",
        "GOPATH",
        "GOMODCACHE",
        "TEMP",
        "TMP",
      ]
    : ["HOME", "GOPATH", "GOMODCACHE", "TEMP", "TMP"];
  for (const key of inheritedPathKeys) {
    const value = env[key]?.trim();
    if (value) {
      childEnvironment[key] = value;
    }
  }

  childEnvironment.PATH = uniquePathEntries(pathEntries, isWindows).join(delimiter);
  return childEnvironment;
}

export function createLauncherGoArgs(command, args = [], platform = process.platform) {
  return platform === "linux"
    ? [command, "-tags", "gtk3", ...args]
    : [command, ...args];
}

export async function waitForChildProcessExit(child, {
  timeoutMs = 5_000,
  pollIntervalMs = 50,
  sleep = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds)),
} = {}) {
  if (!child || typeof child !== "object") {
    throw new Error("child process is required");
  }
  const hasExited = () => child.exitCode !== null || child.signalCode !== null;
  const deadline = Date.now() + timeoutMs;
  while (!hasExited() && Date.now() < deadline) {
    await sleep(pollIntervalMs);
  }
  if (!hasExited()) {
    throw new Error("Child process did not exit before the shutdown timeout.");
  }
}

export function isRayleaBotWebDevHtml(text) {
  const body = String(text ?? "");
  return body.includes("<title>RayleaBot Web</title>") && body.includes("/src/main.ts");
}

export async function isTcpPortAvailable(host, port) {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.once("error", () => resolve(false));
    server.once("listening", () => {
      server.close(() => resolve(true));
    });
    server.listen(port, host);
  });
}

export async function classifyWebDevServer({
  url = WEB_DEV_BASE_URL,
  host = "127.0.0.1",
  port = WEB_DEV_PORT,
  backendBaseUrl,
  fetchImpl = globalThis.fetch,
  portAvailable = isTcpPortAvailable,
  projectDir,
  timeoutMs = 1500,
} = {}) {
  if (await portAvailable(host, port)) {
    return "available";
  }

  try {
    const response = await fetchWithTimeout(fetchImpl, url, timeoutMs);
    const body = await response.text();
    if (!isRayleaBotWebDevHtml(body)) {
      return "occupied";
    }
    if (!backendBaseUrl) {
      return "rayleabot";
    }
    return await hasMatchingBackendTarget({ url, backendBaseUrl, projectDir, fetchImpl, timeoutMs })
      ? "rayleabot"
      : "occupied";
  } catch {
    return "occupied";
  }
}

async function hasMatchingBackendTarget({ url, backendBaseUrl, projectDir, fetchImpl, timeoutMs }) {
  try {
    const statusUrl = new URL(WEB_DEV_STATUS_PATH, url).toString();
    const response = await fetchWithTimeout(fetchImpl, statusUrl, timeoutMs);
    if (!response.ok) {
      return false;
    }
    const payload = await response.json();
    return payload?.app === "RayleaBot Web"
      && normalizeComparableUrl(payload?.backendTarget) === normalizeComparableUrl(backendBaseUrl)
      && (!projectDir || (typeof payload.rootDir === "string" && normalizeComparablePath(payload.rootDir, process.platform) === normalizeComparablePath(projectDir, process.platform)));
  } catch {
    return false;
  }
}

function assertKnownProfile(profile) {
  if (!VALID_PROFILES.has(profile)) {
    throw new Error(`Unsupported RAYLEA_START_PROFILE: ${profile}`);
  }
  return profile;
}

function stripYamlComment(line) {
  const match = String(line).match(/^\s*[^#]*/);
  return match?.[0] ?? "";
}

function leadingWhitespace(line) {
  return line.match(/^\s*/)?.[0].length ?? 0;
}

function stripQuotes(value) {
  const trimmed = String(value ?? "").trim();
  if (
    (trimmed.startsWith('"') && trimmed.endsWith('"'))
    || (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

function trimTrailingSlash(value) {
  return value.replace(/\/+$/, "");
}

function normalizeComparableUrl(value) {
  try {
    return trimTrailingSlash(new URL(String(value ?? "")).toString());
  } catch {
    return "";
  }
}

async function fetchWithTimeout(fetchImpl, url, timeoutMs, options = {}) {
  if (typeof fetchImpl !== "function") {
    throw new Error("fetch is unavailable");
  }
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetchImpl(url, { ...options, signal: controller.signal });
  } finally {
    clearTimeout(timeout);
  }
}

function normalizeComparablePath(value, platform) {
  const normalized = path.resolve(String(value ?? ""));
  return platform === "win32" ? normalized.toLowerCase() : normalized;
}

function normalizeLoopbackHTTPURL(value) {
  let parsed;
  try {
    parsed = new URL(String(value ?? ""));
  } catch {
    throw new Error("development server lease backend URL is invalid");
  }
  if (
    parsed.protocol !== "http:"
    || parsed.username
    || parsed.password
    || parsed.pathname !== "/"
    || parsed.search
    || parsed.hash
  ) {
    throw new Error("development server lease backend URL must use loopback HTTP");
  }
  const hostname = parsed.hostname.toLowerCase();
  if (!new Set(["127.0.0.1", "localhost", "[::1]"]).has(hostname)) {
    throw new Error("development server lease backend URL must use a loopback host");
  }
  return trimTrailingSlash(parsed.toString());
}

function uniquePathEntries(entries, caseInsensitive) {
  const seen = new Set();
  return entries.filter((entry) => {
    if (!entry) {
      return false;
    }
    const key = caseInsensitive ? entry.toLowerCase() : entry;
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}
