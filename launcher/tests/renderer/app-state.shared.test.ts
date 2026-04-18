import { describe, expect, test } from "vitest";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { buildDiagnosticsSummary } from "@renderer/AppState.shared";

const snapshot: LauncherSnapshot = {
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
  endpoint: {
    host: "127.0.0.1",
    port: 8080,
    baseUrl: "http://127.0.0.1:8080/",
  },
  environmentChecks: [],
  recentStderr: [],
  processId: null,
  serviceState: "degraded",
  serviceOwnership: "launcher_managed",
  shutdownRequested: false,
  serviceDetail: "运行条件受限。",
  lastError: "",
  readiness: {
    status: "degraded",
    reason: "OneBot 正在建立连接",
    reason_codes: ["adapter.connection_pending"],
    checks: {
      adapter: "connecting",
    },
    issues: [
      {
        code: "adapter.connection_pending",
        severity: "warning",
        summary: "OneBot 正在建立连接",
        remediation: "请稍后重试，或检查上游服务是否可达。",
      },
    ],
  },
  releaseCheck: {
    status: "up_to_date",
    currentVersion: "0.1.0",
    latestVersion: "0.1.0",
    summary: "当前版本 0.1.0 已是最新。",
    detail: "",
    releasePageUrl: "https://example.invalid/releases/v0.1.0",
    updateAvailable: false,
  },
  recoverySummary: null,
};

describe("buildDiagnosticsSummary", () => {
  test("includes readiness reason, codes, issues, and checks", () => {
    const summary = buildDiagnosticsSummary(snapshot);

    expect(summary).toContain("服务就绪：");
    expect(summary).toContain("状态：degraded");
    expect(summary).toContain("原因：OneBot 正在建立连接");
    expect(summary).toContain("原因代码：adapter.connection_pending");
    expect(summary).toContain("adapter.connection_pending：OneBot 正在建立连接");
    expect(summary).toContain("adapter：connecting");
  });
});
