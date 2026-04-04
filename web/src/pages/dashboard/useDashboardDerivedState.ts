import type { Ref } from 'vue'

import { useDashboardReadinessState } from '@/pages/dashboard/useDashboardReadinessState'
import { useDashboardRecoveryState } from '@/pages/dashboard/useDashboardRecoveryState'

type DerivedStateInput = {
  health: Ref<any>
  readiness: Ref<any>
  selectedRecoveryReviewIds: Ref<string[]>
  system: Ref<any>
}

export function useDashboardDerivedState(input: DerivedStateInput) {
  const readinessState = useDashboardReadinessState(input)
  const recoveryState = useDashboardRecoveryState({
    readiness: input.readiness,
    readinessIssues: readinessState.readinessIssues,
    selectedRecoveryReviewIds: input.selectedRecoveryReviewIds,
    system: input.system,
  })

  return {
    ...readinessState,
    ...recoveryState,
  }
}
