import type {
  PluginDesiredState,
  PluginRegistrationState,
  PluginRuntimeState,
} from './plugins'
import type { ConnectionStatus } from './common'

export type EventsPayload =
  | {
      service_status: 'setup_required' | 'stopped' | 'starting' | 'running' | 'degraded' | 'stopping' | 'failed'
      summary: string
      reason?: string
      reason_codes?: string[]
    }
  | {
      plugin_id: string
      registration_state: PluginRegistrationState
      desired_state: PluginDesiredState
      runtime_state: PluginRuntimeState
      display_state?: string
    }
  | {
      connection_status: ConnectionStatus
      summary: string
    }
  | {
      event_type: string
      summary: string
    }
  | {
      observability_scope: 'bridge_runtime'
      summary: string
      last_supported_event_kind?: string
      last_delivery_outcome?: 'delivered' | 'error'
      delivered_count: number
      result_count: number
      error_count: number
    }
