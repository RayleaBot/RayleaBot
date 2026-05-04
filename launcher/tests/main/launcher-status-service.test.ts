import { describe, expect, test, vi } from "vitest";
import { deriveLauncherPresentation, resolveRecoverySummary } from "@shared/launcher-presentation";
import { createLauncherRuntimeContext } from "@main/services/launcher-runtime-context";
import { createLauncherSnapshotStore } from "@main/services/launcher-snapshot-store";
import { createLauncherStatusService } from "@main/services/launcher-status-service";
import {
  defaultOperationContext,
  FakeEndpointResolver,
  FakeManagementClient,
  FakeProcessController,
  FakeRecoverySummaryReader,
  FakeReleaseFeedClient,
  FakeSettingsStore,
  okInspection,
} from "./launcher-test-doubles";

async function createStatusHarness(options: {
  inspectEnvironment?: ReturnType<typeof vi.fn>;
  managementClient?: FakeManagementClient;
  processController?: FakeProcessController;
  recoverySummaryReader?: FakeRecoverySummaryReader;
  releaseFeedClient?: FakeReleaseFeedClient | { getSnapshot(): Promise<any> };
} = {}) {
  const settingsStore = new FakeSettingsStore();
  const managementClient = options.managementClient ?? new FakeManagementClient();
  const processController = options.processController ?? new FakeProcessController();
  const runtimeContext = createLauncherRuntimeContext({
    settingsStore,
    endpointResolver: new FakeEndpointResolver(),
  });
  const snapshotStore = createLauncherSnapshotStore({
    processController,
    releaseFeedClient: options.releaseFeedClient,
  });
  const initialContext = await runtimeContext.initialize();
  snapshotStore.reset(initialContext);
  const inspectEnvironment = options.inspectEnvironment ?? vi.fn(async () => okInspection());
  const statusService = createLauncherStatusService({
    runtimeContext,
    snapshotStore,
    inspectEnvironment,
    managementClient,
    processController,
    recoverySummaryReader: options.recoverySummaryReader,
  });

  return {
    inspectEnvironment,
    managementClient,
    processController,
    runtimeContext,
    snapshotStore,
    statusService,
  };
}

