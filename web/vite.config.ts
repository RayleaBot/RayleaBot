import { fileURLToPath, URL } from 'node:url'
import net from 'node:net'
import type { IncomingMessage, ServerResponse } from 'node:http'

import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig, type Plugin, type ProxyOptions } from 'vite'

const backendTarget = process.env.VITE_BACKEND_TARGET ?? 'http://127.0.0.1:8080'
process.env.VITE_BACKEND_TARGET = backendTarget
const backendUnavailableHeader = 'x-rayleabot-backend-unavailable'
const backendAvailabilityCacheMs = 500

export function resolveDevWebSocketBaseUrl(configuredBaseUrl: string | undefined, fallbackBaseUrl: string) {
  return configuredBaseUrl?.trim() || fallbackBaseUrl
}

export function resolveClientWebSocketBaseUrl(command: string, configuredBaseUrl: string | undefined, fallbackBaseUrl: string) {
  return command === 'serve' ? resolveDevWebSocketBaseUrl(configuredBaseUrl, fallbackBaseUrl) : ''
}

function isBackendProxyPath(requestUrl: string | undefined) {
  const pathname = new URL(requestUrl ?? '/', 'http://rayleabot.local').pathname
  return /^\/(?:api(?:\/|$)|healthz$|readyz$|plugin-ui(?:\/|$))/.test(pathname)
}

function createBackendAvailabilityChecker(target: string) {
  const targetUrl = new URL(target)
  const host = targetUrl.hostname
  const port = Number(targetUrl.port || (targetUrl.protocol === 'https:' ? 443 : 80))
  let checkedAt = 0
  let available = true

  async function checkBackendAvailable() {
    const now = Date.now()
    if (now - checkedAt < backendAvailabilityCacheMs) {
      return available
    }

    available = await new Promise<boolean>((resolve) => {
      const socket = net.createConnection({ host, port })
      let settled = false
      const finish = (nextAvailable: boolean) => {
        if (settled) {
          return
        }
        settled = true
        socket.destroy()
        resolve(nextAvailable)
      }
      socket.setTimeout(250)
      socket.once('connect', () => finish(true))
      socket.once('timeout', () => finish(false))
      socket.once('error', () => finish(false))
    })
    checkedAt = now
    return available
  }

  return checkBackendAvailable
}

function writeBackendUnavailableResponse(response: ServerResponse) {
  if (response.headersSent) {
    response.end()
    return
  }

  response.writeHead(503, {
    'Content-Type': 'application/json; charset=utf-8',
    [backendUnavailableHeader]: '1',
  })
  response.end(JSON.stringify({
    error: {
      message: '管理服务暂不可用。',
    },
  }))
}

function createBackendAvailabilityGuard(target: string): Plugin {
  const checkBackendAvailable = createBackendAvailabilityChecker(target)

  return {
    name: 'rayleabot-backend-availability-guard',
    configureServer(server) {
      server.middlewares.use((request: IncomingMessage, response: ServerResponse, next) => {
        if (!isBackendProxyPath(request.url)) {
          next()
          return
        }

        void checkBackendAvailable().then((available) => {
          if (available) {
            next()
            return
          }

          writeBackendUnavailableResponse(response)
        })
      })
    },
  }
}

export function createBackendProxyOptions(target: string): ProxyOptions {
  return {
    target,
    changeOrigin: true,
    ws: false,
  }
}

export default defineConfig(({ command }) => {
  const clientWebSocketBaseUrl = resolveClientWebSocketBaseUrl(command, process.env.VITE_WS_BASE_URL, backendTarget)
  process.env.VITE_WS_BASE_URL = clientWebSocketBaseUrl

  return {
    plugins: [createBackendAvailabilityGuard(backendTarget), vue(), tailwindcss()],
    define: {
      'import.meta.env.VITE_WS_BASE_URL': JSON.stringify(clientWebSocketBaseUrl),
    },
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      host: '127.0.0.1',
      port: 4173,
      strictPort: true,
      proxy: {
        '^/(api|healthz|readyz|plugin-ui)': createBackendProxyOptions(backendTarget),
      },
    },
    preview: {
      host: '127.0.0.1',
      port: 4173,
      strictPort: true,
    },
    build: {
      chunkSizeWarningLimit: 1400,
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (id.includes('node_modules')) {
              if (
                id.includes('/vue/')
                || id.includes('/vue-router/')
                || id.includes('/pinia/')
                || id.includes('/@vue/')
              ) {
                return 'vue-vendor'
              }
              if (id.includes('/ant-design-vue/') || id.includes('/@ant-design/')) {
                return 'antd-vendor'
              }
              if (
                id.includes('/@vueuse/')
                || id.includes('/popmotion/')
                || id.includes('/framesync/')
              ) {
                return 'utils-vendor'
              }
            }
          },
        },
      },
    },
    test: {
      environment: 'jsdom',
      globals: true,
      setupFiles: ['./tests/unit/setup.ts'],
      css: true,
      include: ['tests/unit/**/*.spec.ts'],
      coverage: {
        provider: 'v8',
        reporter: ['text-summary'],
        thresholds: {
          statements: 40,
          lines: 40,
          functions: 40,
          branches: 25,
        },
      },
    },
  }
})
