import { describe, expect, test } from "vitest";
import {
  parseLauncherSettingsInput,
  parseRuntimeBootstrapResources,
  sanitizeLauncherWebTargetPath,
} from "@shared/launcher-validation";

describe("launcher validation", () => {
  test("accepts renderer settings payloads with string overrides", () => {
    expect(
      parseLauncherSettingsInput({
        installationRoot: "C:\\RayleaBot",
        closeBehavior: "hide_to_tray",
        advancedOverrides: {
          configPath: "C:\\RayleaBot\\config\\user.yaml",
          serverExecutablePath: "C:\\RayleaBot\\server\\raylea-server.exe",
        },
      }),
    ).toEqual({
      installationRoot: "C:\\RayleaBot",
      closeBehavior: "hide_to_tray",
      advancedOverrides: {
        configPath: "C:\\RayleaBot\\config\\user.yaml",
        serverExecutablePath: "C:\\RayleaBot\\server\\raylea-server.exe",
      },
    });
  });

  test("rejects malformed renderer settings payloads", () => {
    expect(() =>
      parseLauncherSettingsInput({
        installationRoot: "C:\\RayleaBot",
        closeBehavior: "hide_to_tray",
        advancedOverrides: {
          workdir: 42,
        },
      }),
    ).toThrow("启动器设置格式无效。");
  });

  test("rejects runtime bootstrap payloads with non-string resource ids", () => {
    expect(() => parseRuntimeBootstrapResources(["chromium", 42])).toThrow("运行环境资源列表格式无效。");
  });

  test("rejects external and scheme-based launcher web targets", () => {
    expect(() => sanitizeLauncherWebTargetPath("https://evil.example/pwn")).toThrow(
      "启动器只允许打开管理界面的相对路径。",
    );
    expect(() => sanitizeLauncherWebTargetPath("javascript:alert(1)")).toThrow(
      "启动器只允许打开管理界面的相对路径。",
    );
  });
});
