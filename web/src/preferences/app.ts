import theme from 'ant-design-vue/es/theme'
import type { ThemeConfig } from 'ant-design-vue/es/config-provider/context'

import { webThemes } from '@/preferences/theme-tokens'

export type ThemeMode = 'system' | 'light' | 'dark'
export type ResolvedThemeMode = 'light' | 'dark'
export type DensityMode = 'compact' | 'default'
export type ContentWidth = 'fixed' | 'wide'
export type PageTransition = 'fade' | 'fade-slide' | 'none'

export interface LayoutPreferences {
  chromeTabbar: boolean
  contentWidth: ContentWidth
  density: DensityMode
  layoutMode: 'sidebar-nav'
  pageTransition: PageTransition
  rememberTabs: boolean
  themeMode: ThemeMode
}

export const defaultLayoutPreferences: LayoutPreferences = {
  chromeTabbar: true,
  contentWidth: 'wide',
  density: 'default',
  layoutMode: 'sidebar-nav',
  pageTransition: 'fade-slide',
  rememberTabs: true,
  themeMode: 'system',
}

export function normalizeLayoutPreferences(
  value?: Partial<LayoutPreferences> | null,
): LayoutPreferences {
  const nextValue = value ?? {}
  const themeMode: ThemeMode = nextValue.themeMode === 'dark'
    || nextValue.themeMode === 'light'
    || nextValue.themeMode === 'system'
    ? nextValue.themeMode
    : defaultLayoutPreferences.themeMode
  const density: DensityMode = nextValue.density === 'compact' ? 'compact' : 'default'
  const contentWidth: ContentWidth = nextValue.contentWidth === 'fixed' ? 'fixed' : 'wide'
  const pageTransition: PageTransition = nextValue.pageTransition === 'fade'
    || nextValue.pageTransition === 'fade-slide'
    || nextValue.pageTransition === 'none'
    ? nextValue.pageTransition
    : defaultLayoutPreferences.pageTransition

  return {
    chromeTabbar: nextValue.chromeTabbar !== false,
    contentWidth,
    density,
    layoutMode: 'sidebar-nav',
    pageTransition,
    rememberTabs: nextValue.rememberTabs !== false,
    themeMode,
  }
}

export function resolveThemeConfig(
  resolvedThemeMode: ResolvedThemeMode,
  density: DensityMode,
): ThemeConfig {
  const isDark = resolvedThemeMode === 'dark'
  const controlHeight = 36
  const tokens = webThemes[resolvedThemeMode]

  return {
    algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      colorBgBase: tokens.canvas,
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
      colorSuccess: tokens.success,
      colorText: tokens.text,
      colorTextLightSolid: tokens.onBrand,
      colorTextSecondary: tokens.textMuted,
      colorWarning: tokens.warning,
      controlOutline: tokens.focus,
      borderRadius: 10,
      borderRadiusLG: 14,
      borderRadiusSM: 6,
      controlHeight,
      fontFamily: 'Inter, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Noto Sans SC", "Microsoft YaHei", system-ui, sans-serif',
      fontSize: 14,
      wireframe: false,
    },
    components: {
      Button: {
        controlHeight,
      },
      Card: {
        borderRadiusLG: 14,
      },
      Input: {
        controlHeight,
      },
      Layout: {
        bodyBg: tokens.canvas,
        headerBg: tokens.surface,
        siderBg: tokens.chrome,
        triggerBg: tokens.canvas,
      },
      Menu: {
        darkItemBg: tokens.chrome,
        darkItemColor: tokens.chromeMuted,
        darkItemHoverBg: tokens.navHover,
        darkItemSelectedBg: tokens.navSelected,
        darkItemSelectedColor: tokens.navSelectedText,
        darkSubMenuItemBg: tokens.chrome,
        itemBg: 'transparent',
        itemSelectedBg: tokens.brandSoft,
        itemSelectedColor: tokens.brandForeground,
        borderRadius: 10,
      },
      Select: {
        controlHeight,
      },
      Table: {
        headerBg: tokens.canvas,
        headerColor: tokens.textMuted,
        rowHoverBg: `color-mix(in srgb, ${tokens.brandFill} ${isDark ? '5%' : '9%'}, ${tokens.surface})`,
      },
    },
  }
}

export function resolvePreferenceCssVariables(preferences: LayoutPreferences) {
  const compact = preferences.density === 'compact'

  return {
    '--app-content-max-width': preferences.contentWidth === 'fixed' ? '1240px' : 'none',
    '--app-control-height': '36px',
    '--app-layout-gap': compact ? '12px' : '16px',
    '--app-page-gap': compact ? '16px' : '24px',
    '--app-page-header-gap': compact ? '16px' : '24px',
    '--app-page-toolbar-gap': compact ? '12px' : '16px',
    '--app-shell-padding-inline': compact ? '16px' : '24px',
    '--app-shell-padding-block': compact ? '16px' : '24px',
  }
}
