import { spawn } from "node:child_process";
import process from "node:process";
import { waitForChildProcessExit } from "../../scripts/start-dev-support.mjs";

function runTreeKiller(pid, spawnProcess) {
  return new Promise((resolve, reject) => {
    const killer = spawnProcess("taskkill", ["/PID", String(pid), "/T", "/F"], {
      stdio: "ignore",
      windowsHide: true,
    });
    killer.once("exit", (code) => {
      if (code === 0) {
        resolve();
        return;
      }
      reject(new Error(`taskkill exited with code ${code ?? "unknown"}`));
    });
    killer.once("error", reject);
  });
}

export function normalizeChildExitCode(code, signal) {
  if (typeof code === "number") {
    return code;
  }
  return signal ? 1 : 0;
}

export async function terminateDevProcessTree(child, {
  platform = process.platform,
  spawnProcess = spawn,
  killProcess = process.kill,
  waitForExit = waitForChildProcessExit,
} = {}) {
  if (!child?.pid || child.exitCode !== null || child.signalCode !== null) {
    return;
  }

  if (platform === "win32") {
    await runTreeKiller(child.pid, spawnProcess);
    await waitForExit(child);
    return;
  }

  const processGroup = -child.pid;
  try {
    killProcess(processGroup, "SIGTERM");
  } catch (error) {
    if (error?.code === "ESRCH" || child.exitCode !== null || child.signalCode !== null) {
      return;
    }
    throw error;
  }
  try {
    await waitForExit(child);
    return;
  } catch {
    try {
      killProcess(processGroup, "SIGKILL");
    } catch (error) {
      if (error?.code === "ESRCH" || child.exitCode !== null || child.signalCode !== null) {
        return;
      }
      throw error;
    }
    await waitForExit(child);
    if (child.exitCode !== null || child.signalCode !== null) {
      return;
    }
  }
}
