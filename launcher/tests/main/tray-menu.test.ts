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

    expect(entries.map((entry) => `${entry.enabled}|${entry.action ?? ""}`)).toEqual([
      "false|",
      "false|",
      "false|separator",
      "true|restore",
      "false|open_web",
      "true|start",
      "false|separator",
      "true|open_logs",
      "false|separator",
      "true|exit",
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
      enabled: true,
      action: "stop",
    });
  });
});
