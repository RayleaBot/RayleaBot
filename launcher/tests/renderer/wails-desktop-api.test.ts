import { describe, expect, test } from "vitest";

import type * as desktopModels from "../../src/renderer/bindings/github.com/RayleaBot/RayleaBot/launcher/internal/desktop/models";
import { normalizeWailsSnapshot } from "../../src/renderer/src/wailsDesktopApi";
import { createLauncherSnapshot } from "../helpers/snapshot";

describe("Wails desktop snapshot bridge", () => {
  test("normalizes nullable Go slices before renderer state consumes them", () => {
    const snapshot = createLauncherSnapshot() as unknown as desktopModels.LauncherSnapshot;
    snapshot.launcher.environmentChecks = null;
    snapshot.launcher.preflightChecks = null;
    snapshot.launcher.advisoryChecks = null;
    snapshot.launcher.recentStderr = null;
    snapshot.launcher.runtimePrepare = {
      active: true,
      currentKind: "chromium",
      summary: "准备运行环境",
      resources: null,
    };

    const normalized = normalizeWailsSnapshot(snapshot);

    expect(normalized.launcher.environmentChecks).toEqual([]);
    expect(normalized.launcher.preflightChecks).toEqual([]);
    expect(normalized.launcher.advisoryChecks).toEqual([]);
    expect(normalized.launcher.recentStderr).toEqual([]);
    expect(normalized.launcher.runtimePrepare?.resources).toEqual([]);
  });

  test("accepts the updating release status", () => {
    const snapshot = createLauncherSnapshot() as unknown as desktopModels.LauncherSnapshot;
    snapshot.launcher.releaseCheck.status = "updating" as desktopModels.ReleaseCheckStatus;

    expect(normalizeWailsSnapshot(snapshot).launcher.releaseCheck.status).toBe("updating");
  });

  test("rejects enum drift instead of silently passing an incompatible snapshot", () => {
    const snapshot = createLauncherSnapshot() as unknown as desktopModels.LauncherSnapshot;
    snapshot.launcher.processLifecycle = "booting" as desktopModels.LauncherProcessLifecycle;

    expect(() => normalizeWailsSnapshot(snapshot)).toThrow("Invalid Wails payload field: launcher.processLifecycle");
  });
});
