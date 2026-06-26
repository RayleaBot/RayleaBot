import { describe, expect, test, vi } from "vitest";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import { createLauncherRuntimeContext } from "@main/services/launcher-runtime-context";
import { createLauncherSnapshotStore } from "@main/services/launcher-snapshot-store";
import { createLauncherStatusService } from "@main/services/launcher-status-service";
import { createLauncherLifecycleService } from "@main/services/launcher-lifecycle-service";
import {
  FakeEndpointResolver,
  FakeExternalOpener,
  FakeManagementClient,
  FakeProcessController,
  FakeResetAdminRunner,
  FakeSettingsStore,
  okInspection,
} from "./launcher-test-doubles";

async function createLifecycleHarness(options: {
  inspectEnvironment?: ReturnType<typeof vi.fn>;
  endpointResolver?: FakeEndpointResolver;
  managementClient?: FakeManagementClient;
  processController?: FakeProcessController;
  externalOpener?: FakeExternalOpener;
  isEndpointListening?: ReturnType<typeof vi.fn>;
  tryStopEndpointProcess?: ReturnType<typeof vi.fn>;
  confirmExternalServiceStop?: ReturnType<typeof vi.fn>;
  resetAdminRunner?: FakeResetAdminRunner;
} = {}) {
  const settingsStore = new FakeSettingsStore();
  const managementClient = options.managementClient ?? new FakeManagementClient();
  const processController = options.processController ?? new FakeProcessController();
  const runtimeContext = createLauncherRuntimeContext({
    settingsStore,
    endpointResolver: options.endpointResolver ?? new FakeEndpointResolver(),
  });
  const snapshotStore = createLauncherSnapshotStore({
    processController,
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
  });
  const externalOpener = options.externalOpener ?? new FakeExternalOpener();
  const lifecycleService = createLauncherLifecycleService({
    runtimeContext,
    snapshotStore,
    statusService,
    inspectEnvironment,
    managementClient,
    processController,
    isEndpointListening: options.isEndpointListening ?? vi.fn(async () => false),
    tryStopEndpointProcess: options.tryStopEndpointProcess ?? vi.fn(async () => false),
    externalOpener,
    confirmExternalServiceStop: options.confirmExternalServiceStop,
    resetAdminRunner: options.resetAdminRunner,
    options: {
      pollIntervalMs: 1,
      startupTimeoutMs: 60,
      startupReadinessGraceMs: 30,
      shutdownTimeoutMs: 1,
      resetAdminTimeoutMs: 60,
    },
  });

  return {
    externalOpener,
    inspectEnvironment,
    lifecycleService,
    managementClient,
    processController,
    runtimeContext,
    snapshotStore,
    statusService,
  };
}

