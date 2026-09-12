import type { Component } from 'vue'
import {
  PlugZapIcon,
  BlocksIcon,
  NetworkIcon,
  LayersIcon,
  TerminalIcon,
  GaugeIcon,
  CalendarClockIcon,
  FileSearchIcon,
  FileTextIcon,
  ServerIcon,
  HistoryIcon,
  LogInIcon,
  MenuIcon,
  MonitorIcon,
  ShieldCheckIcon,
  SettingsIcon,
  StoreIcon,
  SlidersHorizontalIcon,
  BanIcon,
  TableIcon,
  ZapIcon,
  WrenchIcon,
} from '@lucide/vue'

const iconMap: Record<string, Component> = {
  'access-lists': BanIcon,
  commands: TerminalIcon,
  config: SlidersHorizontalIcon,
  connections: NetworkIcon,
  dashboard: GaugeIcon,
  features: LayersIcon,
  'history-logs': HistoryIcon,
  login: LogInIcon,
  logs: FileSearchIcon,
  'menu-center': MenuIcon,
  'permission-policy': ShieldCheckIcon,
  'plugin-settings': SettingsIcon,
  'plugin-store': StoreIcon,
  plugins: BlocksIcon,
  'protocol-compatibility': TableIcon,
  protocols: PlugZapIcon,
  'rate-limits': ZapIcon,
  'render-templates': FileTextIcon,
  runtime: MonitorIcon,
  scheduler: CalendarClockIcon,
  system: ServerIcon,
  toolbox: WrenchIcon,
}

export function resolveMenuIcon(icon?: string | null) {
  if (!icon) {
    return null
  }

  return iconMap[icon] ?? null
}
