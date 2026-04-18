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
export type RenderTemplateDraft = components['schemas']['RenderTemplateDraft']
export type RenderTemplateListResponse = components['schemas']['RenderTemplateListResponse']
export type RenderTemplateRollbackRequest = components['schemas']['RenderTemplateRollbackRequest']
export type RenderTemplateSource = components['schemas']['RenderTemplateSource']
export type RenderTemplateSourceResponse = components['schemas']['RenderTemplateSourceResponse']
export type RenderTemplateSourceUpdateRequest = components['schemas']['RenderTemplateSourceUpdateRequest']
export type RenderTemplateSummary = components['schemas']['RenderTemplateSummary']
export type RenderTemplateValidateRequest = components['schemas']['RenderTemplateValidateRequest']
export type RenderTemplateValidateResponse = components['schemas']['RenderTemplateValidateResponse']
export type RenderTemplateValidationIssue = components['schemas']['RenderTemplateValidationIssue']
export type RenderTemplateValidationStatus = components['schemas']['RenderTemplateValidationStatus']
export type RenderTemplateVersion = components['schemas']['RenderTemplateVersion']
export type RenderTemplateVersionListResponse = components['schemas']['RenderTemplateVersionListResponse']

export type RenderTemplateTextFieldKey = 'manifest_json' | 'html' | 'stylesheet' | 'input_schema_json'

export interface RenderTemplateTextDraft {
  manifest_json: string
  html: string
  stylesheet: string
  input_schema_json: string
}

export interface RenderTemplateLocalIssue {
  field: RenderTemplateTextFieldKey | 'preview_data'
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
