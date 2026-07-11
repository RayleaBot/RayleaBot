import { useRouter, type RouteLocationRaw } from 'vue-router'
import { getActivePinia } from 'pinia'

import { defaultLayoutPreferences } from '@/preferences/app'
import { useUiShellStore } from '@/stores/ui-shell'
import { navigateWithMotion } from './runtime'

export function useMotionNavigation() {
  const router = useRouter()
  const pinia = getActivePinia()
  const uiShellStore = pinia ? useUiShellStore(pinia) : null

  return (target: RouteLocationRaw) => navigateWithMotion(
    router,
    target,
    uiShellStore?.preferences.pageTransition ?? defaultLayoutPreferences.pageTransition,
  )
}
