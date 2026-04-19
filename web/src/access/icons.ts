import type { Component } from 'vue'
import {
  AppstoreOutlined,
  BarsOutlined,
  ControlOutlined,
  FileTextOutlined,
  DashboardOutlined,
  DisconnectOutlined,
  FileSearchOutlined,
  HddOutlined,
  LoginOutlined,
  ProfileOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  ToolOutlined,
} from '@ant-design/icons-vue'

const iconMap: Record<string, Component> = {
  appstore: AppstoreOutlined,
  commands: BarsOutlined,
  config: SettingOutlined,
  dashboard: DashboardOutlined,
  governance: SafetyCertificateOutlined,
  'history-logs': ControlOutlined,
  login: LoginOutlined,
  logs: FileSearchOutlined,
  'logs-center': FileSearchOutlined,
  protocols: DisconnectOutlined,
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
