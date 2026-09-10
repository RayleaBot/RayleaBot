import { onScopeDispose, reactive, watch, type Ref } from 'vue'
import type { ThirdPartyAccountSummary } from '@/types/api'

export function useAccountAvatars(accounts: Ref<ThirdPartyAccountSummary[]>) {
  const avatarRetryCooldownMs = 30_000

  const avatarLoadFailures = reactive<Record<string, number>>({})

  const avatarRetryTimers = new Map<string, number>()

  function accountAvatarSrc(account: ThirdPartyAccountSummary) {
    const avatarURL = account.profile?.avatar_url?.trim()
    if (!avatarURL || avatarLoadFailures[avatarFailureKey(account)] !== undefined) {
      return ''
    }
    return `/api/third-party/accounts/${encodeURIComponent(account.platform)}/${encodeURIComponent(account.account_id)}/avatar`
  }

  function markAvatarFailed(account: ThirdPartyAccountSummary) {
    const key = avatarFailureKey(account)
    avatarLoadFailures[key] = Date.now()
    const existingTimer = avatarRetryTimers.get(key)
    if (existingTimer !== undefined) {
      window.clearTimeout(existingTimer)
    }
    avatarRetryTimers.set(key, window.setTimeout(() => {
      avatarRetryTimers.delete(key)
      delete avatarLoadFailures[key]
    }, avatarRetryCooldownMs))
  }

  function markAvatarLoaded(account: ThirdPartyAccountSummary) {
    clearAvatarFailure(avatarFailureKey(account))
  }

  function clearAvatarFailure(key: string) {
    const timer = avatarRetryTimers.get(key)
    if (timer !== undefined) {
      window.clearTimeout(timer)
      avatarRetryTimers.delete(key)
    }
    delete avatarLoadFailures[key]
  }

  function clearAvatarFailures() {
    for (const timer of avatarRetryTimers.values()) {
      window.clearTimeout(timer)
    }
    avatarRetryTimers.clear()
    for (const key of Object.keys(avatarLoadFailures)) {
      delete avatarLoadFailures[key]
    }
  }

  function avatarFailureKey(account: ThirdPartyAccountSummary) {
    return `${account.platform}:${account.account_id}:${account.profile?.avatar_url || ''}`
  }
  watch(accounts, clearAvatarFailures, { flush: 'sync' })
  onScopeDispose(clearAvatarFailures)
  return { accountAvatarSrc, markAvatarFailed, markAvatarLoaded, avatarFailureKey }
}
