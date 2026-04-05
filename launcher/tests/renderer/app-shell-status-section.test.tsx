// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";
import type { LauncherResolvedSettings, LauncherSnapshot } from "@shared/launcher-models";

import { AppShellStatusSection } from "@renderer/AppShellStatusSection";

const snapshot: LauncherSnapshot = {
  settings: {
    installationRoot: "C:\\RayleaBot",
    closeBehavior: "ask_every_time",
  },
  resolvedSettings: {
    installationRoot: "C:\\RayleaBot",
    serverExecutablePath: "C:\\RayleaBot\\server\\raylea-server.exe",
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
      code: "runtime.python_managed_ready",
      title: "Python 运行环境准备",
      severity: "warning",
      summary: "依赖 Python 运行环境的功能暂不可用。",
      detail: "当前平台的 Python 运行环境缺少本地可用资源。",
      remediation: "请联网准备运行环境。",
    },
  ],
  recentStderr: ["stderr line"],
  processId: 4242,
  serviceState: "degraded",
  serviceOwnership: "launcher_managed",
  shutdownRequested: false,
  serviceDetail: "Python 运行环境尚未准备完成。",
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
  recoverySummary: null,
};

function renderSection(overrides: Partial<LauncherSnapshot> = {}, resolvedSettings?: LauncherResolvedSettings) {
  return render(
    <AppShellStatusSection
      snapshot={{ ...snapshot, ...overrides }}
      resolvedSettings={resolvedSettings ?? snapshot.resolvedSettings}
      busyAction={null}
      controlsDisabled={false}
      onStart={vi.fn()}
      onStop={vi.fn()}
      onOpenWeb={vi.fn()}
      onRecoveryRecheck={vi.fn()}
      onRuntimeBootstrap={vi.fn()}
      onOpenReleasePage={vi.fn()}
      onOpenLogs={vi.fn()}
    />,
  );
}

describe("AppShellStatusSection", () => {
  test("renders degraded status reason, remediation rail, and stderr panel", () => {
    renderSection();

    expect(screen.getByText("当前限制")).toBeInTheDocument();
    expect(screen.getAllByText("Python 运行环境尚未准备完成。").length).toBeGreaterThan(0);
    expect(screen.getByText("请联网准备运行环境。")).toBeInTheDocument();
    expect(screen.getByText("stderr line")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "重启服务" })).toBeInTheDocument();
  });

  test("shows startup guidance instead of constrained wording while starting", () => {
    renderSection({
      serviceState: "starting",
      serviceDetail: "正在准备运行环境并等待服务就绪。",
      environmentChecks: [],
      recentStderr: [],
    });

    expect(screen.getByText("运行说明")).toBeInTheDocument();
    expect(screen.getAllByText("正在准备运行环境并等待服务就绪。").length).toBeGreaterThan(0);
    expect(screen.queryByText("当前限制")).toBeNull();
  });
});
