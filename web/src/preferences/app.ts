import { theme, type ThemeConfig } from 'ant-design-vue'

export type ThemeMode = 'dark' | 'light'

export interface LayoutPreferences {
  breadcrumb: boolean
  chromeTabbar: boolean
  fixedHeader: boolean
  layoutMode: 'sidebar-nav'
  themeMode: ThemeMode
}

export const defaultLayoutPreferences: LayoutPreferences = {
  breadcrumb: true,
  chromeTabbar: true,
  fixedHeader: true,
  layoutMode: 'sidebar-nav',
  themeMode: 'dark',
}

export function resolveThemeConfig(themeMode: ThemeMode): ThemeConfig {
  const isDark = themeMode === 'dark'

  return {
    algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      colorPrimary: '#1668dc',
      colorInfo: '#1668dc',
      colorSuccess: '#3fbe73',
      colorWarning: '#e4a11b',
      colorError: '#e15b64',
      borderRadius: 14,
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
        darkItemSelectedBg: '#1668dc',
        itemBg: 'transparent',
        itemSelectedBg: '#e8f1ff',
        itemSelectedColor: '#1668dc',
        borderRadius: 12,
      },
      Button: {
        controlHeight: 40,
      },
      Input: {
        controlHeight: 40,
      },
      Select: {
        controlHeight: 40,
      },
      Card: {
        borderRadiusLG: 20,
      },
    },
  }
}
