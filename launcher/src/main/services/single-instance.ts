type AppSecondInstanceListener = () => void;

interface SingleInstanceAppLike {
  on(event: "second-instance", listener: AppSecondInstanceListener): unknown;
  quit(): void;
  requestSingleInstanceLock(): boolean;
}

interface SingleInstanceWindowLike {
  isMinimized(): boolean;
  restore(): void;
  show(): void;
  focus(): void;
}

export function restoreSingleInstanceWindow(window: SingleInstanceWindowLike | null) {
  if (!window) {
    return;
  }

  if (window.isMinimized()) {
    window.restore();
  }

  window.show();
  window.focus();
}

export function wireSingleInstanceLifecycle(
  appLike: SingleInstanceAppLike,
  getWindow: () => SingleInstanceWindowLike | null,
) {
  const hasLock = appLike.requestSingleInstanceLock();
  if (!hasLock) {
    appLike.quit();
    return false;
  }

  appLike.on("second-instance", () => {
    restoreSingleInstanceWindow(getWindow());
  });

  return true;
}
