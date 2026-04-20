import theme from 'ant-design-vue/es/theme'
import type { ThemeConfig } from 'ant-design-vue/es/config-provider/context'

export type ThemeMode = 'dark' | 'light'
export type RadiusLevel = 'sm' | 'md' | 'lg' | 'xl'
export type FontScale = 'sm' | 'md' | 'lg'
export type DensityMode = 'compact' | 'default'
export type ContentWidth = 'fixed' | 'wide'
export type PageTransition = 'fade' | 'fade-slide' | 'none'

export interface LayoutPreferences {
  breadcrumb: boolean
  colorPrimary: string
  chromeTabbar: boolean
  contentWidth: ContentWidth
  density: DensityMode
  fontScale: FontScale
  fixedHeader: boolean
  layoutMode: 'sidebar-nav'
  pageLoading: boolean
  pageTransition: PageTransition
  radiusLevel: RadiusLevel
  rememberTabs: boolean
  themeMode: ThemeMode
}

export const themeColorPresets = [
  '#1677ff',
  '#13c2c2',
  '#52c41a',
  '#722ed1',
  '#fa541c',
] as const

const radiusMap: Record<RadiusLevel, number> = {
  sm: 8,
  md: 10,
  lg: 12,
  xl: 14,
}

const fontScaleMap: Record<FontScale, number> = {
  sm: 13,
  md: 14,
  lg: 15,
}

const densityControlHeightMap: Record<DensityMode, number> = {
  compact: 30,
  default: 34,
}

export const defaultLayoutPreferences: LayoutPreferences = {
  breadcrumb: true,
  colorPrimary: themeColorPresets[0],
  chromeTabbar: true,
  contentWidth: 'wide',
  density: 'default',
  fontScale: 'md',
  fixedHeader: true,
  layoutMode: 'sidebar-nav',
  pageLoading: true,
  pageTransition: 'fade-slide',
  radiusLevel: 'md',
  rememberTabs: true,
  themeMode: 'light',
}

export function normalizeLayoutPreferences(
  value?: Partial<LayoutPreferences> | null,
): LayoutPreferences {
  const nextValue = value ?? {}
  const themeMode = nextValue.themeMode === 'dark' ? 'dark' : 'light'
  const colorPrimary = typeof nextValue.colorPrimary === 'string' && nextValue.colorPrimary.trim()
    ? nextValue.colorPrimary.trim()
    : defaultLayoutPreferences.colorPrimary
  const radiusLevel = nextValue.radiusLevel && radiusMap[nextValue.radiusLevel]
    ? nextValue.radiusLevel
    : defaultLayoutPreferences.radiusLevel
  const fontScale = nextValue.fontScale && fontScaleMap[nextValue.fontScale]
    ? nextValue.fontScale
    : defaultLayoutPreferences.fontScale
  const density = nextValue.density && densityControlHeightMap[nextValue.density]
    ? nextValue.density
    : defaultLayoutPreferences.density
  const contentWidth = nextValue.contentWidth === 'fixed' ? 'fixed' : 'wide'
  const pageTransition = nextValue.pageTransition === 'fade'
    || nextValue.pageTransition === 'fade-slide'
    || nextValue.pageTransition === 'none'
    ? nextValue.pageTransition
    : defaultLayoutPreferences.pageTransition

  return {
    ...defaultLayoutPreferences,
    ...nextValue,
    colorPrimary,
    contentWidth,
    density,
    fontScale,
    pageTransition,
    radiusLevel,
    themeMode,
  }
}

export function resolveThemeConfig(preferences: Pick<
  LayoutPreferences,
  'colorPrimary' | 'density' | 'fontScale' | 'radiusLevel' | 'themeMode'
>): ThemeConfig {
  const isDark = preferences.themeMode === 'dark'
  const colorPrimary = preferences.colorPrimary
  const borderRadius = radiusMap[preferences.radiusLevel]
  const controlHeight = densityControlHeightMap[preferences.density]
  const fontSize = fontScaleMap[preferences.fontScale]

  return {
    algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      colorPrimary,
      colorInfo: colorPrimary,
      colorSuccess: '#3fbe73',
      colorWarning: '#e4a11b',
      colorError: '#e15b64',
      borderRadius,
      borderRadiusLG: borderRadius + 2,
      borderRadiusSM: Math.max(6, borderRadius - 2),
      fontSize,
      wireframe: false,
      fontFamily: '"PingFang SC", "Hiragino Sans GB", "Noto Sans SC", "Microsoft YaHei", sans-serif',
    },
    components: {
      Layout: {
        siderBg: isDark ? '#111827' : '#ffffff',
        headerBg: isDark ? '#0f172a' : '#ffffff',
        bodyBg: 'transparent',
        triggerBg: isDark ? '#0b1220' : '#f5f7fa',
      },
      Menu: {
        darkItemBg: '#111827',
        darkSubMenuItemBg: '#0f172a',
        darkItemSelectedBg: colorPrimary,
        itemBg: 'transparent',
        itemSelectedBg: colorPrimary === '#1677ff' ? '#e8f1ff' : 'color-mix(in srgb, var(--accent) 14%, transparent)',
        itemSelectedColor: colorPrimary,
        borderRadius,
      },
      Button: {
        controlHeight,
      },
      Input: {
        controlHeight,
      },
      Select: {
        controlHeight,
      },
      Card: {
        borderRadiusLG: borderRadius + 2,
      },
    },
  }
}

export function resolvePreferenceCssVariables(preferences: LayoutPreferences) {
  const borderRadius = radiusMap[preferences.radiusLevel]
  const controlHeight = densityControlHeightMap[preferences.density]
  const fontSize = fontScaleMap[preferences.fontScale]
  const compact = preferences.density === 'compact'

  return {
    '--accent': preferences.colorPrimary,
    '--app-primary': preferences.colorPrimary,
    '--app-border-radius': `${borderRadius}px`,
    '--app-card-radius': `${borderRadius + 2}px`,
    '--app-content-max-width': preferences.contentWidth === 'fixed' ? '1380px' : 'none',
    '--app-control-height': `${controlHeight}px`,
    '--app-font-size': `${fontSize}px`,
    '--app-layout-gap': compact ? '10px' : '12px',
    '--app-page-gap': compact ? '10px' : '12px',
    '--app-page-header-gap': compact ? '12px' : '16px',
    '--app-page-toolbar-gap': compact ? '10px' : '12px',
    '--app-shell-padding-inline': compact ? '10px' : '12px',
    '--app-shell-padding-block': compact ? '8px' : '10px',
    '--sider-menu-active': preferences.colorPrimary,
    '--sider-menu-active-bg': 'color-mix(in srgb, var(--accent) 12%, transparent)',
  }
}
