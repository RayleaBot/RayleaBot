// @vitest-environment jsdom
import { mount } from "@vue/test-utils";
import { describe, expect, test } from "vitest";
import AppShell from "@renderer/AppShell.vue";
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
    const wrapper = mount(AppShell, {
      props: {
        snapshot,
        activeSection: "status",
      },
    });

    expect(wrapper.text()).toContain("RayleaBot 启动器");
    expect(wrapper.text()).toContain("状态");
    expect(wrapper.text()).toContain("环境检查");
    expect(wrapper.text()).toContain("日志与诊断");
    expect(wrapper.text()).toContain("设置");
    expect(wrapper.text()).toContain("服务尚未启动。");
    expect(wrapper.text()).toContain("首次启动时会自动生成用户配置。");
  });
});
