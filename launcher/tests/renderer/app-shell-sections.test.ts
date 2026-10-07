// @vitest-environment jsdom
import { mount } from "@vue/test-utils";
import { describe, expect, test } from "vitest";
import { nextTick } from "vue";

import AppShellAboutSection from "@renderer/AppShellAboutSection.vue";
import AppShellDiagnosticsSection from "@renderer/AppShellDiagnosticsSection.vue";
import AppShellEnvironmentSection from "@renderer/AppShellEnvironmentSection.vue";
import AppShellSettingsSection from "@renderer/AppShellSettingsSection.vue";
import AppShellStatusSection from "@renderer/AppShellStatusSection.vue";
import { isRuntimePreparationIssue } from "@renderer/AppShell.shared";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { createLauncherSnapshot } from "../helpers/snapshot";
import { elementsWithText, getButton, getTextbox, hasText, queryButton } from "../helpers/dom";

function mountStatusSection(snapshot: LauncherSnapshot) {
  return mount(AppShellStatusSection, {
    props: {
      snapshot,
      resolvedSettings: snapshot.launcher.resolvedSettings,
      busyAction: null,
      controlsDisabled: false,
    },
  });
}

function mountAboutSection(snapshot: LauncherSnapshot) {
  return mount(AppShellAboutSection, { props: { snapshot, controlsDisabled: false } });
}

function mountSettingsSection(snapshot: LauncherSnapshot) {
  return mount(AppShellSettingsSection, {
    props: {
      snapshot,
      settingsDraft: snapshot.launcher.settings,
      resolvedSettings: snapshot.launcher.resolvedSettings,
      editingSettings: false,
      closeBehavior: snapshot.launcher.settings.closeBehavior,
      closeBehaviorError: "",
      busyAction: null,
      controlsDisabled: false,
    },
  });
}

describe("runtime preparation issue classification", () => {
  test.each([
    ["deps.manifest_missing", true],
    ["chromium.not_ready", true],
    ["python.not_ready", false],
    ["nodejs.not_ready", false],
    ["npm.not_ready", false],
    ["config.user", false],
  ])("classifies %s", (code, expected) => {
    expect(isRuntimePreparationIssue(code)).toBe(expected);
  });
});

const configuredSnapshot = createLauncherSnapshot({
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
  },
});

