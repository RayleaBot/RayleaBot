// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";
import type { LauncherResolvedSettings, LauncherSnapshot } from "@shared/launcher-models";
import { createLauncherSnapshot } from "../helpers/snapshot";

import { AppShellStatusSection } from "@renderer/AppShellStatusSection";

const snapshot: LauncherSnapshot = createLauncherSnapshot({
  server: {
    health: { status: "ok" },
    readiness: {
      status: "degraded",
      reason: "Python 运行环境元数据不完整。",
      reason_codes: ["platform.resource_missing"],
      checks: {
        runtime: "ok",
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
        scope: "advisory",
        code: "runtime.python_managed_ready",
        title: "Python 运行环境准备",
        severity: "warning",
        summary: "依赖 Python 运行环境的功能暂不可用。",
        detail: "当前平台的 Python 运行环境缺少本地可用资源。",
        remediation: "请联网准备运行环境。",
      },
    ],
    advisoryChecks: [
      {
        scope: "advisory",
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

function renderSection(overrides: Partial<LauncherSnapshot> = {}, resolvedSettings?: LauncherResolvedSettings) {
  return render(
    <AppShellStatusSection
      snapshot={{
        ...snapshot,
        ...overrides,
        server: { ...snapshot.server, ...overrides.server },
        launcher: { ...snapshot.launcher, ...overrides.launcher },
      }}
      resolvedSettings={resolvedSettings ?? snapshot.launcher.resolvedSettings}
      busyAction={null}
      controlsDisabled={false}
      onStart={vi.fn()}
      onStop={vi.fn()}
      onOpenWeb={vi.fn()}
      onOpenRecoveryTasks={vi.fn()}
      onOpenRuntimeTasks={vi.fn()}
      onOpenReleasePage={vi.fn()}
      onOpenLogs={vi.fn()}
    />,
  );
}

describe("AppShellStatusSection", () => {
  test("renders readiness reason, diagnostics, and stderr panel for degraded state", () => {
    renderSection();

    expect(screen.getByText("当前限制")).toBeInTheDocument();
    expect(screen.getAllByText("Python 运行环境元数据不完整。")).toHaveLength(1);
    expect(screen.getByText("服务诊断")).toBeInTheDocument();
    expect(screen.getByText("原因代码")).toBeInTheDocument();
    expect(screen.getAllByText("platform.resource_missing").length).toBeGreaterThan(0);
    expect(screen.getAllByText("请在 .deps/manifest.json 中补齐当前平台 Python 运行环境资源。")).toHaveLength(1);
    expect(screen.queryByText("处理提示")).toBeNull();
    expect(screen.queryByText("当前没有阻塞异常。")).toBeNull();
    expect(screen.getByText("stderr line")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "重启服务" })).toBeInTheDocument();
  });

  test("shows startup guidance instead of constrained wording while starting", () => {
    renderSection({
      server: {
        ...snapshot.server,
        readiness: null,
      },
      launcher: {
        ...snapshot.launcher,
        processLifecycle: "starting",
        statusHint: "正在准备运行环境并等待服务就绪。",
        environmentChecks: [],
        advisoryChecks: [],
        recentStderr: [],
      },
    });

    expect(screen.getByText("运行说明")).toBeInTheDocument();
    expect(screen.getAllByText("正在准备运行环境并等待服务就绪。").length).toBeGreaterThan(0);
    expect(screen.queryByText("当前限制")).toBeNull();
  });

  test("shows runtime preparation panel without duplicate diagnostics while starting", () => {
    renderSection({
      server: {
        ...snapshot.server,
        readiness: {
          ...snapshot.server.readiness!,
          reason: "Python 运行环境元数据不完整。",
          reason_codes: ["platform.resource_missing"],
        },
      },
      launcher: {
        ...snapshot.launcher,
        processLifecycle: "starting",
        statusHint: "正在启动服务。",
        recentStderr: [],
        runtimePrepare: {
          active: true,
          currentKind: "chromium",
          summary: "正在下载 Chromium 浏览环境",
          resources: [
            {
              kind: "chromium",
              label: "Chromium 浏览环境",
              resourceId: "chromium-windows-x64",
              version: "147.0.7727.24",
              sourceLabel: "Chrome for Testing",
              sourceUrl: "https://example.invalid/chrome.zip",
              archivePath: "C:\\RayleaBot\\cache\\downloads\\runtime\\chromium-windows-x64.zip",
              storeRoot: "C:\\RayleaBot\\.deps\\store\\chromium-windows-x64\\147.0.7727.24",
              stage: "download",
              status: "running",
              progress: 42,
              downloadedBytes: 1024,
              totalBytes: 2048,
              extractedEntries: null,
              totalEntries: null,
              summary: "正在下载 Chromium 浏览环境",
              error: "",
              updatedAt: "2026-06-06T00:00:00Z",
            },
          ],
        },
      },
    });

    expect(screen.getByText("准备进度")).toBeInTheDocument();
    expect(screen.getByText("运行环境准备")).toBeInTheDocument();
    expect(screen.getAllByText("正在下载 Chromium 浏览环境").length).toBeGreaterThan(0);
    expect(screen.getByText("Chrome for Testing · https://example.invalid/chrome.zip")).toBeInTheDocument();
    expect(screen.queryByText("当前限制")).toBeNull();
    expect(screen.queryByText("服务诊断")).toBeNull();
    expect(screen.queryByText("Python 运行环境元数据不完整。")).toBeNull();
  });
});
