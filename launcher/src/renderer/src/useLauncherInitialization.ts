import { useEffect, useState } from "react";

import { describeLauncherError, initialSnapshot } from "./AppState.shared";

export function useLauncherInitialization() {
  const [initializing, setInitializing] = useState(true);
  const [snapshot, setSnapshot] = useState(initialSnapshot);
  const [platformLabel, setPlatformLabel] = useState("");
  const [isMaximized, setIsMaximized] = useState(false);

  useEffect(() => {
    const unsub = window.rayleaLauncher.onSnapshot((next) => {
      setSnapshot(next);
    });
    window.rayleaLauncher
      .initialize()
      .then(async () => {
        const snap = await window.rayleaLauncher.getSnapshot();
        setSnapshot(snap);
      })
      .catch((error: unknown) => {
        setSnapshot((prev) => ({
          ...prev,
          launcher: {
            ...prev.launcher,
            lastLocalError: describeLauncherError(error, "启动器初始化失败。"),
            statusHint: "启动器初始化失败。",
          },
        }));
      })
      .finally(() => {
        setInitializing(false);
      });

    return unsub;
  }, []);

  useEffect(() => {
    window.rayleaLauncher.isMaximized().then(setIsMaximized);
    const unsub = window.rayleaLauncher.onMaximizedChange(setIsMaximized);
    return unsub;
  }, []);

  useEffect(() => {
    let cancelled = false;
    window.rayleaLauncher
      .getPlatform()
      .then((value) => {
        if (!cancelled) {
          setPlatformLabel(value);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setPlatformLabel("");
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  return {
    initializing,
    isMaximized,
    platformLabel,
    setSnapshot,
    snapshot,
  };
}
