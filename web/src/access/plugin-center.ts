import type { AppMenuItem } from './menu'
import { t } from '@/i18n'

export const pluginCenterPath = '/plugins'
export const pluginCenterMenuKey = 'plugin-center'
export const pluginCenterPages = [
  { name: 'plugins', path: '/plugins', titleKey: 'routes.pluginList', icon: 'plugins', group: 'management' },
  { name: 'plugin-store', path: '/plugins/store', titleKey: 'routes.pluginStore', icon: 'plugin-store', group: 'management' },
  { name: 'plugin-settings', path: '/plugins/settings', titleKey: 'routes.pluginSettings', icon: 'plugin-settings', group: 'management' },
  { name: 'menu-center', path: '/menu-center', titleKey: 'routes.menuCenter', icon: 'menu-center', group: 'tools' },
  { name: 'commands', path: '/commands', titleKey: 'routes.commands', icon: 'commands', group: 'tools' },
] as const

export const pluginCenterPageGroups = [
  { key: 'management', titleKey: 'plugins.navigation.groups.management' },
  { key: 'tools', titleKey: 'plugins.navigation.groups.tools' },
] as const

export function isPluginCenterRoute(name: unknown) {
  return pluginCenterPages.some(page => page.name === name)
}

export function isPluginWorkspaceRoute(name: unknown) {
  return isPluginCenterRoute(name) || name === 'plugin-detail'
}

export function projectPluginCenterMenu(items: AppMenuItem[]): AppMenuItem[] {
  return items.map(item => pluginCenterPages.every(page => item.children?.some(child => child.key === page.name))
    ? { key: pluginCenterMenuKey, path: pluginCenterPath, title: t('routes.pluginCenter'), icon: 'plugins' }
    : item)
}
