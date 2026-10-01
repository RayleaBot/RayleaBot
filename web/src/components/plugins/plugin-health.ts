import type { Component } from 'vue'
import {
  CircleAlertIcon,
  CircleCheckIcon,
  CircleDashedIcon,
  CircleStopIcon,
  CircleXIcon,
  ClockIcon,
  FileCodeIcon,
  FilesIcon,
  FileXIcon,
  HourglassIcon,
  PowerOffIcon,
  RepeatIcon,
  TimerIcon,
} from '@lucide/vue'

import { t } from '@/i18n'
import { getPluginStateLabel } from '@/lib/display'
import { getErrorCodeMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import { resolveStatusTone, type StatusTone } from '@/lib/status-tone'
import type { PluginDetail } from '@/types/api'

type PluginStateDiagnosis = NonNullable<PluginDetail['state_diagnosis']>

export interface PluginHealthFact {
  key: string
  label: string
  icon: Component
  // values are plain text; code and paths are shown in the mono face below or in place of the text.
  value?: string
  code?: string
  paths?: string[]
}

export interface PluginHealth {
  tone: StatusTone
  icon: Component
  title: string
  description: string
  // A failed or invalid plugin lists what the server recorded and how to recover instead of the routine checks.
  problem: boolean
  canReload: boolean
  facts: PluginHealthFact[]
}

const stateIcons: Record<string, Component> = {
  disabled: PowerOffIcon,
  enabled: ClockIcon,
  starting: HourglassIcon,
  running: CircleCheckIcon,
  stopping: CircleStopIcon,
  failed: CircleXIcon,
  invalid: FileXIcon,
}

export function describePluginHealth(plugin: Pick<PluginDetail, 'state' | 'state_diagnosis'> | null): PluginHealth {
  const state = plugin?.state
  const diagnosis = plugin?.state_diagnosis
  const problem = state === 'failed' || state === 'invalid'
  const stateLabel = getPluginStateLabel(state)
  const diagnosisKey = diagnosis ? `plugins.overview.health.diagnoses.${diagnosis.kind}` : null
  const diagnosisLabel = problem && diagnosisKey ? t(`${diagnosisKey}.label`) : null

  return {
    tone: resolveStatusTone(state),
    icon: (state && stateIcons[state]) || CircleDashedIcon,
    // A failed runtime pairs its state with the cause; for an invalid plugin both would name the manifest, so the
    // diagnosis stands alone.
    title: diagnosisLabel ? (state === 'invalid' ? diagnosisLabel : `${stateLabel} · ${diagnosisLabel}`) : stateLabel,
    description: problem && diagnosisKey
      ? t(`${diagnosisKey}.description`)
      : state ? t(`plugins.overview.health.states.${state}`) : '',
    problem,
    // An invalid manifest is fixed in the package; reloading only helps a runtime that failed.
    canReload: state === 'failed',
    facts: problem && diagnosis ? describeDiagnosisFacts(diagnosis) : [],
  }
}

// Only the fields the server recorded are listed. The reason comes from the stable error code, never from
// last_error_message, so the wording matches the rest of the management surface.
function describeDiagnosisFacts(diagnosis: PluginStateDiagnosis): PluginHealthFact[] {
  const facts: PluginHealthFact[] = []
  if (diagnosis.last_error_code) {
    facts.push({ key: 'reason', label: t('plugins.overview.health.reason'), icon: CircleAlertIcon, value: getErrorCodeMessage(diagnosis.last_error_code), code: diagnosis.last_error_code })
  }
  if (diagnosis.crash_count) {
    facts.push({ key: 'crash-count', label: t('plugins.overview.health.crashCount'), icon: RepeatIcon, value: t('plugins.overview.health.crashCountValue', { count: diagnosis.crash_count }) })
  }
  if (diagnosis.entered_at) {
    facts.push({ key: 'entered-at', label: t('plugins.overview.health.enteredAt'), icon: ClockIcon, value: formatDateTime(diagnosis.entered_at) })
  }
  if (diagnosis.retry_at) {
    facts.push({ key: 'retry-at', label: t('plugins.overview.health.retryAt'), icon: TimerIcon, value: formatDateTime(diagnosis.retry_at) })
  }
  if (diagnosis.manifest_path) {
    facts.push({ key: 'manifest-path', label: t('plugins.overview.health.manifestPath'), icon: FileCodeIcon, paths: [diagnosis.manifest_path] })
  }
  if (diagnosis.manifest_paths?.length) {
    facts.push({ key: 'manifest-paths', label: t('plugins.overview.health.manifestPaths'), icon: FilesIcon, paths: diagnosis.manifest_paths })
  }
  return facts
}
