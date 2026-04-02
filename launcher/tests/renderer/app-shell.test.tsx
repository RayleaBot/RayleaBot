// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";
import { AppShell } from "@renderer/AppShell";
import type { LauncherSnapshot } from "@shared/launcher-models";

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
  recoverySummary: {
    status: "degraded",
    phase: "post_startup",
    operation: "upgrade",
    created_at: "2026-04-02T08:00:00Z",
    updated_at: "2026-04-02T08:01:00Z",
    manual_actions: ["处理被跳过插件的兼容性问题后，再在管理面中手动重新启用。"],
  },
};

describe("AppShell", () => {
  test("renders navigation, hero summary, and environment warning", () => {
    render(
      <AppShell
        snapshot={snapshot}
        activeSection="status"
        settingsDraft={snapshot.settings}
        resolvedSettings={snapshot.resolvedSettings}
        editingSettings={false}
        diagnosticsSummary=""
        busyAction={null}
        controlsDisabled={false}
        isMaximized={false}
        onNavigate={vi.fn()}
        onRefresh={vi.fn()}
        onStart={vi.fn()}
        onStop={vi.fn()}
        onOpenWeb={vi.fn()}
        onOpenReleasePage={vi.fn()}
        onOpenLogs={vi.fn()}
        onResetAdmin={vi.fn()}
        onBeginEdit={vi.fn()}
        onCancelEdit={vi.fn()}
        onSaveSettings={vi.fn()}
        onUpdateInstallationRoot={vi.fn()}
        onUpdateCloseBehavior={vi.fn()}
        onUpdateAdvancedOverride={vi.fn()}
        onChooseInstallationRoot={vi.fn()}
        onChooseServer={vi.fn()}
        onChooseConfig={vi.fn()}
        onChooseWorkdir={vi.fn()}
        onExit={vi.fn()}
      />,
    );

    expect(screen.getByText("RayleaBot")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "RayleaLauncher" })).toBeInTheDocument();
    expect(screen.getByText("运行状态")).toBeInTheDocument();
    expect(screen.getByText("环境检查")).toBeInTheDocument();
    expect(screen.getByText("日志诊断")).toBeInTheDocument();
    expect(screen.getByText("偏好设置")).toBeInTheDocument();
    expect(screen.getByText("服务尚未启动。")).toBeInTheDocument();
    expect(screen.getByText("首次启动时会自动生成用户配置。")).toBeInTheDocument();
    expect(screen.getByText(/恢复兼容性/)).toBeInTheDocument();
  });
});
