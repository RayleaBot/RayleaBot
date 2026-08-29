import type { AppMenuItem } from './menu'
import type { ShellTabItem } from '@/stores/ui-shell'
import { t } from '@/i18n'

export const pluginCenterPath = '/plugins'
export const pluginCenterTabName = 'plugin-center'
export const pluginCenterPages = [
  { name: 'menu-center', path: '/menu-center', titleKey: 'routes.menuCenter', icon: 'menu-center' },
  { name: 'plugin-store', path: '/plugins/store', titleKey: 'routes.pluginStore', icon: 'plugin-store' },
  { name: 'plugins', path: '/plugins', titleKey: 'routes.pluginList', icon: 'plugins' },
  { name: 'plugin-settings', path: '/plugins/settings', titleKey: 'routes.pluginSettings', icon: 'plugin-settings' },
  { name: 'commands', path: '/commands', titleKey: 'routes.commands', icon: 'commands' },
] as const

export function isPluginCenterRoute(name: unknown) {
  return pluginCenterPages.some(page => page.name === name)
}

function isPluginCenterLocation(location: string) {
  const path = location.split(/[?#]/, 1)[0]
  return pluginCenterPages.some(page => page.path === path)
}

export function createPluginCenterTab(fullPath = pluginCenterPath): ShellTabItem {
  return {
    name: pluginCenterTabName,
    path: pluginCenterPath,
    fullPath: isPluginCenterLocation(fullPath) ? fullPath : pluginCenterPath,
    title: t('routes.pluginCenter'),
    icon: 'plugins',
    keepAlive: true,
    affix: false,
  }
}

export function projectPluginCenterMenu(items: AppMenuItem[]): AppMenuItem[] {
  return items.map(item => pluginCenterPages.every(page => item.children?.some(child => child.key === page.name))
    ? { key: pluginCenterTabName, path: pluginCenterPath, title: t('routes.pluginCenter'), icon: 'plugins' }
    : item)
}

export function restorePluginCenterTabs(items: ShellTabItem[]): ShellTabItem[] {
  const belongsToCenter = (item: ShellTabItem) => item.name === pluginCenterTabName
    || (isPluginCenterRoute(item.name) && isPluginCenterLocation(item.path))
  const savedCenter = items.find(item => item.name === pluginCenterTabName)
  const merged = createPluginCenterTab(savedCenter?.fullPath)
  let inserted = false
  return items.flatMap(item => {
    if (!belongsToCenter(item)) return [item]
    if (inserted) return []
    inserted = true
    return [merged]
  })
}
