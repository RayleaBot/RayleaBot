export type PluginRegistrationState = 'installed' | 'removed'
export type PluginDesiredState = 'enabled' | 'disabled'
export type PluginRuntimeState = 'starting' | 'running' | 'stopping' | 'crashed' | 'backoff' | 'dead_letter' | 'stopped'
export type PluginRole = 'builtin' | 'user' | 'example' | 'dev'
export type PluginTrustLevel = 'official' | 'third_party' | 'unverified' | 'development'

export type PluginInstallSourceType = 'local_zip' | 'local_directory' | 'remote_url'

export interface PluginSourceSummary {
  root: string
  package_source_type?: PluginInstallSourceType
  package_source_ref?: string
  verified: boolean
}

export interface PluginTrustSummary {
  level: PluginTrustLevel
  label: string
}

export interface PluginSummary {
  id: string
  name: string
  role: PluginRole
  registration_state: PluginRegistrationState
  desired_state: PluginDesiredState
  runtime_state: PluginRuntimeState
  display_state?: string
  source?: PluginSourceSummary
  trust?: PluginTrustSummary
  command_conflicts?: string[]
}

export interface PluginListResponse {
  items: PluginSummary[]
}

export interface PluginDetailResponse {
  plugin: PluginSummary
}

export interface PluginInstallRequest {
  source_type: PluginInstallSourceType
  source: string
  allow_install_scripts?: boolean
}

export interface PluginGrantRequest {
  capability: string
  expires_at?: string
}

export interface PluginGrantSummary {
  plugin_id: string
  capability: string
  granted_at: string
  expires_at?: string | null
}

export interface PluginGrantListResponse {
  items: PluginGrantSummary[]
}
