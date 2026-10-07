// @vitest-environment jsdom
import { mount, type VueWrapper } from "@vue/test-utils";
import { afterEach, describe, expect, test, vi } from "vitest";
import LauncherRoot from "@renderer/LauncherRoot.vue";
import type { LauncherSnapshot } from "@shared/launcher-models";

import { createDesktopApi, installDesktopApi } from "../helpers/desktop-api";
import { findButton, findDialog, getButton, getTextbox, hasText, queryButton, queryDialog } from "../helpers/dom";
import { createLauncherSnapshot } from "../helpers/snapshot";

const TEST_INSTALLATION_ROOT = "C:\\RayleaBotTest\\workspace";

const loadedSnapshot: LauncherSnapshot = createLauncherSnapshot({
  launcher: {
    settings: {
      installationRoot: TEST_INSTALLATION_ROOT,
      closeBehavior: "ask_every_time",
    },
    resolvedSettings: {
      installationRoot: TEST_INSTALLATION_ROOT,
      serverExecutablePath: `${TEST_INSTALLATION_ROOT}\\server\\raylea-server.exe`,
      configPath: `${TEST_INSTALLATION_ROOT}\\config\\user.yaml`,
      workdir: TEST_INSTALLATION_ROOT,
    },
  },
});

const runningManagedSnapshot = createLauncherSnapshot({
  server: {
    health: { status: "ok" },
    readiness: { status: "ready" },
  },
  launcher: {
    ...loadedSnapshot.launcher,
    processId: 4242,
    processLifecycle: "running",
    processOwnership: "launcher_managed",
  },
});

const runningExternalSnapshot = createLauncherSnapshot({
  server: {
    health: { status: "ok" },
    readiness: { status: "ready" },
  },
  launcher: {
    ...loadedSnapshot.launcher,
    processId: null,
    processOwnership: "external",
  },
});

const setupRequiredSnapshot = createLauncherSnapshot({
  server: {
    health: { status: "ok" },
    readiness: {
      status: "setup_required",
      reason: "需要先完成管理员初始化",
      reason_codes: ["setup.required"],
      issues: [
        {
          code: "setup.required",
          severity: "error",
          summary: "需要先完成管理员初始化",
          remediation: "请先完成管理员初始化，然后再使用管理入口。",
        },
      ],
    },
  },
  launcher: {
    ...loadedSnapshot.launcher,
    processId: 4242,
    processLifecycle: "running",
    processOwnership: "launcher_managed",
  },
});

let wrapper: VueWrapper | undefined;

function mountLauncher(api: Parameters<typeof createDesktopApi>[0]) {
  const desktopApi = createDesktopApi(api);
  installDesktopApi(desktopApi);
  wrapper = mount(LauncherRoot, { attachTo: document.body });
  return desktopApi;
}

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = "";
  Reflect.deleteProperty(window, "rayleaLauncher");
});

