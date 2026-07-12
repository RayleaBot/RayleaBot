import theme from 'ant-design-vue/es/theme'
import type { ThemeConfig } from 'ant-design-vue/es/config-provider/context'

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
  const surface = isDark ? '#182126' : '#FAF9F5'
  const canvas = isDark ? '#11181C' : '#F3F6F7'
  const text = isDark ? '#E9F0F2' : '#1F272C'
  const textSecondary = isDark ? '#A7B4BA' : '#58656E'
  const border = isDark ? '#314047' : '#D8E0E4'
  const primary = isDark ? '#66CCFF' : '#0B6B8F'

  return {
    algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      colorBgBase: canvas,
      colorBgContainer: surface,
      colorBgElevated: surface,
      colorBgLayout: canvas,
      colorBorder: border,
      colorBorderSecondary: border,
      colorError: isDark ? '#FF8089' : '#C2414B',
      colorInfo: primary,
      colorPrimary: primary,
      colorSuccess: isDark ? '#67C99B' : '#2F7D5C',
      colorText: text,
      colorTextSecondary: textSecondary,
      colorWarning: isDark ? '#F0B95A' : '#8A5600',
      borderRadius: 8,
      borderRadiusLG: 12,
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
        borderRadiusLG: 12,
      },
      Input: {
        controlHeight,
      },
      Layout: {
        bodyBg: canvas,
        headerBg: surface,
        siderBg: surface,
        triggerBg: canvas,
      },
      Menu: {
        darkItemBg: surface,
        darkSubMenuItemBg: surface,
        darkItemSelectedBg: 'color-mix(in srgb, #66CCFF 14%, #182126)',
        itemBg: 'transparent',
        itemSelectedBg: isDark
          ? 'color-mix(in srgb, #66CCFF 14%, #182126)'
          : 'color-mix(in srgb, #0B6B8F 10%, #FAF9F5)',
        itemSelectedColor: primary,
        borderRadius: 8,
      },
      Select: {
        controlHeight,
      },
      Table: {
        headerBg: isDark ? '#1E292F' : '#F3F6F7',
        headerColor: textSecondary,
        rowHoverBg: isDark
          ? 'color-mix(in srgb, #66CCFF 5%, #182126)'
          : 'color-mix(in srgb, #0B6B8F 4%, #FAF9F5)',
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