describe("Launcher workspace presentation", () => {
  test.each([
    ["", "尚未检查"],
    ["初始化失败。", "检查结果不可用"],
  ])("does not report empty checks as healthy when the local error is %j", (lastLocalError, label) => {
    const wrapper = mount(AppShellEnvironmentSection, {
      props: { snapshot: createLauncherSnapshot({ launcher: { lastLocalError } }), platformLabel: "Windows x64" },
    });

    expect(wrapper.get("#environment-summary-title").text()).toBe(label);
    expect(hasText("可以启动", wrapper.element)).toBe(false);
    expect(hasText("当前未发现阻塞或警告项。", wrapper.element)).toBe(false);
  });

  test("shows issues immediately while keeping healthy environment checks collapsed", async () => {
    const snapshot = createLauncherSnapshot({
      launcher: {
        preflightChecks: [
          {
            scope: "preflight",
            code: "config.user",
            title: "用户配置",
            severity: "error",
            summary: "配置文件不可读。",
            detail: "无法读取当前用户配置。",
            remediation: "重新选择有效的配置文件。",
          },
          {
            scope: "preflight",
            code: "server.executable",
            title: "服务端程序",
            severity: "ok",
            summary: "已找到可执行文件。",
            detail: "服务端程序可用。",
            remediation: "",
          },
        ],
      },
    });

    const wrapper = mount(AppShellEnvironmentSection, { props: { snapshot, platformLabel: "Windows x64" } });

    expect(hasText("配置文件不可读。", wrapper.element)).toBe(true);
    expect(hasText("重新选择有效的配置文件。", wrapper.element)).toBe(true);
    const disclosure = wrapper.get("details");
    expect(disclosure.attributes("open")).toBeUndefined();
    expect(hasText("服务端程序", disclosure.element)).toBe(true);

    await disclosure.get("summary").trigger("click");
    expect(disclosure.attributes("open")).toBeDefined();
  });

  test("explains a failed start with its hint and cause rather than a generic local error", () => {
    const wrapper = mountStatusSection(createLauncherSnapshot({
      launcher: {
        statusHint: "服务进程在启动阶段提前退出。",
        lastLocalError: "listen tcp 127.0.0.1:8080: bind: address already in use",
      },
    }));

    expect(wrapper.get("#service-control-title").text()).toContain("启动失败");
    expect(wrapper.text()).toContain("服务进程在启动阶段提前退出。");
    expect(hasText("失败原因", wrapper.element)).toBe(true);
    expect(hasText("listen tcp 127.0.0.1:8080: bind: address already in use", wrapper.element)).toBe(true);
    expect(hasText("当前限制", wrapper.element)).toBe(false);
  });

  test("sends runtime preparation to the management UI for the resources the service reports", async () => {
    const wrapper = mountStatusSection(createLauncherSnapshot({
      server: {
        health: { status: "ok" },
        readiness: {
          status: "degraded",
          reason: "图片渲染 Chromium 未准备。",
          reason_codes: ["platform.resource_missing"],
          checks: { render: "resource_missing" },
          issues: [{
            code: "platform.resource_missing",
            severity: "warning",
            summary: "图片渲染 Chromium 未准备。",
            remediation: "启动运行环境任务准备图片渲染 Chromium。",
            runtime_resources: ["chromium"],
          }],
        },
      },
      launcher: {
        processLifecycle: "running",
        processOwnership: "launcher_managed",
        preflightChecks: [{
          scope: "preflight",
          code: "chromium.not_ready",
          title: "图片渲染 Chromium",
          severity: "ok",
          summary: "尚未准备，启动服务时自动准备。",
          detail: "",
          remediation: "",
        }],
      },
    }));

    const rail = wrapper.get("aside[aria-label='需要关注的项目']");
    // A file the service prepares on start is not an environment problem.
    expect(hasText("环境问题", rail.element)).toBe(false);
    expect(hasText("图片渲染 Chromium", rail.element)).toBe(true);
    expect(wrapper.text()).not.toContain("platform.resource_missing");
    getButton("在管理界面准备", rail.element).click();
    await nextTick();
    expect(wrapper.emitted("openWeb")).toHaveLength(1);
  });

  test("separates settings reading mode from its editable controls", async () => {
    const wrapper = mountSettingsSection(configuredSnapshot);

    expect(wrapper.find("input").exists()).toBe(false);

    await wrapper.setProps({ editingSettings: true });

    expect(getTextbox("安装目录", wrapper.element).value).toBe("C:\\RayleaBot");
  });

  test("keeps technical diagnostics collapsed and promotes real stderr", async () => {
    const wrapper = mount(AppShellDiagnosticsSection, {
      props: { snapshot: configuredSnapshot, diagnosticsSummary: "状态摘要：未启动" },
    });

    expect(wrapper.get("details").attributes("open")).toBeUndefined();
    expect(hasText("当前没有新的异常输出", wrapper.element)).toBe(true);

    await wrapper.setProps({
      snapshot: createLauncherSnapshot({ launcher: { recentStderr: ["listen failed"] } }),
      diagnosticsSummary: "状态摘要：启动失败",
    });

    expect(wrapper.get("pre.diagnostics-log__surface").text()).toBe("listen failed");
    expect(wrapper.get("#diagnostics-log-title").text()).toBe("异常输出");
  });

  test("does not report a clean log for a service whose output the Launcher never captured", () => {
    const snapshot = createLauncherSnapshot({
      server: { health: { status: "ok" }, readiness: { status: "ready" } },
      launcher: { processOwnership: "external" },
    });
    const wrapper = mount(AppShellDiagnosticsSection, { props: { snapshot, diagnosticsSummary: "" } });

    expect(elementsWithText("未捕获服务输出", wrapper.element).some((element) => element.tagName === "DD")).toBe(true);
    expect(hasText("启动器未捕获该服务的输出，请在管理界面的实时日志查看。", wrapper.element)).toBe(true);
    expect(wrapper.text()).not.toMatch(/未发现异常输出|当前没有新的异常输出/);
  });

  // The HarmonyOS Sans agreement requires a notice in the software that the fonts are used.
  test("states the bundled HarmonyOS Sans interface font", () => {
    const wrapper = mountAboutSection(configuredSnapshot);

    expect(hasText("界面字体", wrapper.element)).toBe(true);
    expect(hasText("HarmonyOS Sans SC", wrapper.element)).toBe(true);
  });

  test("explains unavailable updates without rendering a broken action", () => {
    const wrapper = mountAboutSection(configuredSnapshot);

    expect(hasText("当前构建不提供更新检查", wrapper.element)).toBe(true);
    expect(queryButton("检查更新", wrapper.element)).toBeNull();
    expect(getButton("GitHub", wrapper.element).disabled).toBe(false);
  });

  test("shows the version as loading while the first update check runs", () => {
    const wrapper = mountAboutSection(createLauncherSnapshot({ launcher: { releaseCheck: { status: "checking", currentVersion: "" } } }));

    expect(hasText("读取中", wrapper.element)).toBe(true);
    expect(hasText("开发", wrapper.element)).toBe(false);
  });

  test("offers one-click update and the release page when a newer version exists", async () => {
    const wrapper = mountAboutSection(createLauncherSnapshot({
      launcher: {
        releaseCheck: {
          status: "update_available",
          currentVersion: "0.3.0",
          latestVersion: "0.4.0",
          releasePageUrl: "https://example.invalid/releases/v0.4.0",
          updateAvailable: true,
          canCheck: true,
        },
      },
    }));

    getButton("立即更新", wrapper.element).click();
    getButton("发布页", wrapper.element).click();
    await nextTick();
    expect(wrapper.emitted("applyUpdate")).toHaveLength(1);
    expect(wrapper.emitted("openReleasePage")).toHaveLength(1);
  });

  test("keeps the update action disabled while an update runs", () => {
    const wrapper = mountAboutSection(createLauncherSnapshot({
      launcher: {
        releaseCheck: {
          status: "updating",
          currentVersion: "0.3.0",
          latestVersion: "0.4.0",
          summary: "正在下载更新。",
          updateAvailable: true,
          canCheck: false,
        },
      },
    }));

    expect(getButton("正在更新", wrapper.element).disabled).toBe(true);
    expect(hasText("正在下载更新。", wrapper.element)).toBe(true);
    expect(queryButton("立即更新", wrapper.element)).toBeNull();
  });

  test("offers the release page for platform-guided builds", async () => {
    const wrapper = mountAboutSection(createLauncherSnapshot({
      launcher: {
        releaseCheck: {
          status: "disabled",
          currentVersion: "0.3.0",
          releasePageUrl: "https://example.invalid/releases/latest",
          updateAvailable: false,
          canCheck: false,
        },
      },
    }));

    expect(hasText("0.3.0", wrapper.element)).toBe(true);
    getButton("打开发布页", wrapper.element).click();
    await nextTick();
    expect(wrapper.emitted("openReleasePage")).toHaveLength(1);
  });

  test("shows the specific update error code and complete reason", () => {
    const wrapper = mountAboutSection(createLauncherSnapshot({
      launcher: {
        releaseCheck: {
          status: "failed",
          currentVersion: "0.3.0",
          summary: "检查更新失败",
          detail: "无法获取发布信息，请稍后重试。",
          errorCode: "launcher.update_check_failed",
          canCheck: true,
        },
      },
    }));

    // The version line names the state; only the error bar carries the failure text.
    expect(hasText("检查失败", wrapper.element)).toBe(true);
    expect(hasText("检查更新失败", wrapper.element)).toBe(true);
    expect(hasText("launcher.update_check_failed", wrapper.element)).toBe(true);
    expect(hasText("无法获取发布信息，请稍后重试。", wrapper.element)).toBe(true);
  });
});
