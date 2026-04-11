import type { Component } from 'vue'
import {
  Activity,
  Command,
  LayoutDashboard,
  Plug,
  Settings,
  SquareTerminal,
  Sword,
} from 'lucide-vue-next'

export interface NavigationItem {
  path: string
  labelKey: string
  icon: Component
  children?: Array<{
    path: string
    labelKey: string
  }>
}

export const navigationItems: NavigationItem[] = [
  { path: '/', labelKey: 'routes.status', icon: LayoutDashboard },
  { path: '/plugins', labelKey: 'routes.plugins', icon: Plug },
  { path: '/commands', labelKey: 'routes.commands', icon: Command },
  { path: '/tasks', labelKey: 'routes.tasks', icon: Sword },
  { path: '/logs', labelKey: 'routes.logs', icon: SquareTerminal },
  {
    path: '/protocols',
    labelKey: 'routes.protocols',
    icon: Activity,
    children: [
      { path: '/protocols/logs', labelKey: 'routes.protocolLogs' },
    ],
  },
  { path: '/config', labelKey: 'routes.config', icon: Settings },
]
