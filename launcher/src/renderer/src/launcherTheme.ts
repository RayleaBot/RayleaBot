import {
  launcherThemes,
  type LauncherEffectiveTheme,
} from "@shared/launcher-theme";

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
    "--color-cool": tokens.textMuted,
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
    "--shadow-raised": tokens.shadowRaised,
    "--shadow-floating": tokens.shadowFloating,
  };

  for (const [name, value] of Object.entries(variables)) {
    root.style.setProperty(name, value);
  }
}
