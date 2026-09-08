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

export function resolvePreferenceCssVariables(preferences: LayoutPreferences) {
  const compact = preferences.density === 'compact'

  return {
    '--app-content-max-width': preferences.contentWidth === 'fixed' ? '1240px' : 'none',
    '--app-control-height': '36px',
    '--app-layout-gap': compact ? '12px' : '16px',
    '--app-page-gap': compact ? '12px' : '16px',
    '--app-page-header-gap': compact ? '12px' : '16px',
    '--app-page-toolbar-gap': compact ? '12px' : '16px',
    '--app-shell-padding-inline': compact ? '16px' : '24px',
    '--app-shell-padding-block': compact ? '12px' : '20px',
  }
}
