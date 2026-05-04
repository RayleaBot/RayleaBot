import { afterEach, describe, expect, test, vi } from "vitest";
import { createLauncherRuntimeContext } from "@main/services/launcher-runtime-context";
import { createLauncherSnapshotStore } from "@main/services/launcher-snapshot-store";
import { createLauncherDesktopActions } from "@main/services/launcher-desktop-actions";
import {
  FakeEndpointResolver,
  FakeExternalOpener,
  FakeProcessController,
  FakeSettingsStore,
} from "./launcher-test-doubles";

async function createDesktopActionsHarness() {
  const settingsStore = new FakeSettingsStore();
  const processController = new FakeProcessController();
  const externalOpener = new FakeExternalOpener();
  const runtimeContext = createLauncherRuntimeContext({
    settingsStore,
    endpointResolver: new FakeEndpointResolver(),
  });
  const snapshotStore = createLauncherSnapshotStore({
    processController,
  });
  const initialContext = await runtimeContext.initialize();
  snapshotStore.reset(initialContext);
  const desktopActions = createLauncherDesktopActions({
    runtimeContext,
    snapshotStore,
    externalOpener,
    processController,
  });

  return {
    desktopActions,
    externalOpener,
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
  test("openWebUi opens plain management urls without launcher tokens", async () => {
    const { desktopActions, externalOpener } = await createDesktopActionsHarness();

    await desktopActions.openWebUi("/tasks?task_id=task_fixture_0001");

    expect(externalOpener.openedUris.at(-1)).toContain("/tasks?task_id=task_fixture_0001");
    expect(externalOpener.openedUris.at(-1)).not.toContain("token=");

    await desktopActions.openWebUi();

    const latestUri = externalOpener.openedUris.at(-1) ?? "";
    expect(latestUri.endsWith("/")).toBe(true);
    expect(latestUri.includes("?token=")).toBe(false);
  });

  test("openWebUi can target the web dev server without launcher tokens", async () => {
    process.env.RAYLEA_WEB_UI_BASE_URL = "http://127.0.0.1:4173/";
    const { desktopActions, externalOpener } = await createDesktopActionsHarness();

    await desktopActions.openWebUi("/tasks?task_id=task_fixture_0001");

    expect(externalOpener.openedUris.at(-1)).toBe(
      "http://127.0.0.1:4173/tasks?task_id=task_fixture_0001",
    );
  });

  test("openWebUi falls back to the plain url and rejects absolute external targets", async () => {
    const { desktopActions, externalOpener } = await createDesktopActionsHarness();
    await desktopActions.openWebUi("/plugins/weather-pro");

    expect(externalOpener.openedUris.at(-1)).toBe("http://127.0.0.1:8080/plugins/weather-pro");
    await expect(desktopActions.openWebUi("https://evil.example/pwn")).rejects.toThrow(
      "启动器只允许打开管理界面的相对路径。",
    );
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
