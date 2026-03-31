import { execFile } from "node:child_process";
import path from "node:path";
import type { LauncherResetAdminRunner } from "./launcher-coordinator";
import type { LauncherSettings } from "../../shared/launcher-models";

export class NodeResetAdminRunner implements LauncherResetAdminRunner {
  async run(settings: LauncherSettings): Promise<void> {
    const serverPath = settings.serverExecutablePath;
    const configPath = settings.configPath;
    const schemaPath = path.resolve(path.dirname(configPath), "..", "contracts", "config.user.schema.json");

    return new Promise<void>((resolve, reject) => {
      execFile(
        serverPath,
        ["reset-admin", "-config", configPath, "-config-schema", schemaPath],
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
