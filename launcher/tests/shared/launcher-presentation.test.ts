import { describe, expect, test } from "vitest";

import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import { createLauncherSnapshot } from "../helpers/snapshot";

describe("Launcher presentation", () => {
  test.each([
    [
      "a reachable service that fails readiness",
      createLauncherSnapshot({
        server: { health: { status: "ok" }, readiness: { status: "failed" } },
        launcher: { processLifecycle: "running", processOwnership: "launcher_managed" },
      }),
      "unhealthy",
    ],
    [
      "a reachable service whose readiness cannot be read",
      createLauncherSnapshot({ server: { health: { status: "ok" } }, launcher: { processOwnership: "external", lastLocalError: "timeout" } }),
      "unhealthy",
    ],
    [
      "a live process that fails its health check",
      createLauncherSnapshot({ launcher: { processLifecycle: "running", processOwnership: "launcher_managed", lastLocalError: "健康检查失败。" } }),
      "unhealthy",
    ],
    [
      "a start that left nothing running",
      createLauncherSnapshot({ launcher: { lastLocalError: "服务进程在通过健康检查前退出。" } }),
      "failed",
    ],
  ])("classifies %s", (_name, snapshot, state) => {
    expect(deriveLauncherPresentation(snapshot).state).toBe(state);
  });

  test("explains a running service the Launcher cannot control instead of reporting a fault", () => {
    const hint = "这个服务不是由启动器启动的，启动器无法停止它；需要停止时请在管理界面停止服务。";
    const external = createLauncherSnapshot({
      server: { health: { status: "ok" }, readiness: { status: "ready" } },
      launcher: { processOwnership: "external", statusHint: hint },
    });

    expect(deriveLauncherPresentation(external)).toMatchObject({ label: "运行中", detail: hint });
  });

  test("names a service that still needs its first administrator and keeps it usable", () => {
    const managed = createLauncherSnapshot({
      server: { health: { status: "ok" }, readiness: { status: "setup_required" } },
      launcher: { processLifecycle: "running", processOwnership: "launcher_managed" },
    });
    const external = createLauncherSnapshot({
      server: { health: { status: "ok" }, readiness: { status: "setup_required" } },
      launcher: { processOwnership: "external" },
    });

    expect(deriveLauncherPresentation(managed)).toMatchObject({ state: "setup_required", label: "待初始化", canOpenWebUi: true, canStopService: true });
    // Only the Launcher's own service can be opened with its setup token; another one needs its console address.
    expect(deriveLauncherPresentation(external).detail).not.toBe(deriveLauncherPresentation(managed).detail);
  });

  test("offers runtime preparation only for resources a usable service reports", () => {
    const issue = {
      code: "platform.resource_missing",
      severity: "warning" as const,
      summary: "图片渲染 Chromium 未准备。",
      runtime_resources: ["chromium" as const],
    };
    const degraded = createLauncherSnapshot({ server: { health: { status: "ok" }, readiness: { status: "degraded", issues: [issue, issue] } } });
    const setupRequired = createLauncherSnapshot({ server: { health: { status: "ok" }, readiness: { status: "setup_required", issues: [issue] } } });

    expect(deriveLauncherPresentation(degraded).preparableRuntimeResources).toEqual(["chromium"]);
    expect(deriveLauncherPresentation(setupRequired).preparableRuntimeResources).toEqual([]);
  });
});
