import { t } from '@/i18n'

// Replaced by Vite at compile time; runtime configuration never supplies a version.
declare const __RAYLEA_BUILD_VERSION__: string

export const buildVersion = __RAYLEA_BUILD_VERSION__
export const buildVersionLabel = buildVersion === 'dev' ? t('shell.developmentVersion') : buildVersion
