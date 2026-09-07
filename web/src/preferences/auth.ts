import type { ResolvedThemeMode } from '@/preferences/app'
import { webThemes } from '@/preferences/theme-tokens'

export function resolveAuthCssVariables(mode: ResolvedThemeMode): Record<string, string> {
  const tokens = webThemes[mode]
  return {
    '--auth-border': tokens.border,
    '--auth-border-control': tokens.borderControl,
    '--auth-canvas': tokens.canvas,
    '--auth-canvas-focus': tokens.authCanvasFocus,
    '--auth-canvas-wash': tokens.authCanvasWash,
    '--auth-control': tokens.authControl,
    '--auth-control-hover': tokens.authControlHover,
    '--auth-brand-fill': tokens.brandFill,
    '--auth-brand-fill-hover': tokens.brandFillHover,
    '--auth-brand-fill-pressed': tokens.brandFillPressed,
    '--auth-brand-foreground': tokens.brandForeground,
    '--auth-brand-stroke': tokens.brandForeground,
    '--auth-brand-soft': tokens.brandSoft,
    '--auth-focus': tokens.focus,
    '--auth-danger': tokens.danger,
    '--auth-on-brand': tokens.onBrand,
    '--auth-panel-highlight': tokens.surfaceRaised,
    '--auth-panel-shadow': tokens.shadowFloating,
    '--auth-primary-highlight': 'transparent',
    '--auth-primary-shadow': 'none',
    '--auth-surface': tokens.surface,
    '--auth-surface-raised': tokens.surfaceRaised,
    '--auth-text': tokens.text,
    '--auth-text-muted': tokens.textMuted,
  }
}
