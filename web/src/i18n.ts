import { createI18n } from 'vue-i18n'

import { zhCN } from '@/locales/zh-CN'

export const defaultLocale = 'zh-CN'

export const i18n = createI18n({
  legacy: false,
  locale: defaultLocale,
  fallbackLocale: defaultLocale,
  messages: {
    [defaultLocale]: zhCN,
  },
})

export function t(key: string, values?: Record<string, unknown>) {
  return i18n.global.t(key, values) as string
}
