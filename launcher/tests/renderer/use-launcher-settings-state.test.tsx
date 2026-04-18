// @vitest-environment jsdom
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, test, vi } from "vitest";
import type { LauncherDesktopApi } from "@shared/desktop-api";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { createLauncherSnapshot } from "../helpers/snapshot";

import { useLauncherSettingsState } from "@renderer/useLauncherSettingsState";

const snapshot: LauncherSnapshot = createLauncherSnapshot({
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

function installDesktopApi(api: Partial<LauncherDesktopApi>) {
  Object.defineProperty(window, "rayleaLauncher", {
    configurable: true,
    value: {
      previewResolvedSettings: vi.fn(async () => snapshot.launcher.resolvedSettings),
      ...api,
    },
  });
}

afterEach(() => {
  Reflect.deleteProperty(window, "rayleaLauncher");
});

describe("useLauncherSettingsState", () => {
  test("keeps a draft and refreshes preview settings while editing", async () => {
    installDesktopApi({
      previewResolvedSettings: vi.fn(async (settings) => ({
        installationRoot: settings.installationRoot,
        serverExecutablePath: `${settings.installationRoot}\\custom-server.exe`,
        configPath: `${settings.installationRoot}\\custom.yaml`,
        workdir: settings.installationRoot,
      })),
    });

    const { result } = renderHook(
      ({ editingSettings }) => useLauncherSettingsState(snapshot, editingSettings),
      { initialProps: { editingSettings: true } },
    );

    act(() => {
      result.current.setEditingDraft({
        ...snapshot.launcher.settings,
        installationRoot: "D:\\Portable",
      });
    });

    await waitFor(() => {
      expect(result.current.settingsDraft.installationRoot).toBe("D:\\Portable");
      expect(result.current.previewResolvedSettings.serverExecutablePath).toBe("D:\\Portable\\custom-server.exe");
    });
  });

  test("falls back to current resolved settings when preview fails", async () => {
    installDesktopApi({
      previewResolvedSettings: vi.fn(async () => {
        throw new Error("preview failed");
      }),
    });

    const { result } = renderHook(
      ({ editingSettings }) => useLauncherSettingsState(snapshot, editingSettings),
      { initialProps: { editingSettings: true } },
    );

    act(() => {
      result.current.setEditingDraft({
        ...snapshot.launcher.settings,
        installationRoot: "E:\\Broken",
      });
    });

    await waitFor(() => {
      expect(result.current.previewResolvedSettings).toEqual(snapshot.launcher.resolvedSettings);
    });
  });
});
