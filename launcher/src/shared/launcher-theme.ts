export const launcherThemeModes = ["system", "light", "dark"] as const;

export type LauncherThemeMode = (typeof launcherThemeModes)[number];
export type LauncherEffectiveTheme = Exclude<LauncherThemeMode, "system">;

export interface LauncherThemeTokens {
  canvas: string;
  surface: string;
  surfaceRaised: string;
  text: string;
  textMuted: string;
  border: string;
  coolAction: string;
  warmAttention: string;
  coolSoft: string;
  warmSoft: string;
  success: string;
  warning: string;
  danger: string;
  onAction: string;
  onAttention: string;
  shadowSurface: string;
  shadowFloating: string;
}

export const launcherThemes: Record<LauncherEffectiveTheme, LauncherThemeTokens> = {
  light: {
    canvas: "#EDF2F4",
    surface: "#F8FAFB",
    surfaceRaised: "#FFFFFF",
    text: "#1B2328",
    textMuted: "#5B6873",
    border: "#D9E1E6",
    coolAction: "#0A6E94",
    warmAttention: "#B04A2E",
    coolSoft: "#E3F2F9",
    warmSoft: "#F9EDE8",
    success: "#23795A",
    warning: "#8F5A00",
    danger: "#C2404A",
    onAction: "#FFFFFF",
    onAttention: "#FFFFFF",
    shadowSurface: "0 1px 2px rgb(20 33 41 / 4%), 0 10px 30px rgb(20 33 41 / 7%)",
    shadowFloating: "0 2px 6px rgb(20 33 41 / 6%), 0 24px 60px rgb(20 33 41 / 16%)",
  },
  dark: {
    canvas: "#0D1417",
    surface: "#131D22",
    surfaceRaised: "#1B2830",
    text: "#E9F1F3",
    textMuted: "#9FB0B8",
    border: "#2A3A42",
    coolAction: "#5EC9F2",
    warmAttention: "#DE7E58",
    coolSoft: "#13303E",
    warmSoft: "#33241D",
    success: "#6CCE9D",
    warning: "#F0BC5F",
    danger: "#FF828B",
    onAction: "#082029",
    onAttention: "#082029",
    shadowSurface: "0 1px 2px rgb(0 0 0 / 24%), 0 12px 32px rgb(0 0 0 / 26%)",
    shadowFloating: "0 2px 8px rgb(0 0 0 / 28%), 0 28px 64px rgb(0 0 0 / 46%)",
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
