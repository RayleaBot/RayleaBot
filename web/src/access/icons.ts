import type { Component } from 'vue'
import {
  ApiOutlined,
  AppstoreOutlined,
  ApartmentOutlined,
  BlockOutlined,
  CodeOutlined,
  DashboardOutlined,
  FieldTimeOutlined,
  FileSearchOutlined,
  FileTextOutlined,
  HddOutlined,
  HistoryOutlined,
  IdcardOutlined,
  LoginOutlined,
  MenuOutlined,
  MonitorOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  ShopOutlined,
  SlidersOutlined,
  StopOutlined,
  TableOutlined,
  ThunderboltOutlined,
  ToolOutlined,
} from '@ant-design/icons-vue'

const iconMap: Record<string, Component> = {
  'access-lists': StopOutlined,
  commands: CodeOutlined,
  config: SlidersOutlined,
  connections: ApartmentOutlined,
  dashboard: DashboardOutlined,
  features: BlockOutlined,
  'history-logs': HistoryOutlined,
  login: LoginOutlined,
  logs: FileSearchOutlined,
  'menu-center': MenuOutlined,
  'permission-policy': SafetyCertificateOutlined,
  'plugin-settings': SettingOutlined,
  'plugin-store': ShopOutlined,
  plugins: AppstoreOutlined,
  'protocol-compatibility': TableOutlined,
  protocols: ApiOutlined,
  'rate-limits': ThunderboltOutlined,
  'render-templates': FileTextOutlined,
  runtime: MonitorOutlined,
  scheduler: FieldTimeOutlined,
  system: HddOutlined,
  'third-party-accounts': IdcardOutlined,
  toolbox: ToolOutlined,
}

export function resolveMenuIcon(icon?: string | null) {
  if (!icon) {
    return null
  }

  return iconMap[icon] ?? null
}
