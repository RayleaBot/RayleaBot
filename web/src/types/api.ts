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

export type OneBot11ProtocolSnapshotResponse = components['schemas']['OneBot11ProtocolSnapshotResponse']
export type OneBot11ProtocolCompatibilityResponse = components['schemas']['OneBot11ProtocolCompatibilityResponse']
export type ThirdPartyAccountSummary = components['schemas']['ThirdPartyAccountSummary']
export type ThirdPartyAccountProfile = components['schemas']['ThirdPartyAccountProfile']
export type ThirdPartyCredentialState = components['schemas']['ThirdPartyCredentialState']
export type ThirdPartyAccountUpsertRequest = components['schemas']['ThirdPartyAccountUpsertRequest']
export type ThirdPartyAccountUpsertResponse = components['schemas']['ThirdPartyAccountUpsertResponse']
export type ThirdPartyAccountsResponse = components['schemas']['ThirdPartyAccountsResponse']
export type ThirdPartyMonitorItem = components['schemas']['ThirdPartyMonitorItem']
export type ThirdPartyMonitorService = components['schemas']['ThirdPartyMonitorService']
export type ThirdPartyMonitorsResponse = components['schemas']['ThirdPartyMonitorsResponse']
export type ThirdPartyPlatform = components['schemas']['ThirdPartyPlatform']
export type BilibiliQRCodeLoginCreateResponse = components['schemas']['BilibiliQRCodeLoginCreateResponse']
export type BilibiliQRCodeLoginPollResponse = components['schemas']['BilibiliQRCodeLoginPollResponse']
export type BilibiliQRCodeLoginState = components['schemas']['BilibiliQRCodeLoginState']
export type BilibiliSourceStatusResponse = components['schemas']['BilibiliSourceStatusResponse']
export type BilibiliSourceRestartResponse = components['schemas']['BilibiliSourceRestartResponse']
