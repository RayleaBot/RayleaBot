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
    colorNeutralStroke1: tokens.borderControl,
    colorNeutralStroke2: tokens.border,
    colorBrandBackground: tokens.brandFill,
    colorBrandBackgroundHover: tokens.brandFillHover,
    colorBrandBackgroundPressed: tokens.brandFillPressed,
    colorBrandBackground2: tokens.brandSoft,
    colorBrandBackground2Hover: tokens.surfaceSoft,
    colorBrandBackground2Pressed: tokens.brandSoft,
    colorBrandForeground1: tokens.brandForeground,
    colorBrandForeground2: tokens.brandForeground,
    colorBrandForegroundLink: tokens.brandForeground,
    colorBrandStroke1: tokens.brandStroke,
    colorBrandStroke2: tokens.brandStroke,
    colorCompoundBrandBackground: tokens.brandFill,
    colorCompoundBrandBackgroundHover: tokens.brandFillHover,
    colorCompoundBrandBackgroundPressed: tokens.brandFillPressed,
    colorCompoundBrandStroke: tokens.brandStroke,
    colorCompoundBrandStrokeHover: tokens.brandStroke,
    colorCompoundBrandStrokePressed: tokens.brandStroke,
    colorNeutralForegroundOnBrand: tokens.onBrand,
    colorStrokeFocus2: tokens.focus,
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
    "--color-surface-soft": tokens.surfaceSoft,
    "--color-text": tokens.text,
    "--color-text-muted": tokens.textMuted,
    "--color-border": tokens.border,
    "--color-border-control": tokens.borderControl,
    "--color-brand-fill": tokens.brandFill,
    "--color-brand-fill-hover": tokens.brandFillHover,
    "--color-brand-fill-pressed": tokens.brandFillPressed,
    "--color-brand-foreground": tokens.brandForeground,
    "--color-brand-stroke": tokens.brandStroke,
    "--color-brand-soft": tokens.brandSoft,
    "--color-focus": tokens.focus,
    "--color-chrome": tokens.chrome,
    "--color-chrome-text": tokens.chromeText,
    "--color-chrome-muted": tokens.chromeMuted,
    "--color-nav-hover": tokens.navHover,
    "--color-nav-selected": tokens.navSelected,
    "--color-nav-selected-text": tokens.navSelectedText,
    "--color-on-brand": tokens.onBrand,
    "--color-cool": tokens.brandForeground,
    "--color-attention": tokens.attention,
    "--color-warm": tokens.attention,
    "--color-cool-soft": tokens.brandSoft,
    "--color-attention-soft": tokens.attentionSoft,
    "--color-warm-soft": tokens.attentionSoft,
    "--color-success": tokens.success,
    "--color-success-soft": tokens.successSoft,
    "--color-warning": tokens.warning,
    "--color-warning-soft": tokens.warningSoft,
    "--color-danger": tokens.danger,
    "--color-danger-soft": tokens.dangerSoft,
    "--color-on-action": tokens.onBrand,
    "--color-on-attention": tokens.onAttention,
    "--shadow-surface": tokens.shadowSurface,
    "--shadow-floating": tokens.shadowFloating,
  };

  for (const [name, value] of Object.entries(variables)) {
    root.style.setProperty(name, value);
  }
}
