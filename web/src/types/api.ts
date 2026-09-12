export * from './common'
export * from './tasks'
export * from './plugins'
export * from './system'
export * from './logs'
export * from './scheduler'
export * from './config'
export * from './events'
export * from './governance'
export type { PluginConsoleFrameData } from './websocket.generated'
export type { components } from './generated'

import type { components } from './generated'

export type AccountCredentialsUpdateRequest = components['schemas']['AccountCredentialsUpdateRequest']

export type AdaptersResponse = components['schemas']['AdaptersResponse']
export type AdapterDescriptor = components['schemas']['AdapterDescriptor']
export type AdapterProtocolDescriptor = components['schemas']['AdapterProtocolDescriptor']
export type AdapterProtocol = components['schemas']['AdapterProtocol']
export type AdapterState = components['schemas']['AdapterState']
export type OneBot11ProtocolCompatibilityResponse = components['schemas']['OneBot11ProtocolCompatibilityResponse']
