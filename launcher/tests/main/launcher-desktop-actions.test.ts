import { afterEach, describe, expect, test, vi } from "vitest";
import { createLauncherRuntimeContext } from "@main/services/launcher-runtime-context";
import { createLauncherSnapshotStore } from "@main/services/launcher-snapshot-store";
import { createLauncherDesktopActions } from "@main/services/launcher-desktop-actions";
import {
  FakeEndpointResolver,
  FakeExternalOpener,
  FakeManagementClient,
  FakeProcessController,
  FakeSettingsStore,
} from "./launcher-test-doubles";

async function createDesktopActionsHarness() {
  const settingsStore = new FakeSettingsStore();
  const managementClient = new FakeManagementClient();
  const processController = new FakeProcessController();
  const externalOpener = new FakeExternalOpener();
  const runtimeContext = createLauncherRuntimeContext({
    settingsStore,
    endpointResolver: new FakeEndpointResolver(),
    managementClient,
  });
  const snapshotStore = createLauncherSnapshotStore({
    processController,
  });
  const initialContext = await runtimeContext.initialize();
  snapshotStore.reset(initialContext);
  const desktopActions = createLauncherDesktopActions({
    runtimeContext,
    snapshotStore,
    managementClient,
    externalOpener,
    processController,
  });

  return {
    desktopActions,
    externalOpener,
    managementClient,
    processController,
    snapshotStore,
  };
}

const originalWebUiBaseUrl = process.env.RAYLEA_WEB_UI_BASE_URL;

afterEach(() => {
  if (originalWebUiBaseUrl === undefined) {
    delete process.env.RAYLEA_WEB_UI_BASE_URL;
  } else {
    process.env.RAYLEA_WEB_UI_BASE_URL = originalWebUiBaseUrl;
  }
  vi.restoreAllMocks();
});

describe("launcher desktop actions", () => {
  test("openWebUi adds launcher token only when setup is initialized", async () => {
    const { desktopActions, externalOpener, managementClient } = await createDesktopActionsHarness();

    await desktopActions.openWebUi("/tasks?task_id=task_fixture_0001");

    expect(externalOpener.openedUris.at(-1)).toContain("/tasks?task_id=task_fixture_0001");
    expect(externalOpener.openedUris.at(-1)).toContain("&token=");

    managementClient.setupInitialized = false;
    await desktopActions.openWebUi();

    const latestUri = externalOpener.openedUris.at(-1) ?? "";
    expect(latestUri.endsWith("/")).toBe(true);
    expect(latestUri.includes("?token=")).toBe(false);
  });

  test("openWebUi can target the web dev server while using the backend endpoint for tokens", async () => {
    process.env.RAYLEA_WEB_UI_BASE_URL = "http://127.0.0.1:4173/";
    const { desktopActions, externalOpener, managementClient } = await createDesktopActionsHarness();
    const issueLauncherToken = vi.spyOn(managementClient, "issueLauncherToken");

    await desktopActions.openWebUi("/tasks?task_id=task_fixture_0001");

    expect(issueLauncherToken).toHaveBeenCalledWith(expect.objectContaining({ baseUrl: "http://127.0.0.1:8080/" }));
    expect(externalOpener.openedUris.at(-1)).toBe(
      "http://127.0.0.1:4173/tasks?task_id=task_fixture_0001&token=launcher_fixture_token",
    );
  });

  test("openWebUi falls back to the plain url and rejects absolute external targets", async () => {
    const { desktopActions, externalOpener, managementClient } = await createDesktopActionsHarness();
    managementClient.getSetupInitialized = vi.fn(async () => {
      throw new Error("setup status unavailable");
    });

    await desktopActions.openWebUi("/plugins/weather-pro");

    expect(externalOpener.openedUris.at(-1)).toBe("http://127.0.0.1:8080/plugins/weather-pro");
    await expect(desktopActions.openWebUi("https://evil.example/pwn")).rejects.toThrow(
      "启动器只允许打开管理界面的相对路径。",
    );
  });

  test("recovery and bootstrap actions reuse in-progress tasks and open the task page", async () => {
    const { desktopActions, externalOpener, managementClient, snapshotStore } = await createDesktopActionsHarness();
    await snapshotStore.publish({
      ...snapshotStore.snapshot,
      server: {
        ...snapshotStore.snapshot.server,
        systemStatus: {
          status: "running",
          recovery_summary: {
            status: "degraded",
            phase: "post_startup",
            operation: "upgrade",
            created_at: "2026-04-04T08:00:00Z",
            updated_at: "2026-04-04T08:00:01Z",
          },
        },
      },
    });
    managementClient.inProgressTask = {
      task_id: "task_existing_0001",
      task_type: "runtime.bootstrap",
      status: "running",
      created_at: "2026-04-04T08:00:00Z",
      updated_at: "2026-04-04T08:00:01Z",
    } as any;

    const createRecoveryRecheck = vi.spyOn(managementClient, "createRecoveryRecheck");
    const createRuntimeBootstrap = vi.spyOn(managementClient, "createRuntimeBootstrap");

    await desktopActions.createRecoveryRecheck();
    await desktopActions.createRuntimeBootstrap(["chromium"]);

    expect(createRecoveryRecheck).not.toHaveBeenCalled();
    expect(createRuntimeBootstrap).not.toHaveBeenCalled();
    expect(externalOpener.openedUris.at(-2)).toContain("/tasks?task_id=task_existing_0001");
    expect(externalOpener.openedUris.at(-1)).toContain("/tasks?task_id=task_existing_0001");
  });

  test("openReleasePage and openLogsDirectory keep current behavior", async () => {
    const { desktopActions, externalOpener, processController, snapshotStore } = await createDesktopActionsHarness();

    await desktopActions.openReleasePage();

    expect(snapshotStore.snapshot.launcher.statusHint).toBe("当前运行没有可打开的发布页。");

    await snapshotStore.publish({
      ...snapshotStore.snapshot,
      launcher: {
        ...snapshotStore.snapshot.launcher,
        releaseCheck: {
          ...snapshotStore.snapshot.launcher.releaseCheck,
          releasePageUrl: "https://example.invalid/releases/v0.1.0",
        },
      },
    });

    await desktopActions.openReleasePage();
    await desktopActions.openLogsDirectory();

    expect(externalOpener.openedUris.at(-1)).toBe("https://example.invalid/releases/v0.1.0");
    expect(externalOpener.openedDirectories.at(-1)).toBe(processController.logDirectory);
  });
});
