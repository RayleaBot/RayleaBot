import type { Component } from 'vue'
import {
  AppstoreOutlined,
  BarsOutlined,
  ControlOutlined,
  FileTextOutlined,
  MenuOutlined,
  DashboardOutlined,
  DisconnectOutlined,
  FileSearchOutlined,
  HddOutlined,
  LoginOutlined,
  ProfileOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  StopOutlined,
  ThunderboltOutlined,
  ToolOutlined,
} from '@ant-design/icons-vue'

const iconMap: Record<string, Component> = {
  appstore: AppstoreOutlined,
  'access-lists': StopOutlined,
  'builtin-features': MenuOutlined,
  commands: BarsOutlined,
  config: SettingOutlined,
  dashboard: DashboardOutlined,
  'history-logs': ControlOutlined,
  login: LoginOutlined,
  logs: FileSearchOutlined,
  'logs-center': FileSearchOutlined,
  'menu-center': MenuOutlined,
  'permission-policy': SafetyCertificateOutlined,
  protocols: DisconnectOutlined,
  'rate-limits': ThunderboltOutlined,
  'render-templates': FileTextOutlined,
  setting: SettingOutlined,
  system: HddOutlined,
  tasks: ProfileOutlined,
  toolbox: ToolOutlined,
}

export function resolveMenuIcon(icon?: string | null) {
  if (!icon) {
    return null
  }

  return iconMap[icon] ?? null
}
