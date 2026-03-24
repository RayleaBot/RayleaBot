import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { toBootstrapStatusMessage } from '@/lib/auth-feedback'
import { apiRequest } from '@/lib/http'
import type { LauncherAdmissionRequest, SessionLoginRequest, SessionLoginResponse, SetupStatusResponse } from '@/types/api'

const sessionStorageKey = 'rayleabot.session_token'

function readStoredToken() {
  return window.sessionStorage.getItem(sessionStorageKey)
}

function writeStoredToken(token: string | null) {
  if (token) {
    window.sessionStorage.setItem(sessionStorageKey, token)
    return
  }

  window.sessionStorage.removeItem(sessionStorageKey)
}

export const useSessionStore = defineStore('session', () => {
  const token = ref<string | null>(typeof window === 'undefined' ? null : readStoredToken())
  const setupInitialized = ref<boolean | null>(null)
  const bootstrapPending = ref(false)
  const loginPending = ref(false)
  const bootstrapError = ref<string | null>(null)
  const launcherAdmissionHint = ref<string | null>(null)

  const isAuthenticated = computed(() => Boolean(token.value))
  const requiresSetup = computed(() => setupInitialized.value === false)
  const isBootstrapped = computed(() => setupInitialized.value !== null)

  async function bootstrap(force = false) {
    if (bootstrapPending.value) {
      return
    }

    if (isBootstrapped.value && !force) {
      return
    }

    bootstrapPending.value = true
    bootstrapError.value = null
    try {
      const response = await apiRequest<SetupStatusResponse>('/api/setup/status', { auth: false })
      setupInitialized.value = response.initialized
    } catch (error) {
      bootstrapError.value = toBootstrapStatusMessage(error)
      throw error
    } finally {
      bootstrapPending.value = false
    }
  }

  function setToken(nextToken: string | null) {
    token.value = nextToken
    writeStoredToken(nextToken)
  }

  function matchesCurrentToken(tokenSnapshot?: string | null) {
    return tokenSnapshot === undefined || tokenSnapshot === token.value
  }

  async function login(payload: SessionLoginRequest) {
    loginPending.value = true
    try {
      const response = await apiRequest<SessionLoginResponse>('/api/session/login', {
        method: 'POST',
        auth: false,
        body: payload,
      })
      setupInitialized.value = true
      launcherAdmissionHint.value = null
      setToken(response.session_token)
      return response
    } finally {
      loginPending.value = false
    }
  }

  async function setupAdmin(payload: SessionLoginRequest) {
    loginPending.value = true
    try {
      const response = await apiRequest<SessionLoginResponse>('/api/setup/admin', {
        method: 'POST',
        auth: false,
        body: payload,
      })
      setupInitialized.value = true
      launcherAdmissionHint.value = null
      setToken(response.session_token)
      return response
    } finally {
      loginPending.value = false
    }
  }

  async function admitLauncherToken(launcherToken: string) {
    loginPending.value = true
    try {
      const response = await apiRequest<SessionLoginResponse>('/api/session/launcher-admission', {
        method: 'POST',
        auth: false,
        body: { launcher_token: launcherToken } satisfies LauncherAdmissionRequest,
      })
      setupInitialized.value = true
      launcherAdmissionHint.value = null
      setToken(response.session_token)
      return response
    } finally {
      loginPending.value = false
    }
  }

  async function logout() {
    if (token.value) {
      try {
        await apiRequest('/api/session', { method: 'DELETE' })
      } catch {
        // local logout still wins
      }
    }
    clearSession()
  }

  function clearSession(tokenSnapshot?: string | null) {
    if (!matchesCurrentToken(tokenSnapshot)) {
      return false
    }

    setToken(null)
    return true
  }

  function setLauncherAdmissionHint(message: string | null) {
    launcherAdmissionHint.value = message
  }

  function handleSessionExpired(tokenSnapshot?: string | null) {
    clearSession(tokenSnapshot)
  }

  return {
    bootstrapError,
    bootstrapPending,
    isAuthenticated,
    isBootstrapped,
    launcherAdmissionHint,
    loginPending,
    requiresSetup,
    setupInitialized,
    token,
    bootstrap,
    clearSession,
    handleSessionExpired,
    login,
    logout,
    setLauncherAdmissionHint,
    setToken,
    setupAdmin,
    admitLauncherToken,
  }
})
