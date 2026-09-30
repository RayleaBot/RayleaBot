import type { ResolvedThemeMode } from '@/preferences/app'
import { webThemes } from '@/preferences/theme-tokens'

// The authentication pages read the shared theme through one set of --auth-* variables: a white page, a
// light gray panel and white controls, like the management workspace.
export function resolveAuthCssVariables(mode: ResolvedThemeMode): Record<string, string> {
  const tokens = webThemes[mode]
  return {
    '--auth-border': tokens.border,
    '--auth-border-control': tokens.borderControl,
    '--auth-canvas': tokens.canvas,
    '--auth-control': tokens.surfaceRaised,
    '--auth-brand-fill': tokens.brandFill,
    '--auth-brand-fill-hover': tokens.brandFillHover,
    '--auth-brand-fill-pressed': tokens.brandFillPressed,
    '--auth-brand-foreground': tokens.brandForeground,
    '--auth-brand-soft': tokens.brandSoft,
    '--auth-focus': tokens.focus,
    '--auth-danger': tokens.danger,
    '--auth-on-brand': tokens.onBrand,
    '--auth-surface': tokens.surface,
    '--auth-text': tokens.text,
    '--auth-text-muted': tokens.textMuted,
  }
}
