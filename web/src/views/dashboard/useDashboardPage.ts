import { useDashboardState } from '@/views/dashboard/useDashboardState'
import { useDashboardActions } from '@/views/dashboard/useDashboardActions'

export function useDashboardPage() {
  const state = useDashboardState()
  const actions = useDashboardActions(state)

  return {
    ...state,
    ...actions,
  }
}
