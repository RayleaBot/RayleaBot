import { execFile } from "node:child_process";
import type { LauncherResetAdminRunner } from "./launcher-coordinator.types";
import type { LauncherResolvedSettings } from "../../shared/launcher-models";

export class NodeResetAdminRunner implements LauncherResetAdminRunner {
  async run(settings: LauncherResolvedSettings): Promise<void> {
    const serverPath = settings.serverExecutablePath;
    const configPath = settings.configPath;

    return new Promise<void>((resolve, reject) => {
      execFile(
        serverPath,
        ["-config", configPath, "reset-admin"],
        { timeout: 15000 },
        (error, _stdout, stderr) => {
          if (error) {
            reject(new Error(stderr?.trim() || error.message || "管理员重置失败。"));
          } else {
            resolve();
          }
        },
      );
    });
  }
}
