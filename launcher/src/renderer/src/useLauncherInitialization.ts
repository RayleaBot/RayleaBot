import { useEffect, useState } from "react";

import { describeLauncherError, initialSnapshot } from "./AppState.shared";

export function useLauncherInitialization() {
  const [initializing, setInitializing] = useState(true);
  const [snapshot, setSnapshot] = useState(initialSnapshot);
  const [platformLabel, setPlatformLabel] = useState("");
  const [isMaximized, setIsMaximized] = useState(false);

  useEffect(() => {
    let active = true;
    let snapshotVersion = 0;
    const unsub = window.rayleaLauncher.onSnapshot((next) => {
      if (!active) return;
      snapshotVersion += 1;
      setSnapshot(next);
    });
    window.rayleaLauncher
      .initialize()
      .then(async () => {
        if (!active) return;
        const version = snapshotVersion;
        const snap = await window.rayleaLauncher.getSnapshot();
        if (active && version === snapshotVersion) setSnapshot(snap);
      })
      .catch((error: unknown) => {
        if (!active) return;
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
        if (active) setInitializing(false);
      });

    return () => {
      active = false;
      unsub();
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    let receivedChange = false;
    const unsub = window.rayleaLauncher.onMaximizedChange((value) => {
      if (cancelled) return;
      receivedChange = true;
      setIsMaximized(value);
    });
    window.rayleaLauncher
      .isMaximized()
      .then((value) => {
        if (!cancelled && !receivedChange) {
          setIsMaximized(value);
        }
      })
      .catch(() => {
        if (!cancelled && !receivedChange) {
          setIsMaximized(false);
        }
      });
    return () => {
      cancelled = true;
      unsub();
    };
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
