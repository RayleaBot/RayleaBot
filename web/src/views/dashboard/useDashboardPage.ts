import { AUTO_REFRESH_INTERVAL } from '@/views/dashboard/constants'
import { useDashboardState } from '@/views/dashboard/useDashboardState'
import { useDashboardActions } from '@/views/dashboard/useDashboardActions'

export function useDashboardPage() {
  const state = useDashboardState()
  const actions = useDashboardActions(state)

  return {
    AUTO_REFRESH_INTERVAL,
    ...state,
    ...actions,
  }
}