describe("App", () => {
  test("restarts the managed service through its secondary restart action", async () => {
    const api = mountLauncher({ getSnapshot: vi.fn(async () => runningManagedSnapshot) });

    (await findButton("重启服务")).click();

    await vi.waitFor(() => expect(api.restart).toHaveBeenCalledOnce());
    expect(api.start).not.toHaveBeenCalled();
    expect(api.stop).not.toHaveBeenCalled();
  });

  test("opens management for an external service without offering start or restart", async () => {
    mountLauncher({ getSnapshot: vi.fn(async () => runningExternalSnapshot) });

    const managementButton = await findButton("管理界面");
    expect(managementButton.disabled).toBe(false);
    expect(queryButton("启动服务")).toBeNull();
    expect(getButton("重启服务").disabled).toBe(true);
  });

  // The Launcher cannot stop a service another process started, so it hands the stop to that service's console.
  test("sends the stop of an external service to its management console", async () => {
    const externalWithConsole = createLauncherSnapshot({
      ...runningExternalSnapshot,
      launcher: { ...runningExternalSnapshot.launcher, controlCapability: "open_web" },
    });
    const api = mountLauncher({ getSnapshot: vi.fn(async () => externalWithConsole) });

    (await findButton("在管理界面停止")).click();

    await vi.waitFor(() => expect(api.openWebUi).toHaveBeenCalledOnce());
    expect(api.stop).not.toHaveBeenCalled();
    expect(queryButton("停止服务")).toBeNull();
  });

  test("confirms before starting a one-click update", async () => {
    const updateSnapshot = createLauncherSnapshot({
      launcher: {
        ...loadedSnapshot.launcher,
        releaseCheck: {
          status: "update_available",
          currentVersion: "0.3.0",
          latestVersion: "0.4.0",
          releasePageUrl: "https://example.invalid/releases/v0.4.0",
          updateAvailable: true,
          canCheck: true,
        },
      },
    });
    const api = mountLauncher({ getSnapshot: vi.fn(async () => updateSnapshot) });

    (await findButton("关于应用")).click();
    (await findButton("立即更新")).click();
    const dialog = await findDialog("安装更新");
    expect(api.applyUpdate).not.toHaveBeenCalled();
    getButton("安装更新", dialog).click();

    await vi.waitFor(() => expect(api.applyUpdate).toHaveBeenCalledOnce());
  });

  test("shows first-run setup as waiting for an administrator and opens the management UI", async () => {
    const api = mountLauncher({ getSnapshot: vi.fn(async () => setupRequiredSnapshot) });

    const managementButton = await findButton("管理界面");
    expect(managementButton.disabled).toBe(false);
    managementButton.click();

    await vi.waitFor(() => expect(api.openWebUi).toHaveBeenCalledOnce());
    expect(hasText("待初始化")).toBe(true);
  });

  test("previews derived settings while editing the installation root", async () => {
    const api = mountLauncher({ getSnapshot: vi.fn(async () => loadedSnapshot) });

    (await findButton("偏好设置")).click();
    (await findButton("编辑设置")).click();
    const installInput = await vi.waitFor(() => getTextbox("安装目录"));
    installInput.value = "D:\\RayleaPortable";
    installInput.dispatchEvent(new Event("input"));

    await vi.waitFor(() => {
      expect(api.previewResolvedSettings).toHaveBeenCalledWith(expect.objectContaining({ installationRoot: "D:\\RayleaPortable" }));
      expect(getTextbox("服务端程序").value).toBe("D:\\RayleaPortable\\server\\raylea-server.exe");
      expect(getTextbox("配置文件").value).toBe("D:\\RayleaPortable\\config\\user.yaml");
    });
  });

  test("restores a pending close confirmation until cancellation succeeds", async () => {
    let rejectCloseConfirmation: (reason: Error) => void = () => undefined;
    const closeConfirmResponse = vi.fn()
      .mockImplementationOnce(() => new Promise<void>((_resolve, reject) => {
        rejectCloseConfirmation = reject;
      }))
      .mockResolvedValue(undefined);
    mountLauncher({
      getSnapshot: vi.fn(async () => loadedSnapshot),
      hasPendingCloseConfirm: vi.fn(async () => true),
      closeConfirmResponse,
    });

    getButton("取消", await findDialog()).click();
    await vi.waitFor(() => {
      expect(closeConfirmResponse).toHaveBeenCalledWith({ action: "cancel", setAsDefault: false });
      expect(queryDialog()).toBeNull();
    });

    rejectCloseConfirmation(new Error("desktop bridge unavailable"));

    getButton("取消", await findDialog()).click();
    await vi.waitFor(() => {
      expect(closeConfirmResponse).toHaveBeenCalledTimes(2);
      expect(queryDialog()).toBeNull();
    });
  });

  test("always resolves the external-service stop confirmation", async () => {
    let showExternalStopConfirm: (() => void) | undefined;
    const api = mountLauncher({
      getSnapshot: vi.fn(async () => loadedSnapshot),
      hasPendingExternalStopConfirm: vi.fn(async () => true),
      onShowExternalStopConfirm: vi.fn((listener: () => void) => {
        showExternalStopConfirm = listener;
        return () => undefined;
      }),
    });

    const pendingDialog = await findDialog("停止现有服务");
    pendingDialog.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    await vi.waitFor(() => {
      expect(api.externalStopConfirmResponse).toHaveBeenLastCalledWith(false);
      expect(queryDialog("停止现有服务")).toBeNull();
    });

    showExternalStopConfirm?.();
    getButton("打开管理界面", await findDialog("停止现有服务")).click();
    await vi.waitFor(() => expect(api.externalStopConfirmResponse).toHaveBeenLastCalledWith(true));
  });
});
