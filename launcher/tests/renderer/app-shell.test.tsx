// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";
import { AppShell } from "@renderer/AppShell";
import type { LauncherSnapshot } from "@shared/launcher-models";

const snapshot: LauncherSnapshot = {
  settings: {
    serverExecutablePath: "C:\\RayleaBot\\raylea-server.exe",
    configPath: "C:\\RayleaBot\\config\\user.yaml",
    workdir: "C:\\RayleaBot",
    closeBehavior: "ask_every_time",
  },
  endpoint: {
    host: "127.0.0.1",
    port: 8080,
    baseUrl: "http://127.0.0.1:8080/",
  },
  environmentChecks: [
    {
      code: "config.bootstrap_available",
      title: "用户配置",
      severity: "warning",
      summary: "首次启动时会自动生成用户配置。",
      detail: "缺少用户配置文件。",
      remediation: "启动服务后会基于 default.yaml 生成首份用户配置。",
    },
  ],
  recentStderr: ["stderr line"],
  processId: null,
  serviceState: "stopped",
  shutdownRequested: false,
  serviceDetail: "服务尚未启动。",
  lastError: "",
  releaseCheck: {
    status: "up_to_date",
    currentVersion: "0.1.0",
    latestVersion: "0.1.0",
    summary: "当前版本 0.1.0 已是最新。",
    detail: "",
    releasePageUrl: "https://example.invalid/releases/v0.1.0",
    updateAvailable: false,
  },
};

describe("AppShell", () => {
  test("renders navigation, hero summary, and environment warning", () => {
    render(
      <AppShell
        snapshot={snapshot}
        activeSection="status"
        settingsDraft={snapshot.settings}
        editingSettings={false}
        diagnosticsSummary=""
        busyAction={null}
        controlsDisabled={false}
        onNavigate={vi.fn()}
        onRefresh={vi.fn()}
        onStart={vi.fn()}
        onStop={vi.fn()}
        onOpenWeb={vi.fn()}
        onOpenReleasePage={vi.fn()}
        onOpenLogs={vi.fn()}
        onBeginEdit={vi.fn()}
        onCancelEdit={vi.fn()}
        onSaveSettings={vi.fn()}
        onUpdateSettings={vi.fn()}
        onChooseServer={vi.fn()}
        onChooseConfig={vi.fn()}
        onChooseWorkdir={vi.fn()}
        onExit={vi.fn()}
      />,
    );

    expect(screen.getByText("RayleaBot 启动器")).toBeInTheDocument();
    expect(screen.getByText("状态")).toBeInTheDocument();
    expect(screen.getByText("环境检查")).toBeInTheDocument();
    expect(screen.getByText("日志与诊断")).toBeInTheDocument();
    expect(screen.getByText("设置")).toBeInTheDocument();
    expect(screen.getByText("服务尚未启动。")).toBeInTheDocument();
    expect(screen.getByText("首次启动时会自动生成用户配置。")).toBeInTheDocument();
  });
});
