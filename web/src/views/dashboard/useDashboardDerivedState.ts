import type { Ref } from 'vue'

import { useDashboardAlertState } from '@/views/dashboard/useDashboardAlertState'
import { useDashboardDiagnosticsState } from '@/views/dashboard/useDashboardDiagnosticsState'
import { useDashboardReadinessState } from '@/views/dashboard/useDashboardReadinessState'

type DerivedStateInput = {
  diagnostics: Ref<any>
  health: Ref<any>
  readiness: Ref<any>
  system: Ref<any>
}

export function useDashboardDerivedState(input: DerivedStateInput) {
  const diagnosticsState = useDashboardDiagnosticsState({
    diagnostics: input.diagnostics,
  })
  const readinessState = useDashboardReadinessState(input)
  const alertState = useDashboardAlertState({
    readiness: input.readiness,
    readinessIssues: readinessState.readinessIssues,
  })

  return {
    ...diagnosticsState,
    ...readinessState,
    ...alertState,
  }
}
