import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { toBootstrapStatusMessage } from '@/lib/auth-feedback'
import { ApiError, apiRequest } from '@/lib/http'
import type { SessionLoginRequest, SessionLoginResponse, SetupStatusResponse } from '@/types/api'

const legacySessionStorageKey = 'rayleabot.session_token'

function clearLegacyBearerToken() {
  if (typeof window !== 'undefined') {
    window.localStorage.removeItem(legacySessionStorageKey)
  }
}

function consumeSetupTokenFragment() {
  if (typeof window === 'undefined' || !window.location.hash) {
    return null
  }

  const parameters = new URLSearchParams(window.location.hash.slice(1))
  const token = parameters.get('setup_token')?.trim() ?? ''
  if (!token) {
    return null
  }

  parameters.delete('setup_token')
  const remainingFragment = parameters.toString()
  window.history.replaceState(
    window.history.state,
    '',
    `${window.location.pathname}${window.location.search}${remainingFragment ? `#${remainingFragment}` : ''}`,
  )
  return token
}

type BrowserSessionResponse = SessionLoginResponse & {
  transport: 'cookie' | 'bearer'
  csrf_token?: string
}

export const useSessionStore = defineStore('session', () => {
  clearLegacyBearerToken()

  const authenticated = ref(false)
  const csrfToken = ref<string | null>(null)
  const setupToken = ref<string | null>(consumeSetupTokenFragment())
  const setupInitialized = ref<boolean | null>(null)
  const bootstrapPending = ref(false)
  const loginPending = ref(false)
  const bootstrapError = ref<string | null>(null)

  const isAuthenticated = computed(() => authenticated.value)
  const requiresSetup = computed(() => setupInitialized.value === false)
  const isBootstrapped = computed(() => setupInitialized.value !== null)

  async function restoreCookieSession() {
    try {
      await apiRequest('/api/system/status')
      authenticated.value = true
    } catch (error) {
      authenticated.value = false
      csrfToken.value = null
      if (!(error instanceof ApiError) || error.status !== 401) {
        throw error
      }
    }
  }

  async function bootstrap(force = false) {
    if (bootstrapPending.value || (isBootstrapped.value && !force)) {
      return
    }

    bootstrapPending.value = true
    bootstrapError.value = null
    try {
      const response = await apiRequest<SetupStatusResponse>('/api/setup/status', { auth: false })
      setupInitialized.value = response.initialized
      if (response.initialized) {
        await restoreCookieSession()
      } else {
        clearSession()
      }
    } catch (error) {
      bootstrapError.value = toBootstrapStatusMessage(error)
      throw error
    } finally {
      bootstrapPending.value = false
    }
  }

  function acceptBrowserSession(response: BrowserSessionResponse) {
    if (response.transport !== 'cookie' || !response.csrf_token?.trim()) {
      throw new Error('服务端未建立浏览器会话。')
    }
    csrfToken.value = response.csrf_token
    authenticated.value = true
    setupInitialized.value = true
  }

  async function login(payload: SessionLoginRequest) {
    loginPending.value = true
    try {
      const response = await apiRequest<BrowserSessionResponse>('/api/session/login', {
        method: 'POST',
        auth: false,
        headers: { 'X-Raylea-Session-Transport': 'cookie' },
        body: payload,
      })
      acceptBrowserSession(response)
      return response
    } finally {
      loginPending.value = false
    }
  }

  async function setupAdmin(payload: SessionLoginRequest) {
    loginPending.value = true
    try {
      const response = await apiRequest<BrowserSessionResponse>('/api/setup/admin', {
        method: 'POST',
        auth: false,
        headers: {
          'X-Raylea-Session-Transport': 'cookie',
          ...(setupToken.value ? { 'X-Raylea-Setup-Token': setupToken.value } : {}),
        },
        body: payload,
      })
      acceptBrowserSession(response)
      setupToken.value = null
      return response
    } finally {
      loginPending.value = false
    }
  }

  async function logout() {
    if (authenticated.value) {
      try {
        await apiRequest<void>('/api/session', { method: 'DELETE' })
      } catch {
        // Clearing browser state remains safe when the server is unavailable.
      }
    }
    clearSession()
  }

  function clearSession() {
    authenticated.value = false
    csrfToken.value = null
    clearLegacyBearerToken()
    return true
  }

  function handleSessionExpired() {
    clearSession()
  }

  return {
    bootstrapError,
    bootstrapPending,
    csrfToken,
    isAuthenticated,
    isBootstrapped,
    loginPending,
    requiresSetup,
    setupInitialized,
    bootstrap,
    clearSession,
    handleSessionExpired,
    login,
    logout,
    setupAdmin,
  }
})
