import type { Ref } from 'vue'

import { useDashboardDiagnosticsState } from '@/views/dashboard/useDashboardDiagnosticsState'
import { useDashboardReadinessState } from '@/views/dashboard/useDashboardReadinessState'
import { useDashboardRecoveryState } from '@/views/dashboard/useDashboardRecoveryState'

type DerivedStateInput = {
  diagnostics: Ref<any>
  health: Ref<any>
  readiness: Ref<any>
  selectedRecoveryReviewIds: Ref<string[]>
  system: Ref<any>
}

export function useDashboardDerivedState(input: DerivedStateInput) {
  const diagnosticsState = useDashboardDiagnosticsState({
    diagnostics: input.diagnostics,
  })
  const readinessState = useDashboardReadinessState(input)
  const recoveryState = useDashboardRecoveryState({
    readiness: input.readiness,
    readinessIssues: readinessState.readinessIssues,
    selectedRecoveryReviewIds: input.selectedRecoveryReviewIds,
    system: input.system,
  })

  return {
    ...diagnosticsState,
    ...readinessState,
    ...recoveryState,
  }
}
