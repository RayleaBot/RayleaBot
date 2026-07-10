import { describe, expect, test } from "vitest";
import {
  isLauncherThemeMode,
  launcherThemes,
  resolveLauncherEffectiveTheme,
  resolveLauncherWindowBackground,
} from "@shared/launcher-theme";

function luminance(hex: string) {
  const channels = hex.slice(1).match(/.{2}/g)!.map((channel) => Number.parseInt(channel, 16) / 255);
  const [r, g, b] = channels.map((channel) =>
    channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4,
  );
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(foreground: string, background: string) {
  const light = Math.max(luminance(foreground), luminance(background));
  const dark = Math.min(luminance(foreground), luminance(background));
  return (light + 0.05) / (dark + 0.05);
}

describe("launcher themes", () => {
  test("maps the project semantic colors into complete light and dark themes", () => {
    expect(launcherThemes.light).toMatchObject({
      canvas: "#F3F6F7",
      surface: "#FAF9F5",
      coolAction: "#0B6B8F",
      warmAttention: "#A44F32",
    });
    expect(launcherThemes.dark).toMatchObject({
      canvas: "#11181C",
      surface: "#182126",
      coolAction: "#66CCFF",
      warmAttention: "#D97757",
    });
  });

  test("resolves system mode and window backgrounds from the same source", () => {
    expect(resolveLauncherEffectiveTheme("system", false)).toBe("light");
    expect(resolveLauncherEffectiveTheme("system", true)).toBe("dark");
    expect(resolveLauncherWindowBackground("light")).toBe(launcherThemes.light.canvas);
    expect(resolveLauncherWindowBackground("dark")).toBe(launcherThemes.dark.canvas);
    expect(isLauncherThemeMode("sepia")).toBe(false);
  });

  test.each(["light", "dark"] as const)("keeps %s text and interaction contrast accessible", (mode) => {
    const theme = launcherThemes[mode];
    expect(contrast(theme.text, theme.canvas)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.text, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.coolAction, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.warmAttention, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.coolAction, theme.canvas)).toBeGreaterThanOrEqual(3);
    expect(contrast(theme.success, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.warning, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.danger, theme.surface)).toBeGreaterThanOrEqual(4.5);
  });

  test("does not use signature colors as light-theme ordinary text", () => {
    expect(launcherThemes.light.text).not.toBe("#66CCFF");
    expect(launcherThemes.light.textMuted).not.toBe("#66CCFF");
    expect(launcherThemes.light.text).not.toBe("#D97757");
    expect(launcherThemes.light.textMuted).not.toBe("#D97757");
  });
});
