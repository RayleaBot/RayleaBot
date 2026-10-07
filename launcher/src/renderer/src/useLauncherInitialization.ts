import { onScopeDispose, ref, shallowRef } from "vue";
import type { LauncherSnapshot } from "@shared/launcher-models";

import { describeLauncherError, initialSnapshot } from "./AppState.shared";

/** Replaces part of the Launcher's local state, such as an error the renderer reports itself. */
export function patchLauncherState(snapshot: LauncherSnapshot, patch: Partial<LauncherSnapshot["launcher"]>): LauncherSnapshot {
  return { ...snapshot, launcher: { ...snapshot.launcher, ...patch } };
}

export function useLauncherInitialization() {
  const initializing = ref(true);
  const snapshot = shallowRef<LauncherSnapshot>(initialSnapshot);
  const platformLabel = ref("");
  const isMaximized = ref(false);
  let active = true;

  // Snapshots pushed while the first read is in flight are newer than its result.
  let snapshotVersion = 0;
  const unsubscribeSnapshot = window.rayleaLauncher.onSnapshot((next) => {
    if (!active) return;
    snapshotVersion += 1;
    snapshot.value = next;
  });
  window.rayleaLauncher
    .initialize()
    .then(async () => {
      if (!active) return;
      const version = snapshotVersion;
      const next = await window.rayleaLauncher.getSnapshot();
      if (active && version === snapshotVersion) snapshot.value = next;
    })
    .catch((error: unknown) => {
      if (!active) return;
      snapshot.value = patchLauncherState(snapshot.value, {
        lastLocalError: describeLauncherError(error, "启动器初始化失败。"),
        statusHint: "启动器初始化失败。",
      });
    })
    .finally(() => {
      if (active) initializing.value = false;
    });

  let receivedMaximizedChange = false;
  const unsubscribeMaximized = window.rayleaLauncher.onMaximizedChange((value) => {
    if (!active) return;
    receivedMaximizedChange = true;
    isMaximized.value = value;
  });
  window.rayleaLauncher
    .isMaximized()
    .then((value) => {
      if (active && !receivedMaximizedChange) {
        isMaximized.value = value;
      }
    })
    .catch(() => {
      if (active && !receivedMaximizedChange) {
        isMaximized.value = false;
      }
    });

  window.rayleaLauncher
    .getPlatform()
    .then((value) => {
      if (active) {
        platformLabel.value = value;
      }
    })
    .catch(() => {
      if (active) {
        platformLabel.value = "";
      }
    });

  onScopeDispose(() => {
    active = false;
    unsubscribeSnapshot();
    unsubscribeMaximized();
  });

  return {
    initializing,
    isMaximized,
    platformLabel,
    snapshot,
  };
}
