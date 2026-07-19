import { describe, expect, test } from "vitest";
import {
  isLauncherThemeMode,
  launcherThemes,
  resolveLauncherEffectiveTheme,
  resolveLauncherWindowBackground,
} from "@shared/launcher-theme";
import { launcherFluentThemes } from "@renderer/launcherTheme";

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
      canvas: "#F6F3F5",
      surface: "#FFFBFD",
      brandFill: "#8A285D",
      brandForeground: "#8A285D",
      warmAttention: "#9B4A2F",
    });
    expect(launcherThemes.dark).toMatchObject({
      canvas: "#151114",
      surface: "#1D181C",
      brandFill: "#D57BA6",
      brandForeground: "#F0A4C9",
      warmAttention: "#E08A61",
    });
  });

  test("resolves system mode and window backgrounds from the same source", () => {
    expect(resolveLauncherEffectiveTheme("system", false)).toBe("light");
    expect(resolveLauncherEffectiveTheme("system", true)).toBe("dark");
    expect(resolveLauncherWindowBackground("light")).toBe(launcherThemes.light.canvas);
    expect(resolveLauncherWindowBackground("dark")).toBe(launcherThemes.dark.canvas);
    expect(isLauncherThemeMode("sepia")).toBe(false);
  });

  test("keeps brand fills separate from accessible foreground roles", () => {
    expect(launcherFluentThemes.light.colorBrandBackground).toBe("#8A285D");
    expect(launcherFluentThemes.light.colorBrandForegroundLink).toBe("#8A285D");
    expect(launcherFluentThemes.light.colorNeutralForegroundOnBrand).toBe("#FFFFFF");
    expect(launcherFluentThemes.dark.colorBrandBackground).toBe("#D57BA6");
    expect(launcherThemes.light.coolAction).toBe(launcherThemes.light.brandForeground);
    expect(launcherThemes.light.coolSoft).toBe(launcherThemes.light.brandSoft);
    expect(launcherThemes.light.onAction).toBe(launcherThemes.light.onBrand);
  });

  test.each(["light", "dark"] as const)("keeps %s text and interaction contrast accessible", (mode) => {
    const theme = launcherThemes[mode];
    expect(contrast(theme.text, theme.canvas)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.text, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.brandForeground, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.warmAttention, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.brandStroke, theme.canvas)).toBeGreaterThanOrEqual(3);
    expect(contrast(theme.onBrand, theme.brandFill)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.success, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.warning, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.danger, theme.surface)).toBeGreaterThanOrEqual(4.5);
  });

  test("does not use action colors as ordinary text", () => {
    expect(launcherThemes.light.text).not.toBe(launcherThemes.light.brandFill);
    expect(launcherThemes.light.textMuted).not.toBe(launcherThemes.light.brandFill);
    expect(launcherThemes.dark.text).not.toBe(launcherThemes.dark.brandFill);
    expect(launcherThemes.dark.textMuted).not.toBe(launcherThemes.dark.brandFill);
  });
});