describe("launcher status service", () => {
  test("refresh reports blocking preflight checks and local recovery fallback", async () => {
    const recoverySummaryReader = new FakeRecoverySummaryReader();
    recoverySummaryReader.summary = {
      status: "blocked",
      phase: "pre_restore",
      operation: "rollback",
      created_at: "2026-04-02T08:00:00Z",
      updated_at: "2026-04-02T08:01:00Z",
    };
    const inspectEnvironment = vi.fn(async () =>
      okInspection({
        checks: [
          {
            scope: "preflight",
            code: "config.missing",
            title: "用户配置",
            severity: "error",
            summary: "配置基线不完整。",
            detail: "缺少 user.yaml。",
            remediation: "请先恢复配置。",
          },
        ],
        preflightChecks: [
          {
            scope: "preflight",
            code: "config.missing",
            title: "用户配置",
            severity: "error",
            summary: "配置基线不完整。",
            detail: "缺少 user.yaml。",
            remediation: "请先恢复配置。",
          },
        ],
        hasBlockingIssues: true,
      }),
    );
    const { snapshotStore, statusService } = await createStatusHarness({
      inspectEnvironment,
      recoverySummaryReader,
    });

    await statusService.refresh(false);

    expect(deriveLauncherPresentation(snapshotStore.snapshot).state).toBe("stopped");
    expect(deriveLauncherPresentation(snapshotStore.snapshot).detail).toContain("配置基线不完整");
    expect(resolveRecoverySummary(snapshotStore.snapshot)?.status).toBe("blocked");
  });

  test("refresh reports readiness retrieval failures without losing health state", async () => {
    const managementClient = new FakeManagementClient();
    managementClient.getReadiness = vi.fn(async () => {
      throw new Error("readyz warming up");
    });
    const { snapshotStore, statusService } = await createStatusHarness({ managementClient });

    await statusService.refresh(false);

    expect(snapshotStore.snapshot.server.health?.status).toBe("ok");
    expect(deriveLauncherPresentation(snapshotStore.snapshot).state).toBe("failed");
    expect(snapshotStore.snapshot.launcher.lastLocalError).toContain("readyz warming up");
  });

  test("refresh keeps ready and degraded states from /readyz", async () => {
    const managementClient = new FakeManagementClient();
    managementClient.readiness = { status: "ready", reason: "服务稳定。" };
    const readyHarness = await createStatusHarness({ managementClient });

    await readyHarness.statusService.refresh(false);

    expect(deriveLauncherPresentation(readyHarness.snapshotStore.snapshot).state).toBe("running");
    expect(readyHarness.managementClient.systemStatusCalls).toBe(1);

    const degradedClient = new FakeManagementClient();
    degradedClient.readiness = {
      status: "degraded",
      issues: [
        {
          code: "adapter.connection_pending",
          severity: "warning",
          summary: "OneBot 正在建立连接",
          remediation: "请稍后重试。",
        },
      ],
    };
    const degradedHarness = await createStatusHarness({ managementClient: degradedClient });

    await degradedHarness.statusService.refresh(false);

    expect(deriveLauncherPresentation(degradedHarness.snapshotStore.snapshot).state).toBe("degraded");
    expect(deriveLauncherPresentation(degradedHarness.snapshotStore.snapshot).detail).toBe("OneBot 正在建立连接");
  });

  test("refresh preserves setup_required and shutting_down semantics", async () => {
    const setupClient = new FakeManagementClient();
    setupClient.readiness = {
      status: "setup_required",
      reason: "管理员初始化尚未完成。",
    };
    const setupHarness = await createStatusHarness({ managementClient: setupClient });

    await setupHarness.statusService.refresh(false);

    expect(deriveLauncherPresentation(setupHarness.snapshotStore.snapshot).state).toBe("setup_required");

    const stoppingClient = new FakeManagementClient();
    stoppingClient.systemStatus = {
      status: "shutting_down",
      recovery_summary: null,
    };
    const stoppingHarness = await createStatusHarness({ managementClient: stoppingClient });

    await stoppingHarness.statusService.refresh(false);

    expect(deriveLauncherPresentation(stoppingHarness.snapshotStore.snapshot).state).toBe("stopping");
  });

  test("refresh falls back to local recovery summary when the service is unavailable", async () => {
    const managementClient = new FakeManagementClient();
    managementClient.health = false;
    const recoverySummaryReader = new FakeRecoverySummaryReader();
    recoverySummaryReader.summary = {
      status: "degraded",
      phase: "post_startup",
      operation: "upgrade",
      created_at: "2026-04-02T08:00:00Z",
      updated_at: "2026-04-02T08:01:00Z",
    };
    const { snapshotStore, statusService } = await createStatusHarness({
      managementClient,
      recoverySummaryReader,
    });

    await statusService.refresh(false);

    expect(resolveRecoverySummary(snapshotStore.snapshot)?.status).toBe("degraded");
    expect(snapshotStore.snapshot.server.readiness).toBeNull();
  });

  test("refresh does not block on slow release checks", async () => {
    let resolveRelease: ((value: Awaited<ReturnType<FakeReleaseFeedClient["getSnapshot"]>>) => void) | null = null;
    const slowReleaseClient = {
      getSnapshot: vi.fn(
        () =>
          new Promise<Awaited<ReturnType<FakeReleaseFeedClient["getSnapshot"]>>>((resolve) => {
            resolveRelease = resolve;
          }),
      ),
    };
    const { snapshotStore, statusService } = await createStatusHarness({
      releaseFeedClient: slowReleaseClient,
    });

    const result = await Promise.race([
      statusService.refresh(false).then(() => "resolved"),
      new Promise<string>((resolve) => setTimeout(() => resolve("timeout"), 25)),
    ]);

    expect(result).toBe("resolved");
    expect(snapshotStore.snapshot.launcher.releaseCheck.status).toBe("unavailable");

    resolveRelease?.({
      status: "up_to_date",
      currentVersion: "0.1.0",
      latestVersion: "0.1.0",
      summary: "当前版本 0.1.0 已是最新。",
      detail: "",
      releasePageUrl: "https://example.invalid/releases/v0.1.0",
      updateAvailable: false,
    });
  });
});
