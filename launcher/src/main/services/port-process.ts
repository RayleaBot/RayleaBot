import { execFile } from "node:child_process";
import { promisify } from "node:util";
import type { ServerEndpoint } from "../../shared/launcher-models";
import { terminateProcessId } from "./process-termination";

const execFileAsync = promisify(execFile);

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

async function resolveListeningPid(endpoint: ServerEndpoint) {
  if (process.platform === "win32") {
    const { stdout } = await execFileAsync("netstat", ["-ano", "-p", "tcp"]);
    return parseWindowsPid(stdout, endpoint.port);
  }

  try {
    const { stdout } = await execFileAsync("lsof", ["-iTCP:" + endpoint.port, "-sTCP:LISTEN", "-t"]);
    const pid = Number.parseInt(stdout.trim().split(/\r?\n/)[0] ?? "", 10);
    return Number.isNaN(pid) ? null : pid;
  } catch {
    return null;
  }
}

export async function isEndpointListening(endpoint: ServerEndpoint) {
  return (await resolveListeningPid(endpoint)) !== null;
}

export async function tryStopEndpointProcess(endpoint: ServerEndpoint) {
  const pid = await resolveListeningPid(endpoint);
  if (!pid) {
    return false;
  }
  return terminateProcessId(pid);
}
