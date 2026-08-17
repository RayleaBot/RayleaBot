import {
  launcherGeneratedThemes,
  type GeneratedLauncherThemeTokens,
} from "./launcher-theme-tokens.generated";

export const launcherThemeModes = ["system", "light", "dark"] as const;

export type LauncherThemeMode = (typeof launcherThemeModes)[number];
export type LauncherEffectiveTheme = Exclude<LauncherThemeMode, "system">;

export interface LauncherThemeTokens extends GeneratedLauncherThemeTokens {
  brandStroke: string;
}

export const launcherThemes: Record<LauncherEffectiveTheme, LauncherThemeTokens> = {
  light: {
    ...launcherGeneratedThemes.light,
    brandStroke: launcherGeneratedThemes.light.brandForeground,
  },
  dark: {
    ...launcherGeneratedThemes.dark,
    brandStroke: launcherGeneratedThemes.dark.brandForeground,
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
