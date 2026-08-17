import { describe, expect, test } from "vitest";
import {
  isLauncherThemeMode,
  launcherThemes,
  resolveLauncherEffectiveTheme,
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
  test("resolves and validates theme modes", () => {
    expect(resolveLauncherEffectiveTheme("system", false)).toBe("light");
    expect(resolveLauncherEffectiveTheme("system", true)).toBe("dark");
    expect(isLauncherThemeMode("sepia")).toBe(false);
  });

  test.each(["light", "dark"] as const)("maps %s semantic colors into Fluent roles", (mode) => {
    const theme = launcherThemes[mode];
    const fluentTheme = launcherFluentThemes[mode];
    expect(fluentTheme.colorBrandBackground).toBe(theme.brandFill);
    expect(fluentTheme.colorBrandForegroundLink).toBe(theme.brandForeground);
    expect(fluentTheme.colorNeutralForegroundOnBrand).toBe(theme.onBrand);
  });

  test.each(["light", "dark"] as const)("keeps %s text and interaction contrast accessible", (mode) => {
    const theme = launcherThemes[mode];
    expect(contrast(theme.text, theme.canvas)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.text, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.brandForeground, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.attention, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.brandStroke, theme.canvas)).toBeGreaterThanOrEqual(3);
    expect(contrast(theme.onBrand, theme.brandFill)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.success, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.warning, theme.surface)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme.danger, theme.surface)).toBeGreaterThanOrEqual(4.5);
  });

});
