import { execFile } from "node:child_process";
import type { LauncherResetAdminRunner } from "./launcher-coordinator.types";
import type { LauncherResolvedSettings } from "../../shared/launcher-models";
import { resolveConfigSchemaPath } from "./process-controller";

export class NodeResetAdminRunner implements LauncherResetAdminRunner {
  async run(settings: LauncherResolvedSettings): Promise<void> {
    const serverPath = settings.serverExecutablePath;
    const configPath = settings.configPath;
    const schemaPath = await resolveConfigSchemaPath(settings);

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
