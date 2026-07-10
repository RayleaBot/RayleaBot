import { spawn } from "node:child_process";
import type { Dirent } from "node:fs";
import fs from "node:fs/promises";
import path from "node:path";
import type { LauncherSnapshot } from "../../shared/launcher-models";

interface UpdateHeartbeatRequest {
  heartbeatPath: string;
  token: string;
  restartService: boolean;
}

interface BuildInfo {
  version: string;
  artifact_id: string;
  update_protocol_version: number;
}

interface RecoveryJournal {
  version: number;
  state: string;
  phase: string;
  install_root: string;
  transaction_root: string;
}

export interface UpdateHeartbeatDependencies {
  startService(): Promise<void>;
  getSnapshot(): LauncherSnapshot;
  launcherPid: number;
  servicePid(): number | null;
}

export interface UpdateRecoveryLauncher {
  spawnDetached(executable: string, args: string[]): Promise<void>;
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function parseBoolean(value: string) {
  return value.trim().toLowerCase() === "true";
}

function samePath(left: string, right: string) {
  return path.resolve(left).toLowerCase() === path.resolve(right).toLowerCase();
}

function isTransactionSibling(installRoot: string, transactionRoot: string) {
  const installParent = path.dirname(path.resolve(installRoot));
  const transaction = path.resolve(transactionRoot);
  return samePath(path.dirname(transaction), installParent)
    && path.basename(transaction).startsWith(".rayleabot-update-");
}

async function regularFile(filePath: string) {
  try {
    const info = await fs.lstat(filePath);
    return info.isFile() && !info.isSymbolicLink();
  } catch {
    return false;
  }
}

export function consumeUpdateHeartbeatEnvironment(
  environment: NodeJS.ProcessEnv = process.env,
): UpdateHeartbeatRequest | null {
  const heartbeatPath = stringValue(environment.RAYLEA_UPDATE_HEARTBEAT);
  const token = stringValue(environment.RAYLEA_UPDATE_TOKEN);
  const restartService = parseBoolean(stringValue(environment.RAYLEA_UPDATE_RESTART_SERVICE));
  delete environment.RAYLEA_UPDATE_HEARTBEAT;
  delete environment.RAYLEA_UPDATE_TOKEN;
  delete environment.RAYLEA_UPDATE_RESTART_SERVICE;
  if (!heartbeatPath && !token) {
    return null;
  }
  if (!heartbeatPath || !/^[0-9a-f]{64}$/i.test(token)) {
    return null;
  }
  return { heartbeatPath, token, restartService };
}

export async function completeUpdateHeartbeat(
  installRoot: string,
  request: UpdateHeartbeatRequest | null,
  dependencies: UpdateHeartbeatDependencies,
) {
  if (!request) {
    return;
  }
  const transactionRoot = path.dirname(path.resolve(request.heartbeatPath));
  if (
    !isTransactionSibling(installRoot, transactionRoot)
    || !["postflight-launcher-heartbeat.json", "rollback-launcher-heartbeat.json"].includes(path.basename(request.heartbeatPath))
  ) {
    return;
  }
  let buildInfo: BuildInfo | null = null;
  let failure = "";
  try {
    const parsed = JSON.parse(await fs.readFile(path.join(installRoot, "build_info.json"), "utf8")) as Record<string, unknown>;
    const version = stringValue(parsed.version);
    const artifactId = stringValue(parsed.artifact_id);
    const protocolVersion = typeof parsed.update_protocol_version === "number" ? parsed.update_protocol_version : 0;
    if (!version || !artifactId || protocolVersion < 2) {
      throw new Error("构建信息不包含可信更新基线。");
    }
    buildInfo = { version, artifact_id: artifactId, update_protocol_version: protocolVersion };
    if (request.restartService) {
      await dependencies.startService();
      const snapshot = dependencies.getSnapshot();
      if (
        !dependencies.servicePid()
        || snapshot.launcher.processLifecycle !== "running"
        || snapshot.server.health?.status !== "ok"
        || snapshot.server.readiness?.status !== "ready"
      ) {
        throw new Error("服务未通过 healthz 和 readyz 检查。");
      }
    }
  } catch (error) {
    failure = error instanceof Error ? error.message : "Launcher 启动后检查失败。";
  }

  const payload = {
    token: request.token,
    status: failure ? "failed" : "ready",
    version: buildInfo?.version ?? "",
    artifact_id: buildInfo?.artifact_id ?? "",
    launcher_pid: dependencies.launcherPid,
    service_pid: dependencies.servicePid() ?? undefined,
    service_running: request.restartService && !failure,
    error: failure || undefined,
  };
  await writeJSONAtomically(request.heartbeatPath, payload);
}

async function writeJSONAtomically(destination: string, value: unknown) {
  const parent = path.dirname(destination);
  const temporary = path.join(parent, `.${path.basename(destination)}-${process.pid}-${Date.now()}.tmp`);
  const handle = await fs.open(temporary, "wx", 0o600);
  try {
    await handle.writeFile(`${JSON.stringify(value)}\n`, "utf8");
    await handle.sync();
  } finally {
    await handle.close();
  }
  try {
    await fs.rename(temporary, destination);
  } catch (error) {
    await fs.rm(temporary, { force: true });
    throw error;
  }
}

export async function launchInterruptedUpdateRecovery(
  installRoot: string,
  launcherPid: number,
  recoveryLauncher: UpdateRecoveryLauncher = {
    spawnDetached(executable, args) {
      return new Promise((resolve, reject) => {
        const child = spawn(executable, args, { detached: true, stdio: "ignore", windowsHide: true });
        child.once("spawn", () => {
          child.unref();
          resolve();
        });
        child.once("error", reject);
      });
    },
  },
) {
  const parent = path.dirname(path.resolve(installRoot));
  let entries: Dirent[];
  try {
    entries = await fs.readdir(parent, { withFileTypes: true });
  } catch {
    return false;
  }
  const candidates: Array<{ root: string; modifiedAt: number }> = [];
  for (const entry of entries) {
    if (!entry.isDirectory() || !entry.name.startsWith(".rayleabot-update-")) {
      continue;
    }
    const root = path.join(parent, entry.name);
    try {
      const journalPath = path.join(root, "journal.json");
      const journal = JSON.parse(await fs.readFile(journalPath, "utf8")) as RecoveryJournal;
      if (
        journal.version !== 1
        || ["succeeded", "rolled_back", "rollback_failed"].includes(journal.state)
        || !samePath(journal.install_root, installRoot)
        || !samePath(journal.transaction_root, root)
        || !isTransactionSibling(installRoot, root)
      ) {
        continue;
      }
      const helper = path.join(root, "raylea-updater.exe");
      if (!await regularFile(helper)) {
        continue;
      }
      const stat = await fs.stat(journalPath);
      candidates.push({ root, modifiedAt: stat.mtimeMs });
    } catch {
      // Invalid or incomplete transaction directories are never executed.
    }
  }
  candidates.sort((left, right) => right.modifiedAt - left.modifiedAt);
  const candidate = candidates[0];
  if (!candidate) {
    return false;
  }
  try {
    await recoveryLauncher.spawnDetached(path.join(candidate.root, "raylea-updater.exe"), [
      "recover",
      "--transaction-root", candidate.root,
      "--launcher-pid", String(launcherPid),
    ]);
  } catch {
    return false;
  }
  return true;
}
