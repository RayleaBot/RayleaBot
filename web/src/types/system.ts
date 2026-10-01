import type { components } from './generated'

export type SetupStatusResponse = components['schemas']['SetupStatusResponse']
export type SessionLoginRequest = components['schemas']['SessionLoginRequest']
export type SessionLoginResponse = components['schemas']['SessionLoginResponse']
export type LivenessStatusResponse = components['schemas']['LivenessStatusResponse']
export type ReadinessStatusResponse = components['schemas']['ReadinessStatusResponse']
export type SystemDiagnosticsResponse = components['schemas']['SystemDiagnosticsResponse']
export type SystemStatusResponse = components['schemas']['SystemStatusResponse']
export type MessageStatsResponse = components['schemas']['MessageStatsResponse']
export type MessageStatsConnection = components['schemas']['MessageStatsConnection']
export type MessageStatsCounts = components['schemas']['MessageStatsCounts']
export type MessageStatsIncident = components['schemas']['MessageStatsIncident']
export type MessageStatsGranularity = components['schemas']['MessageStatsGranularity']
export type SystemShutdownResponse = components['schemas']['SystemShutdownResponse']
export type UpdateStatusResponse = components['schemas']['UpdateStatusResponse']
export type RuntimeBootstrapResource = components['schemas']['RuntimeBootstrapResource']
export type RenderTemplateDetail = components['schemas']['RenderTemplateDetail']
export type RenderTemplateDetailResponse = components['schemas']['RenderTemplateDetailResponse']
export type RenderTemplateListResponse = components['schemas']['RenderTemplateListResponse']
export type RenderTemplatePreviewHTMLRequest = components['schemas']['RenderTemplatePreviewHTMLRequest']
export type RenderTemplatePreviewHTMLResponse = components['schemas']['RenderTemplatePreviewHTMLResponse']
export type RenderTemplateSummary = components['schemas']['RenderTemplateSummary']

export interface RenderTemplateLocalIssue {
  field: 'preview_data'
  message: string
}

export interface RenderTemplateSchemaNode {
  key: string
  path: string
  label: string
  type: string
  required: boolean
  description: string
  depth: number
}
