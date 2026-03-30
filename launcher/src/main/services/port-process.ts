import { execFile } from "node:child_process";
import path from "node:path";
import { promisify } from "node:util";
import type { ServerEndpoint } from "../../shared/launcher-models";
import { terminateProcessId } from "./process-termination";

const execFileAsync = promisify(execFile);

type ExecFileLike = (
  file: string,
  args: string[],
) => Promise<{ stdout: string; stderr: string }>;

interface PortProcessDependencies {
  platform?: NodeJS.Platform;
  execFileAsync?: ExecFileLike;
  terminateProcessId?: (pid: number) => Promise<boolean>;
}

function parseWindowsPid(output: string, port: number) {
  for (const rawLine of output.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line.startsWith("TCP") || !line.includes("LISTENING")) {
      continue;
    }
    const columns = line.split(/\s+/);
    const localAddress = columns[1] ?? "";
    const localPort = Number.parseInt(localAddress.split(":").at(-1) ?? "", 10);
    const pid = Number.parseInt(columns.at(-1) ?? "", 10);
    if (localPort === port && !Number.isNaN(pid)) {
      return pid;
    }
  }
  return null;
}

function parseWindowsProcessName(output: string) {
  const line = output
    .split(/\r?\n/)
    .map((entry) => entry.trim())
    .find((entry) => entry && !entry.startsWith("INFO:"));

  if (!line) {
    return null;
  }

  const match = line.match(/^"([^"]+)"/);
  return match?.[1] ?? null;
}

function normalizeProcessName(command: string) {
  return path.basename(command.trim().replace(/^['"]|['"]$/g, "")).toLowerCase();
}

function isRayleaServerProcess(command: string) {
  const normalized = normalizeProcessName(command);
  return normalized === "raylea-server" || normalized === "raylea-server.exe";
}

async function resolveListeningPid(endpoint: ServerEndpoint, deps: PortProcessDependencies = {}) {
  const platform = deps.platform ?? process.platform;
  const runCommand = deps.execFileAsync ?? execFileAsync;

  if (platform === "win32") {
    const { stdout } = await runCommand("netstat", ["-ano", "-p", "tcp"]);
    return parseWindowsPid(stdout, endpoint.port);
  }

  try {
    const { stdout } = await runCommand("lsof", ["-iTCP:" + endpoint.port, "-sTCP:LISTEN", "-t"]);
    const pid = Number.parseInt(stdout.trim().split(/\r?\n/)[0] ?? "", 10);
    return Number.isNaN(pid) ? null : pid;
  } catch {
    return null;
  }
}

async function resolveProcessCommand(pid: number, deps: PortProcessDependencies = {}) {
  const platform = deps.platform ?? process.platform;
  const runCommand = deps.execFileAsync ?? execFileAsync;

  try {
    if (platform === "win32") {
      const { stdout } = await runCommand("tasklist", ["/FI", `PID eq ${pid}`, "/FO", "CSV", "/NH"]);
      return parseWindowsProcessName(stdout);
    }

    const { stdout } = await runCommand("ps", ["-p", String(pid), "-o", "comm="]);
    const command = stdout
      .split(/\r?\n/)
      .map((entry) => entry.trim())
      .find(Boolean);
    return command ?? null;
  } catch {
    return null;
  }
}

export async function isEndpointListening(endpoint: ServerEndpoint, deps: PortProcessDependencies = {}) {
  return (await resolveListeningPid(endpoint, deps)) !== null;
}

export async function tryStopEndpointProcess(endpoint: ServerEndpoint, deps: PortProcessDependencies = {}) {
  const pid = await resolveListeningPid(endpoint, deps);
  if (!pid) {
    return false;
  }

  const command = await resolveProcessCommand(pid, deps);
  if (!command || !isRayleaServerProcess(command)) {
    return false;
  }

  return (deps.terminateProcessId ?? terminateProcessId)(pid);
}
