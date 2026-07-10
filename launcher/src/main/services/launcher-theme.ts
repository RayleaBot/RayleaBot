import type { LauncherThemeMode } from "../../shared/launcher-theme";
import {
  resolveLauncherWindowBackground,
} from "../../shared/launcher-theme";

interface NativeThemeLike {
  themeSource: LauncherThemeMode;
  readonly shouldUseDarkColors: boolean;
}

interface ThemeWindowLike {
  setBackgroundColor(color: string): void;
}

export function syncLauncherWindowBackground(
  nativeTheme: NativeThemeLike,
  window: ThemeWindowLike | null,
): void {
  if (!window) {
    return;
  }

  window.setBackgroundColor(
    resolveLauncherWindowBackground(nativeTheme.shouldUseDarkColors ? "dark" : "light"),
  );
}

export function applyLauncherThemeMode(
  nativeTheme: NativeThemeLike,
  window: ThemeWindowLike | null,
  mode: LauncherThemeMode,
): void {
  nativeTheme.themeSource = mode;
  if (!window) {
    return;
  }
  const effectiveTheme = mode === "system"
    ? (nativeTheme.shouldUseDarkColors ? "dark" : "light")
    : mode;
  window.setBackgroundColor(resolveLauncherWindowBackground(effectiveTheme));
}
