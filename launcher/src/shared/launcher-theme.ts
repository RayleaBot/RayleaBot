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
    canvas: "#F3F6F7",
    surface: "#FAF9F5",
    surfaceRaised: "#FFFFFF",
    text: "#1F272C",
    textMuted: "#58656E",
    border: "#D8E0E4",
    coolAction: "#0B6B8F",
    warmAttention: "#A44F32",
    coolSoft: "#E8F6FC",
    warmSoft: "#F8ECE7",
    success: "#2F7D5C",
    warning: "#8A5600",
    danger: "#C2414B",
    onAction: "#FFFFFF",
    onAttention: "#FFFFFF",
    shadowSurface: "0 10px 28px rgb(31 39 44 / 8%)",
    shadowFloating: "0 18px 48px rgb(31 39 44 / 18%)",
  },
  dark: {
    canvas: "#11181C",
    surface: "#182126",
    surfaceRaised: "#202C32",
    text: "#E9F0F2",
    textMuted: "#A7B4BA",
    border: "#314047",
    coolAction: "#66CCFF",
    warmAttention: "#D97757",
    coolSoft: "#16323F",
    warmSoft: "#34231E",
    success: "#67C99B",
    warning: "#F0B95A",
    danger: "#FF8089",
    onAction: "#11181C",
    onAttention: "#11181C",
    shadowSurface: "0 10px 28px rgb(0 0 0 / 22%)",
    shadowFloating: "0 18px 48px rgb(0 0 0 / 42%)",
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
