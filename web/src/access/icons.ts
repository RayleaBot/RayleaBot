import type { Component } from 'vue'
import {
  AppstoreOutlined,
  BarsOutlined,
  DashboardOutlined,
  DisconnectOutlined,
  FileSearchOutlined,
  HddOutlined,
  LoginOutlined,
  SettingOutlined,
  ToolOutlined,
} from '@ant-design/icons-vue'

const iconMap: Record<string, Component> = {
  appstore: AppstoreOutlined,
  commands: BarsOutlined,
  dashboard: DashboardOutlined,
  login: LoginOutlined,
  logs: FileSearchOutlined,
  protocols: DisconnectOutlined,
  setting: SettingOutlined,
  system: HddOutlined,
  toolbox: ToolOutlined,
}

export function resolveMenuIcon(icon?: string | null) {
  if (!icon) {
    return null
  }

  return iconMap[icon] ?? null
}
