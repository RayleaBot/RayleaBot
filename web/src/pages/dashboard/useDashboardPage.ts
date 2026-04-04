import { AUTO_REFRESH_INTERVAL } from '@/pages/dashboard/constants'
import { useDashboardState } from '@/pages/dashboard/useDashboardState'
import { useDashboardActions } from '@/pages/dashboard/useDashboardActions'

export function useDashboardPage() {
  const state = useDashboardState()
  const actions = useDashboardActions(state)

  return {
    AUTO_REFRESH_INTERVAL,
    ...state,
    ...actions,
  }
}
