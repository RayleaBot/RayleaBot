import { getLauncherStateLabel, type LauncherPresentationState } from "./launcher-presentation";
import type { CheckSeverity, LauncherCloseBehavior } from "./launcher-models";

export const launcherCopy = {
  trayTitleLabel: "RayleaBot 启动器",
  restoreLauncherLabel: "恢复窗口",
  openWebUiLabel: "打开管理界面",
  trayOpenLogsLabel: "日志目录",
  exitAppLabel: "完全退出",
  statusSummary(state: LauncherPresentationState) {
    return getLauncherStateLabel(state);
  },
  severityLabel(severity: CheckSeverity) {
    switch (severity) {
      case "ok":
        return "正常";
      case "warning":
        return "需注意";
      default:
        return "阻塞";
    }
  },
  closeBehaviorTitle(closeBehavior: LauncherCloseBehavior) {
    switch (closeBehavior) {
      case "hide_to_tray":
        return "隐藏到托盘";
      case "exit_application":
        return "完全退出";
      default:
        return "每次询问";
    }
  },
};

export function createReleaseUnavailable(detail = "当前运行没有可读取的版本包信息。") {
  return {
    status: "unavailable",
    currentVersion: "",
    latestVersion: "",
    summary: "版本信息不可用",
    detail,
    releasePageUrl: "",
    updateAvailable: false,
  };
}
