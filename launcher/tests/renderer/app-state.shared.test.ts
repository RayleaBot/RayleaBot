import { describe, expect, test } from "vitest";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { createLauncherSnapshot } from "../helpers/snapshot";
import { buildDiagnosticsSummary } from "@renderer/AppState.shared";

const snapshot: LauncherSnapshot = createLauncherSnapshot({
  server: {
    health: { status: "ok" },
    readiness: {
      status: "degraded",
      reason: "Python 运行环境元数据不完整。",
      reason_codes: ["platform.resource_missing"],
      checks: {
        runtime: "resource_missing",
        render: "ok",
      },
      issues: [
        {
          code: "platform.resource_missing",
          severity: "warning",
          summary: "Python 运行环境元数据不完整。",
          remediation: "请在 .deps/manifest.json 中补齐当前平台 Python 运行环境资源。",
        },
      ],
    },
  },
  launcher: {
    settings: {
      installationRoot: "C:\\RayleaBot",
      closeBehavior: "ask_every_time",
    },
    resolvedSettings: {
      installationRoot: "C:\\RayleaBot",
      serverExecutablePath: "C:\\RayleaBot\\raylea-server.exe",
      configPath: "C:\\RayleaBot\\config\\user.yaml",
      workdir: "C:\\RayleaBot",
    },
    processLifecycle: "running",
    processOwnership: "launcher_managed",
    releaseCheck: {
      status: "up_to_date",
      currentVersion: "0.1.0",
      latestVersion: "0.1.0",
      summary: "当前版本 0.1.0 已是最新。",
      detail: "",
      releasePageUrl: "https://example.invalid/releases/v0.1.0",
      updateAvailable: false,
    },
  },
});

describe("buildDiagnosticsSummary", () => {
  test("includes readiness reason, codes, issues, and checks", () => {
    const summary = buildDiagnosticsSummary(snapshot);

    expect(summary).toContain("服务状态：");
    expect(summary).toContain("服务连接：可连接");
    expect(summary).toContain("就绪状态：部分功能受限");
    expect(summary).toContain("原因：Python 运行环境元数据不完整。");
    expect(summary).toContain("原因代码：platform.resource_missing");
    expect(summary).toContain("platform.resource_missing：Python 运行环境元数据不完整。");
    expect(summary).toContain("运行环境：资源缺失");
    expect(summary).toContain("进程状态：运行中");
    expect(summary).toContain("进程来源：由启动器启动");
  });
});
