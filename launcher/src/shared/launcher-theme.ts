import {
  launcherGeneratedThemes,
  type GeneratedLauncherThemeTokens,
} from "./launcher-theme-tokens.generated";

export const launcherThemeModes = ["system", "light", "dark"] as const;

export type LauncherThemeMode = (typeof launcherThemeModes)[number];
export type LauncherEffectiveTheme = Exclude<LauncherThemeMode, "system">;

export interface LauncherThemeTokens extends GeneratedLauncherThemeTokens {
  /** Compatibility alias for brandForeground. */
  brandStroke: string;
  /** @deprecated Use brandForeground. Kept for compatibility with existing consumers. */
  coolAction: string;
  /** @deprecated Use brandSoft. Kept for compatibility with existing consumers. */
  coolSoft: string;
  /** @deprecated Use onBrand. Kept for compatibility with existing consumers. */
  onAction: string;
  /** @deprecated Use attention. Kept for compatibility with existing consumers. */
  warmAttention: string;
  /** @deprecated Use attentionSoft. Kept for compatibility with existing consumers. */
  warmSoft: string;
}

export const launcherThemes: Record<LauncherEffectiveTheme, LauncherThemeTokens> = {
  light: {
    ...launcherGeneratedThemes.light,
    brandStroke: launcherGeneratedThemes.light.brandForeground,
    coolAction: launcherGeneratedThemes.light.brandForeground,
    coolSoft: launcherGeneratedThemes.light.brandSoft,
    onAction: launcherGeneratedThemes.light.onBrand,
    warmAttention: launcherGeneratedThemes.light.attention,
    warmSoft: launcherGeneratedThemes.light.attentionSoft,
  },
  dark: {
    ...launcherGeneratedThemes.dark,
    brandStroke: launcherGeneratedThemes.dark.brandForeground,
    coolAction: launcherGeneratedThemes.dark.brandForeground,
    coolSoft: launcherGeneratedThemes.dark.brandSoft,
    onAction: launcherGeneratedThemes.dark.onBrand,
    warmAttention: launcherGeneratedThemes.dark.attention,
    warmSoft: launcherGeneratedThemes.dark.attentionSoft,
  },
};

export function isLauncherThemeMode(value: unknown): value is LauncherThemeMode {
  return typeof value === "string" && launcherThemeModes.includes(value as LauncherThemeMode);
}

export function resolveLauncherEffectiveTheme(
  mode: LauncherThemeMode,
  prefersDark: boolean,
): LauncherEffectiveTheme {
  return mode === "system" ? (prefersDark ? "dark" : "light") : mode;
}

export function resolveLauncherWindowBackground(theme: LauncherEffectiveTheme): string {
  return launcherThemes[theme].canvas;
}
