import type { components } from './generated'

export type SetupStatusResponse = components['schemas']['SetupStatusResponse']
export type SessionLoginRequest = components['schemas']['SessionLoginRequest']
export type SessionLoginResponse = components['schemas']['SessionLoginResponse']
export type LauncherTokenResponse = components['schemas']['LauncherTokenResponse']
export type LauncherAdmissionRequest = components['schemas']['LauncherAdmissionRequest']
export type LivenessStatusResponse = components['schemas']['LivenessStatusResponse']
export type ReadinessIssue = components['schemas']['DiagnosticIssue']
export type RecoveryCompatibilityIssue = components['schemas']['RecoveryCompatibilityIssue']
export type RecoveryCompatibilitySkippedPlugin = components['schemas']['RecoveryCompatibilitySkippedPlugin']
export type RecoveryCompatibilityAuditItem = components['schemas']['RecoveryCompatibilityAuditItem']
export type RecoveryCompatibilityAuditEntry = components['schemas']['RecoveryCompatibilityAuditEntry']
export type RecoveryCompatibilitySummary = components['schemas']['RecoveryCompatibilitySummary']
export type ReadinessStatusResponse = components['schemas']['ReadinessStatusResponse']
export type SystemStatusResponse = components['schemas']['SystemStatusResponse']
export type SystemShutdownResponse = components['schemas']['SystemShutdownResponse']
export type RecoveryConfirmRequest = components['schemas']['RecoveryConfirmRequest']
export type RuntimeBootstrapResource = components['schemas']['RuntimeBootstrapResource']
export type RuntimeBootstrapRequest = components['schemas']['RuntimeBootstrapRequest']
export type RenderPreviewRequest = components['schemas']['RenderPreviewRequest']
export type RenderTemplateDetail = components['schemas']['RenderTemplateDetail']
export type RenderTemplateDetailResponse = components['schemas']['RenderTemplateDetailResponse']
export type RenderTemplateListResponse = components['schemas']['RenderTemplateListResponse']
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
