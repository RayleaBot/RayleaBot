import { sanitizeLauncherWebTargetPath } from "../../shared/launcher-validation";
import type {
  ExternalOpener,
  LauncherDesktopActions,
  LauncherRuntimeContext,
  LauncherSnapshotStore,
  ServerProcessController,
} from "./launcher-coordinator.types";

interface LauncherDesktopActionsDependencies {
  runtimeContext: LauncherRuntimeContext;
  snapshotStore: LauncherSnapshotStore;
  externalOpener: ExternalOpener;
  processController: ServerProcessController;
}

function resolveWebUiBaseUrl(fallbackBaseUrl: string) {
  const candidate = process.env.RAYLEA_WEB_UI_BASE_URL?.trim();
  if (!candidate) {
    return fallbackBaseUrl;
  }

  try {
    const url = new URL(candidate);
    if (url.protocol !== "http:" && url.protocol !== "https:") {
      return fallbackBaseUrl;
    }
    return url.toString();
  } catch {
    return fallbackBaseUrl;
  }
}

export function createLauncherDesktopActions(deps: LauncherDesktopActionsDependencies): LauncherDesktopActions {
  async function openWebUi(targetPath = "") {
    const context = await deps.runtimeContext.createOperationContext();
    const normalizedTarget = sanitizeLauncherWebTargetPath(targetPath);
    const webUiBaseUrl = resolveWebUiBaseUrl(context.endpoint.baseUrl);
    const url = normalizedTarget ? new URL(normalizedTarget, webUiBaseUrl) : new URL(webUiBaseUrl);

    await deps.externalOpener.openUri(url.toString());
  }

  return {
    openWebUi,
    async openReleasePage() {
      if (!deps.snapshotStore.snapshot.launcher.releaseCheck.releasePageUrl) {
        await deps.snapshotStore.publish({
          ...deps.snapshotStore.snapshot,
          launcher: {
            ...deps.snapshotStore.snapshot.launcher,
            statusHint: "当前运行没有可打开的发布页。",
            lastLocalError: "",
          },
        });
        return;
      }
      await deps.externalOpener.openUri(deps.snapshotStore.snapshot.launcher.releaseCheck.releasePageUrl);
    },
    async openLogsDirectory() {
      await deps.externalOpener.openDirectory(deps.processController.logDirectory);
    },
  };
}
