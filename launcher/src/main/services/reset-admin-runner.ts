import { execFile } from "node:child_process";
import type { LauncherResetAdminRunner } from "./launcher-coordinator.types";
import type { LauncherResolvedSettings } from "../../shared/launcher-models";

export function runServerOfflineCommand(
  settings: LauncherResolvedSettings,
  args: string[],
  failureMessage: string,
): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    execFile(
      settings.serverExecutablePath,
      ["-config", settings.configPath, ...args],
      { timeout: 15000 },
      (error, _stdout, stderr) => {
        if (error) {
          reject(new Error(stderr?.trim() || error.message || failureMessage));
        } else {
          resolve();
        }
      },
    );
  });
}

export class NodeResetAdminRunner implements LauncherResetAdminRunner {
  async run(settings: LauncherResolvedSettings): Promise<void> {
    return runServerOfflineCommand(settings, ["reset-admin"], "管理员重置失败。");
  }
}
