import { describe, expect, test, vi } from "vitest";
import { applyLauncherThemeMode, syncLauncherWindowBackground } from "@main/services/launcher-theme";
import { launcherThemes } from "@shared/launcher-theme";

describe("native launcher theme", () => {
  test("applies the selected mode and synchronizes the BrowserWindow background", () => {
    const nativeTheme = { themeSource: "system" as const, shouldUseDarkColors: false };
    const setBackgroundColor = vi.fn();

    applyLauncherThemeMode(nativeTheme, { setBackgroundColor }, "dark");

    expect(nativeTheme.themeSource).toBe("dark");
    expect(setBackgroundColor).toHaveBeenLastCalledWith(launcherThemes.dark.canvas);
  });

  test("refreshes the window background after the effective system theme changes", () => {
    const nativeTheme = { themeSource: "system" as const, shouldUseDarkColors: true };
    const setBackgroundColor = vi.fn();

    syncLauncherWindowBackground(nativeTheme, { setBackgroundColor });

    expect(setBackgroundColor).toHaveBeenCalledWith(launcherThemes.dark.canvas);
  });
});
