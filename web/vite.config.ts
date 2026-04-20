import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

const backendTarget = process.env.VITE_BACKEND_TARGET ?? 'http://127.0.0.1:8080'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
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
      '^/(api|healthz|readyz|ws|plugin-ui)': {
        target: backendTarget,
        changeOrigin: true,
        ws: true,
      },
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
})
