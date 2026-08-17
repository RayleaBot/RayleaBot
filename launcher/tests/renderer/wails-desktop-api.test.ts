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

  test("rejects enum drift instead of silently passing an incompatible snapshot", () => {
    const snapshot = createLauncherSnapshot() as unknown as desktopModels.LauncherSnapshot;
    snapshot.launcher.processLifecycle = "booting";

    expect(() => normalizeWailsSnapshot(snapshot)).toThrow("Invalid Wails payload field: launcher.processLifecycle");
  });

  test("normalizes server contract payloads after runtime validation", () => {
    const snapshot = createLauncherSnapshot() as unknown as desktopModels.LauncherSnapshot;
    snapshot.server.health = { status: "ok" };
    snapshot.server.readiness = {
      status: "degraded",
      reason_codes: ["runtime.not_ready"],
      issues: [{ code: "runtime.not_ready", severity: "warning", summary: "Runtime is not ready" }],
    };
    snapshot.server.systemStatus = {
      status: "running",
      active_plugins: 2,
      health: { status: "ready" },
    };
    snapshot.launcher.localRecoverySummary = {
      status: "compatible",
      phase: "post_startup",
      operation: "upgrade",
      created_at: "2026-08-18T00:00:00Z",
      updated_at: "2026-08-18T00:00:01Z",
    };

    const normalized = normalizeWailsSnapshot(snapshot);

    expect(normalized.server.health).toEqual({ status: "ok" });
    expect(normalized.server.readiness?.issues?.[0]?.code).toBe("runtime.not_ready");
    expect(normalized.server.systemStatus?.active_plugins).toBe(2);
    expect(normalized.launcher.localRecoverySummary?.operation).toBe("upgrade");
  });

  test.each([
    ["health", (snapshot: desktopModels.LauncherSnapshot) => { snapshot.server.health = { status: "ready" }; }, "server.health.status"],
    ["readiness", (snapshot: desktopModels.LauncherSnapshot) => { snapshot.server.readiness = { status: "unknown" }; }, "server.readiness.status"],
    ["system status", (snapshot: desktopModels.LauncherSnapshot) => { snapshot.server.systemStatus = { status: "running", active_plugins: -1 }; }, "server.systemStatus.active_plugins"],
    ["recovery summary", (snapshot: desktopModels.LauncherSnapshot) => { snapshot.launcher.localRecoverySummary = { status: "compatible" }; }, "launcher.localRecoverySummary.phase"],
  ])("rejects invalid %s payloads", (_name, mutate, field) => {
    const snapshot = createLauncherSnapshot() as unknown as desktopModels.LauncherSnapshot;
    mutate(snapshot);

    expect(() => normalizeWailsSnapshot(snapshot)).toThrow(`Invalid server payload field: ${field}`);
  });
});
