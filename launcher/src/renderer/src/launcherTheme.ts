import { webDarkTheme, webLightTheme, type Theme } from "@fluentui/react-components";
import {
  launcherThemes,
  type LauncherEffectiveTheme,
} from "@shared/launcher-theme";

const fluentBaseThemes: Record<LauncherEffectiveTheme, Theme> = {
  light: webLightTheme,
  dark: webDarkTheme,
};

export const launcherFluentThemes: Record<LauncherEffectiveTheme, Theme> = {
  light: createLauncherFluentTheme("light"),
  dark: createLauncherFluentTheme("dark"),
};

function createLauncherFluentTheme(effectiveTheme: LauncherEffectiveTheme): Theme {
  const base = fluentBaseThemes[effectiveTheme];
  const tokens = launcherThemes[effectiveTheme];

  return {
    ...base,
    colorNeutralBackground1: tokens.surface,
    colorNeutralBackground2: tokens.canvas,
    colorNeutralBackground3: tokens.surfaceRaised,
    colorNeutralForeground1: tokens.text,
    colorNeutralForeground2: tokens.textMuted,
    colorNeutralStroke1: tokens.border,
    colorNeutralStroke2: tokens.border,
    colorBrandBackground: tokens.coolAction,
    colorBrandBackgroundHover: `color-mix(in srgb, ${tokens.coolAction} 88%, ${tokens.text})`,
    colorBrandBackgroundPressed: `color-mix(in srgb, ${tokens.coolAction} 78%, ${tokens.text})`,
    colorBrandBackground2: tokens.coolSoft,
    colorBrandBackground2Hover: `color-mix(in srgb, ${tokens.coolAction} 14%, ${tokens.surface})`,
    colorBrandBackground2Pressed: `color-mix(in srgb, ${tokens.coolAction} 20%, ${tokens.surface})`,
    colorBrandForeground1: tokens.coolAction,
    colorBrandForeground2: tokens.coolAction,
    colorBrandForegroundLink: tokens.coolAction,
    colorBrandStroke1: tokens.coolAction,
    colorBrandStroke2: tokens.coolAction,
    colorCompoundBrandBackground: tokens.coolAction,
    colorCompoundBrandBackgroundHover: `color-mix(in srgb, ${tokens.coolAction} 88%, ${tokens.text})`,
    colorCompoundBrandBackgroundPressed: `color-mix(in srgb, ${tokens.coolAction} 78%, ${tokens.text})`,
    colorCompoundBrandStroke: tokens.coolAction,
    colorCompoundBrandStrokeHover: tokens.coolAction,
    colorCompoundBrandStrokePressed: tokens.coolAction,
    colorNeutralForegroundOnBrand: tokens.onAction,
    colorStrokeFocus2: tokens.coolAction,
    colorStatusSuccessForeground1: tokens.success,
    colorStatusWarningForeground1: tokens.warning,
    colorStatusDangerForeground1: tokens.danger,
  };
}

export function applyLauncherDocumentTheme(effectiveTheme: LauncherEffectiveTheme): void {
  if (typeof document === "undefined") {
    return;
  }

  const root = document.documentElement;
  const tokens = launcherThemes[effectiveTheme];
  root.dataset.theme = effectiveTheme;
  root.style.colorScheme = effectiveTheme;

  const variables: Record<string, string> = {
    "--color-canvas": tokens.canvas,
    "--color-surface": tokens.surface,
    "--color-surface-raised": tokens.surfaceRaised,
    "--color-text": tokens.text,
    "--color-text-muted": tokens.textMuted,
    "--color-border": tokens.border,
    "--color-cool": tokens.coolAction,
    "--color-warm": tokens.warmAttention,
    "--color-cool-soft": tokens.coolSoft,
    "--color-warm-soft": tokens.warmSoft,
    "--color-success": tokens.success,
    "--color-warning": tokens.warning,
    "--color-danger": tokens.danger,
    "--color-on-action": tokens.onAction,
    "--color-on-attention": tokens.onAttention,
    "--shadow-surface": tokens.shadowSurface,
    "--shadow-floating": tokens.shadowFloating,
  };

  for (const [name, value] of Object.entries(variables)) {
    root.style.setProperty(name, value);
  }
}