describe("launcher lifecycle service", () => {
  test("start does not launch another process when the endpoint is already healthy", async () => {
    const { lifecycleService, processController, snapshotStore } = await createLifecycleHarness();

    await lifecycleService.start();

    expect(processController.startCalls).toBe(0);
    expect(deriveLauncherPresentation(snapshotStore.snapshot).state).toBe("running");
    expect(snapshotStore.snapshot.launcher.processOwnership).toBe("external");
  });

  test("start waits for readiness and ignores transient failed snapshots", async () => {
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    let healthChecks = 0;
    let readinessChecks = 0;

    managementClient.health = false;
    managementClient.isHealthy = vi.fn(async () => {
      healthChecks += 1;
      return healthChecks >= 3;
    });
    managementClient.getReadiness = vi.fn(async () => {
      readinessChecks += 1;
      if (readinessChecks === 1) {
        return {
          status: "failed",
          reason: "服务仍在完成启动。",
        };
      }
      return {
        status: "ready",
      };
    });

    const { lifecycleService, snapshotStore } = await createLifecycleHarness({
      managementClient,
      processController,
    });

    await lifecycleService.start();

    expect(processController.startCalls).toBe(1);
    expect(readinessChecks).toBeGreaterThanOrEqual(2);
    expect(deriveLauncherPresentation(snapshotStore.snapshot).state).toBe("running");
  });

  test("start reports port occupation when the child exits and another process is listening", async () => {
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    processController.recentStderr = ["listen on 127.0.0.1:8080: bind: address already in use"];
    managementClient.health = false;
    managementClient.isHealthy = vi.fn(async () => {
      processController.isRunning = false;
      return false;
    });
    const isEndpointListening = vi.fn()
      .mockResolvedValueOnce(false)
      .mockResolvedValue(true);
    const { lifecycleService, snapshotStore } = await createLifecycleHarness({
      managementClient,
      processController,
      isEndpointListening,
    });

    await lifecycleService.start();

    expect(deriveLauncherPresentation(snapshotStore.snapshot).state).toBe("failed");
    expect(deriveLauncherPresentation(snapshotStore.snapshot).detail).toContain("目标端口已被现有进程占用");
  });

  test("stop keeps an external service running when confirmation is declined", async () => {
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const tryStopEndpointProcess = vi.fn(async () => false);
    managementClient.shutdownFromLauncher = vi.fn(async () => undefined);
    const { lifecycleService, snapshotStore } = await createLifecycleHarness({
      managementClient,
      processController,
      tryStopEndpointProcess,
      confirmExternalServiceStop: vi.fn(async () => false),
    });

    await lifecycleService.stop();

    expect(managementClient.shutdownFromLauncher).not.toHaveBeenCalled();
    expect(processController.forceKillCalls).toBe(0);
    expect(tryStopEndpointProcess).not.toHaveBeenCalled();
    expect(deriveLauncherPresentation(snapshotStore.snapshot).state).toBe("running");
    expect(snapshotStore.snapshot.launcher.processOwnership).toBe("external");
  });

  test("stop surfaces external launcher shutdown failures without force killing the foreign process", async () => {
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const tryStopEndpointProcess = vi.fn(async () => false);
    managementClient.shutdownFromLauncher = vi.fn(async () => {
      throw new Error("launcher shutdown failed");
    });
    const { lifecycleService, snapshotStore } = await createLifecycleHarness({
      managementClient,
      processController,
      tryStopEndpointProcess,
      confirmExternalServiceStop: vi.fn(async () => true),
    });

    await lifecycleService.stop();

    expect(processController.forceKillCalls).toBe(0);
    expect(tryStopEndpointProcess).not.toHaveBeenCalled();
    expect(deriveLauncherPresentation(snapshotStore.snapshot).state).toBe("running");
    expect(snapshotStore.snapshot.launcher.lastLocalError).toContain("launcher shutdown failed");
  });

  test("stop does not call launcher shutdown for remote external services", async () => {
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const tryStopEndpointProcess = vi.fn(async () => false);
    const endpointResolver = new FakeEndpointResolver();
    endpointResolver.endpoint = {
      host: "192.0.2.10",
      port: 8080,
      baseUrl: "http://192.0.2.10:8080/",
    };
    managementClient.shutdownFromLauncher = vi.fn(async () => undefined);
    const { lifecycleService, snapshotStore } = await createLifecycleHarness({
      endpointResolver,
      managementClient,
      processController,
      tryStopEndpointProcess,
      confirmExternalServiceStop: vi.fn(async () => true),
    });

    await lifecycleService.stop();

    expect(managementClient.shutdownFromLauncher).not.toHaveBeenCalled();
    expect(processController.forceKillCalls).toBe(0);
    expect(tryStopEndpointProcess).not.toHaveBeenCalled();
    expect(snapshotStore.snapshot.launcher.lastLocalError).toContain("远程服务只能通过管理界面操作");
  });

  test("stop falls back to force kill when launcher-managed shutdown fails", async () => {
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    processController.isRunning = true;
    processController.forceKill = vi.fn(async () => {
      processController.forceKillCalls += 1;
      processController.isRunning = false;
      managementClient.health = false;
    });
    managementClient.shutdownFromLauncher = vi.fn(async () => {
      throw new Error("launcher shutdown failed");
    });
    const { lifecycleService, snapshotStore } = await createLifecycleHarness({
      managementClient,
      processController,
    });

    await lifecycleService.stop();

    expect(processController.forceKillCalls).toBe(1);
    expect(deriveLauncherPresentation(snapshotStore.snapshot).state).toBe("stopped");
  });

  test("resetAdmin waits for setup_required before opening the initialization entry", async () => {
    const managementClient = new FakeManagementClient();
    const resetAdminRunner = new FakeResetAdminRunner();
    let healthChecks = 0;

    managementClient.health = false;
    managementClient.isHealthy = vi.fn(async () => {
      healthChecks += 1;
      return healthChecks >= 3;
    });
    managementClient.getReadiness = vi.fn(async () => ({
      status: "setup_required",
      reason: "管理员初始化尚未完成。",
    }));

    const { externalOpener, lifecycleService, processController, snapshotStore } = await createLifecycleHarness({
      managementClient,
      resetAdminRunner,
    });

    await lifecycleService.resetAdmin();

    expect(resetAdminRunner.calls).toBe(1);
    expect(processController.startCalls).toBe(1);
    expect(deriveLauncherPresentation(snapshotStore.snapshot).state).toBe("setup_required");
    expect(externalOpener.openedUris.at(-1)).toBe("http://127.0.0.1:8080/");
  });

  test("resetAdmin surfaces restart failures with contextual errors", async () => {
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const resetAdminRunner = new FakeResetAdminRunner();
    managementClient.health = false;
    processController.start = vi.fn(async () => {
      throw new Error("spawn ENOENT");
    });

    const { lifecycleService, snapshotStore } = await createLifecycleHarness({
      managementClient,
      processController,
      resetAdminRunner,
    });

    await lifecycleService.resetAdmin();

    expect(resetAdminRunner.calls).toBe(1);
    expect(deriveLauncherPresentation(snapshotStore.snapshot).state).toBe("failed");
    expect(snapshotStore.snapshot.launcher.lastLocalError).toContain("spawn ENOENT");
    expect(deriveLauncherPresentation(snapshotStore.snapshot).detail).toContain("管理员凭据已重置");
  });
});
