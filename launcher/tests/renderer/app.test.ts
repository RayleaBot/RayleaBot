// @vitest-environment jsdom
import { mount, flushPromises } from "@vue/test-utils";
import { afterEach, describe, expect, test, vi } from "vitest";
import App from "@renderer/App.vue";
import type { LauncherDesktopApi } from "@shared/desktop-api";
import type { LauncherSnapshot } from "@shared/launcher-models";

const blankSnapshot: LauncherSnapshot = {
  settings: {
    serverExecutablePath: "",
    configPath: "",
    workdir: "",
    closeBehavior: "ask_every_time",
  },
  endpoint: {
    host: "127.0.0.1",
    port: 8080,
    baseUrl: "http://127.0.0.1:8080/",
  },
  environmentChecks: [],
  recentStderr: [],
  processId: null,
  serviceState: "stopped",
  shutdownRequested: false,
  serviceDetail: "服务尚未启动。",
  lastError: "",
  releaseCheck: {
    status: "unavailable",
    currentVersion: "",
    latestVersion: "",
    summary: "版本信息不可用",
    detail: "",
    releasePageUrl: "",
    updateAvailable: false,
  },
};

const loadedSnapshot: LauncherSnapshot = {
  ...blankSnapshot,
  settings: {
    serverExecutablePath: "C:\\Users\\26789\\Desktop\\RayleaBot\\server\\raylea-server.exe",
    configPath: "C:\\Users\\26789\\Desktop\\RayleaBot\\config\\user.yaml",
    workdir: "C:\\Users\\26789\\Desktop\\RayleaBot",
    closeBehavior: "ask_every_time",
  },
  serviceDetail: "服务尚未启动。",
};

function installDesktopApi(api: LauncherDesktopApi) {
  Object.defineProperty(window, "rayleaLauncher", {
    configurable: true,
    value: api,
  });
}

afterEach(() => {
  Reflect.deleteProperty(window, "rayleaLauncher");
});

describe("App", () => {
  test("hydrates settings from getSnapshot after initialize resolves", async () => {
    let initialized = false;
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => (initialized ? loadedSnapshot : blankSnapshot)),
      initialize: vi.fn(async () => {
        initialized = true;
      }),
      refresh: vi.fn(async () => undefined),
      retry: vi.fn(async () => undefined),
      start: vi.fn(async () => undefined),
      stop: vi.fn(async () => undefined),
      openWebUi: vi.fn(async () => undefined),
      openReleasePage: vi.fn(async () => undefined),
      openLogsDirectory: vi.fn(async () => undefined),
      saveSettings: vi.fn(async () => undefined),
      chooseServerExecutable: vi.fn(async () => null),
      chooseConfigFile: vi.fn(async () => null),
      chooseWorkdir: vi.fn(async () => null),
      exitApplication: vi.fn(async () => undefined),
      onSnapshot: vi.fn(() => () => undefined),
    });

    const wrapper = mount(App);
    await flushPromises();

    expect(wrapper.text()).toContain("C:\\Users\\26789\\Desktop\\RayleaBot");
    wrapper.unmount();
  });

  test("keeps service actions disabled while initialization is still running", async () => {
    let resolveInitialize: (() => void) | null = null;
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => blankSnapshot),
      initialize: vi.fn(
        () =>
          new Promise<void>((resolve) => {
            resolveInitialize = resolve;
          }),
      ),
      refresh: vi.fn(async () => undefined),
      retry: vi.fn(async () => undefined),
      start: vi.fn(async () => undefined),
      stop: vi.fn(async () => undefined),
      openWebUi: vi.fn(async () => undefined),
      openReleasePage: vi.fn(async () => undefined),
      openLogsDirectory: vi.fn(async () => undefined),
      saveSettings: vi.fn(async () => undefined),
      chooseServerExecutable: vi.fn(async () => null),
      chooseConfigFile: vi.fn(async () => null),
      chooseWorkdir: vi.fn(async () => null),
      exitApplication: vi.fn(async () => undefined),
      onSnapshot: vi.fn(() => () => undefined),
    });

    const wrapper = mount(App);
    await flushPromises();

    expect(wrapper.find("button.action.primary").attributes("disabled")).toBeDefined();

    resolveInitialize?.();
    await flushPromises();
    wrapper.unmount();
  });
});
