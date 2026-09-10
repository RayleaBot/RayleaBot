import { onActivated, onDeactivated, onMounted, onScopeDispose, reactive } from 'vue'
import { notifyError } from '@/adapter/feedback'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { useThirdPartyAccountsStore, type ThirdPartyPlatform } from '@/stores/third-party-accounts'
import type { ThirdPartyAccountSummary, ThirdPartyQRCodeLoginCreateResponse, ThirdPartyQRCodeLoginPollResponse, ThirdPartyQRCodeLoginState } from '@/types/api'

export interface QRLoginState {
  platform: ThirdPartyPlatform
  loginId: string
  qrcodeUrl: string
  expiresAt: string
  state: ThirdPartyQRCodeLoginState
  accountNickname: string
  accountUid: string
  accountAvatarUrl: string
  pollErrorNotified: boolean
}

// This owner keeps remote sessions, pending creates and polling in the same
// lifecycle. A create response arriving after cancellation is retired remotely.
export function useAccountQRLogins(onAccount: (key: string, account: ThirdPartyAccountSummary) => string) {
  const store = useThirdPartyAccountsStore()
  const qrLogins = reactive<Record<string, QRLoginState>>({})
  const versions = new Map<string, number>()
  const inFlight = new Map<string, string>()
  let sequence = 0
  let active = true
  let timer: ReturnType<typeof setInterval> | undefined

  function isPending(qr: QRLoginState) {
    return ['pending_scan', 'pending_confirm', 'verification_required'].includes(qr.state)
  }

  function stopPolling() {
    clearInterval(timer)
    timer = undefined
  }

  function schedulePolling() {
    if (active && timer === undefined) timer = setInterval(() => { void poll() }, 2000)
  }

  function accept(key: string, response: ThirdPartyQRCodeLoginCreateResponse | ThirdPartyQRCodeLoginPollResponse) {
    const previous = qrLogins[key]
    const account = 'account' in response ? response.account : null
    qrLogins[key] = {
      platform: response.platform, loginId: response.login_id, expiresAt: response.expires_at, state: response.state,
      qrcodeUrl: 'qrcode_url' in response ? response.qrcode_url : previous?.qrcodeUrl || '',
      accountNickname: account?.profile?.nickname || account?.label || previous?.accountNickname || '',
      accountUid: account?.profile?.uid || account?.account_id || previous?.accountUid || '',
      accountAvatarUrl: account?.profile?.avatar_url || previous?.accountAvatarUrl || '',
      pollErrorNotified: false,
    }
    if (response.state === 'succeeded' && account) {
      const nextKey = onAccount(key, account)
      if (nextKey !== key) {
        qrLogins[nextKey] = qrLogins[key]
        delete qrLogins[key]
      }
    }
  }

  async function start(key: string, platform: ThirdPartyPlatform) {
    const version = await cancel(key)
    if (!active || versions.get(key) !== version) return
    try {
      const response = await store.createQRCodeLogin(platform)
      if (!active || versions.get(key) !== version) {
        await store.cancelQRCodeLogin(response.platform, response.login_id, true)
        return
      }
      accept(key, response)
      schedulePolling()
    } catch (error) {
      if (active && versions.get(key) === version) notifyError(getDisplayErrorMessage(error))
    }
  }

  async function cancel(key: string, keepalive = false, strict = false) {
    const version = ++sequence
    versions.set(key, version)
    const qr = qrLogins[key]
    delete qrLogins[key]
    inFlight.delete(key)
    if (!qr) return version
    try {
      await store.cancelQRCodeLogin(qr.platform, qr.loginId, keepalive)
    } catch (error) {
      if (strict) {
        if (active && versions.get(key) === version) {
          qrLogins[key] = qr
          schedulePolling()
        }
        throw error
      }
    }
    if (!Object.values(qrLogins).some(isPending)) stopPolling()
    return version
  }

  async function cancelAll(keepalive: boolean) {
    versions.clear()
    inFlight.clear()
    stopPolling()
    const sessions = Object.values(qrLogins)
    for (const key of Object.keys(qrLogins)) delete qrLogins[key]
    await Promise.allSettled(sessions.map(qr => store.cancelQRCodeLogin(qr.platform, qr.loginId, keepalive)))
  }

  async function poll() {
    if (!active) return
    const sessions = Object.entries(qrLogins).filter(([key, qr]) => isPending(qr) && inFlight.get(key) !== qr.loginId)
    await Promise.all(sessions.map(async ([key, qr]) => {
      inFlight.set(key, qr.loginId)
      try {
        const response = await store.pollQRCodeLogin(qr.platform, qr.loginId)
        if (active && qrLogins[key]?.loginId === qr.loginId) accept(key, response)
      } catch (error) {
        if (!active || qrLogins[key]?.loginId !== qr.loginId) return
        if (Date.parse(qr.expiresAt) <= Date.now()) qr.state = 'expired'
        else if (!qr.pollErrorNotified) {
          qr.pollErrorNotified = true
          notifyError(getDisplayErrorMessage(error))
        }
      } finally {
        if (inFlight.get(key) === qr.loginId) inFlight.delete(key)
      }
    }))
    if (!Object.values(qrLogins).some(isPending)) stopPolling()
  }

  function deactivate(keepalive: boolean) {
    active = false
    void cancelAll(keepalive)
  }
  const pageHide = () => deactivate(true)
  const pageShow = () => { active = true }
  onMounted(() => {
    window.addEventListener('pagehide', pageHide)
    window.addEventListener('pageshow', pageShow)
  })
  onActivated(pageShow)
  onDeactivated(() => deactivate(false))
  onScopeDispose(() => {
    window.removeEventListener('pagehide', pageHide)
    window.removeEventListener('pageshow', pageShow)
    deactivate(true)
  })
  return { qrLogins, start, cancel }
}
