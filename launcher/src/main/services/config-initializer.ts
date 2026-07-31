import type { LauncherResolvedSettings } from "../../shared/launcher-models";
import type { LauncherConfigInitializer } from "./launcher-coordinator.types";
import { runServerOfflineCommand } from "./reset-admin-runner";

export class NodeConfigInitializer implements LauncherConfigInitializer {
  async run(settings: LauncherResolvedSettings): Promise<void> {
    return runServerOfflineCommand(settings, ["config", "init"], "用户配置初始化失败。");
  }
}
