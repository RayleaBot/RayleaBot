// Code generated from contracts/plugin-management-ui-bridge.schema.json; DO NOT EDIT.

export type BridgeSource = "management_host"
 | "plugin_management_ui"

export type BridgeType = "page.ready"
  | "host.connect"
  | "host.init"
  | "settings.reload"
  | "settings.save"
  | "settings.changed"
  | "secrets.status.reload"
  | "secrets.status.changed"
  | "secrets.set"
  | "secrets.delete"
  | "ui.resize"
  | "scheduler.trigger"
  | "scheduler.triggered"
  | "render_template.open"
  | "protocol.targets.reload"
  | "protocol.targets.changed"
  | "protocol.identities.resolve"
  | "protocol.identities.resolved"
  | "plugin.action.invoke"
  | "plugin.action.result"
  | "error"

export interface BridgeMessage<T = unknown> {
  version: "2"
  source: BridgeSource
  type: BridgeType
  nonce?: string
  request_id?: string
  payload?: T
}

export type PluginDescriptor = {
  id: string
  name: string
  version: string
  state: string
}

export type PluginPageDescriptor = {
  id: string
  label: string
}

export type HostInitPayload = {
  plugin: {
      id: string
      name: string
      version: string
      state: string
    }
  page: {
      id: string
      label: string
    }
  config: Record<string, unknown>
  secrets_configured: Record<string, boolean>
  theme: {
      mode: "light" | "dark"
      tokens: Record<string, string>
    }
  language: string
  allowed_capabilities: Array<string>
}

export type SettingsChangedPayload = {
  config: Record<string, unknown>
}

export type SecretsStatusPayload = {
  configured: Record<string, boolean>
}

export type BridgeErrorPayload = {
  code: string
  message: string
  details?: Record<string, unknown>
}
