import theme from 'ant-design-vue/es/theme'
import type { ThemeConfig } from 'ant-design-vue/es/config-provider/context'

import type { ResolvedThemeMode } from '@/preferences/app'
import { webThemes } from '@/preferences/theme-tokens'

export function resolveAuthThemeConfig(mode: ResolvedThemeMode): ThemeConfig {
  const tokens = webThemes[mode]
  return {
    algorithm: mode === 'dark' ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      borderRadius: 8,
      borderRadiusLG: 12,
      colorBgContainer: tokens.surface,
      colorBgElevated: tokens.surfaceRaised,
      colorBgLayout: tokens.canvas,
      colorBorder: tokens.borderControl,
      colorBorderSecondary: tokens.border,
      colorError: tokens.danger,
      colorInfo: tokens.brandForeground,
      colorLink: tokens.brandForeground,
      colorLinkActive: tokens.brandFillPressed,
      colorLinkHover: tokens.brandFillHover,
      colorPrimary: tokens.brandFill,
      colorPrimaryActive: tokens.brandFillPressed,
      colorPrimaryHover: tokens.brandFillHover,
      colorSplit: tokens.border,
      colorSuccess: tokens.success,
      colorText: tokens.text,
      colorTextLightSolid: tokens.onBrand,
      colorTextSecondary: tokens.textMuted,
      colorTextPlaceholder: tokens.textMuted,
      colorWarning: tokens.warning,
      controlHeight: 36,
      controlOutline: tokens.focus,
      fontFamily: 'var(--font-sans)',
      fontSize: 14,
      wireframe: false,
    },
    components: {
      Alert: {
        borderRadiusLG: 8,
      },
      Button: {
        controlHeight: 36,
      },
      Input: {
        activeBorderColor: tokens.focus,
        activeShadow: `0 0 0 2px ${tokens.focus}`,
        hoverBorderColor: tokens.brandForeground,
        controlHeight: 36,
      },
    },
  }
}

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
    '--auth-on-brand': tokens.onBrand,
    '--auth-panel-highlight': tokens.surfaceRaised,
    '--auth-panel-shadow': tokens.shadowSurface,
    '--auth-primary-highlight': 'transparent',
    '--auth-primary-shadow': 'none',
    '--auth-surface': tokens.surface,
    '--auth-surface-raised': tokens.surfaceRaised,
    '--auth-text': tokens.text,
    '--auth-text-muted': tokens.textMuted,
  }
}
