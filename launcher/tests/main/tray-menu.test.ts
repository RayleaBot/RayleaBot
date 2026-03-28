import { describe, expect, test } from "vitest";
import { buildTrayMenuEntries } from "@main/services/tray-menu";

describe("buildTrayMenuEntries", () => {
  test("uses expected ordering when service is stopped", () => {
    const entries = buildTrayMenuEntries({
      trayStatusSummary: "未启动",
      canOpenWebUi: false,
      trayServiceAction: "start",
      trayServiceActionLabel: "启动服务",
      canRunTrayServiceAction: true,
    });

    expect(entries.map((entry) => `${entry.label}|${entry.enabled}|${entry.action ?? ""}`)).toEqual([
      "RayleaBot 启动器|false|",
      "状态：未启动|false|",
      "|false|separator",
      "恢复窗口|true|restore",
      "打开管理界面|false|open_web",
      "启动服务|true|start",
      "|false|separator",
      "日志目录|true|open_logs",
      "|false|separator",
      "完全退出|true|exit",
    ]);
  });

  test("uses stop action when service is running", () => {
    const entries = buildTrayMenuEntries({
      trayStatusSummary: "运行中",
      canOpenWebUi: true,
      trayServiceAction: "stop",
      trayServiceActionLabel: "停止服务",
      canRunTrayServiceAction: true,
    });

    expect(entries[5]).toMatchObject({
      label: "停止服务",
      enabled: true,
      action: "stop",
    });
  });
});
