import fs from "node:fs/promises";
import net from "node:net";
import path from "node:path";

export const WEB_DEV_PROFILE = "web-dev";
export const BUILD_PROFILE = "build";
export const LAUNCHER_DEV_PROFILE = "launcher-dev";
export const SERVER_RELOAD_AIR = "air";
export const WEB_DEV_PORT = 4173;
export const WEB_DEV_BASE_URL = `http://127.0.0.1:${WEB_DEV_PORT}/`;
export const WEB_DEV_STATUS_PATH = "/__rayleabot-dev/status";

const VALID_PROFILES = new Set([WEB_DEV_PROFILE, BUILD_PROFILE, LAUNCHER_DEV_PROFILE]);
const VALID_INSTALL_MODES = new Set(["auto", "always", "skip"]);
const VALID_SERVER_RELOAD_MODES = new Set(["", SERVER_RELOAD_AIR]);
const WILDCARD_HOSTS = new Set(["", "*", "0.0.0.0", "::", "[::]"]);
const INSTALL_MARKER_NAME = ".rayleabot-start-install.stamp";

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
  return mode;
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
    RAYLEA_WEB_UI_BASE_URL: env.RAYLEA_WEB_UI_BASE_URL?.trim() || webBaseUrl,
  };
}

export async function shouldInstallDependencies({
  projectDir,
  lockfileName = "pnpm-lock.yaml",
  markerName = INSTALL_MARKER_NAME,
  mode = "auto",
  stat = fs.stat,
} = {}) {
  if (mode === "always") {
    return true;
  }
  if (mode === "skip") {
    return false;
  }
  if (mode !== "auto") {
    throw new Error(`Unsupported install mode: ${mode}`);
  }
  if (!projectDir) {
    throw new Error("projectDir is required");
  }

  const nodeModulesPath = path.join(projectDir, "node_modules");
  const lockfilePath = path.join(projectDir, lockfileName);
  const markerPath = path.join(nodeModulesPath, markerName);

  const nodeModulesStat = await statOrNull(stat, nodeModulesPath);
  if (!nodeModulesStat?.isDirectory()) {
    return true;
  }

  const lockfileStat = await statOrNull(stat, lockfilePath);
  if (!lockfileStat?.isFile()) {
    return false;
  }

  const markerStat = await statOrNull(stat, markerPath);
  if (!markerStat?.isFile()) {
    return true;
  }

  return lockfileStat.mtimeMs > markerStat.mtimeMs;
}

export async function markDependenciesInstalled({
  projectDir,
  markerName = INSTALL_MARKER_NAME,
  writeFile = fs.writeFile,
  mkdir = fs.mkdir,
} = {}) {
  const nodeModulesPath = path.join(projectDir, "node_modules");
  await mkdir(nodeModulesPath, { recursive: true });
  await writeFile(path.join(nodeModulesPath, markerName), `${new Date().toISOString()}\n`, "utf8");
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
  timeoutMs = 1500,
} = {}) {
  if (await isTcpPortAvailable(host, port)) {
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
    return await hasMatchingBackendTarget({ url, backendBaseUrl, fetchImpl, timeoutMs })
      ? "rayleabot"
      : "occupied";
  } catch {
    return "occupied";
  }
}

async function hasMatchingBackendTarget({ url, backendBaseUrl, fetchImpl, timeoutMs }) {
  try {
    const statusUrl = new URL(WEB_DEV_STATUS_PATH, url).toString();
    const response = await fetchWithTimeout(fetchImpl, statusUrl, timeoutMs);
    if (!response.ok) {
      return false;
    }
    const payload = await response.json();
    return payload?.app === "RayleaBot Web"
      && normalizeComparableUrl(payload?.backendTarget) === normalizeComparableUrl(backendBaseUrl);
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

async function statOrNull(stat, targetPath) {
  try {
    return await stat(targetPath);
  } catch (error) {
    if (error?.code === "ENOENT") {
      return null;
    }
    throw error;
  }
}

async function fetchWithTimeout(fetchImpl, url, timeoutMs) {
  if (typeof fetchImpl !== "function") {
    throw new Error("fetch is unavailable");
  }
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetchImpl(url, { signal: controller.signal });
  } finally {
    clearTimeout(timeout);
  }
}
