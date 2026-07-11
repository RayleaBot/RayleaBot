import theme from 'ant-design-vue/es/theme'
import type { ThemeConfig } from 'ant-design-vue/es/config-provider/context'

import type { ThemeMode } from '@/preferences/app'

interface AuthThemeTokens {
  border: string
  canvas: string
  canvasFocus: string
  canvasWash: string
  control: string
  controlHover: string
  cool: string
  coolHover: string
  coolSoft: string
  danger: string
  onAction: string
  panelHighlight: string
  panelShadow: string
  primaryHighlight: string
  primaryShadow: string
  success: string
  surface: string
  surfaceRaised: string
  text: string
  textMuted: string
  warning: string
}

export interface AuthParticlePalette {
  line: string
  particle: string
}

const authThemes: Record<ThemeMode, AuthThemeTokens> = {
  light: {
    border: '#D8E0E4',
    canvas: '#F3F6F7',
    canvasFocus: 'rgb(102 204 255 / 14%)',
    canvasWash: 'rgb(11 107 143 / 7%)',
    control: '#FDFCF9',
    controlHover: '#F8FCFD',
    cool: '#0B6B8F',
    coolHover: '#085A79',
    coolSoft: '#E8F6FC',
    danger: '#C2414B',
    onAction: '#FAF9F5',
    panelHighlight: 'rgb(250 249 245 / 88%)',
    panelShadow: '0 2px 8px rgb(31 39 44 / 8%), 0 24px 64px rgb(31 39 44 / 8%)',
    primaryHighlight: 'rgb(250 249 245 / 24%)',
    primaryShadow: '0 8px 20px rgb(11 107 143 / 18%)',
    success: '#2F7D5C',
    surface: '#FAF9F5',
    surfaceRaised: '#FAF9F5',
    text: '#1F272C',
    textMuted: '#58656E',
    warning: '#8A5600',
  },
  dark: {
    border: '#314047',
    canvas: '#11181C',
    canvasFocus: 'rgb(102 204 255 / 12%)',
    canvasWash: 'rgb(102 204 255 / 6%)',
    control: '#141D21',
    controlHover: '#17252B',
    cool: '#66CCFF',
    coolHover: '#8ADAFF',
    coolSoft: '#16323F',
    danger: '#FF8089',
    onAction: '#11181C',
    panelHighlight: 'rgb(233 240 242 / 7%)',
    panelShadow: '0 2px 10px rgb(0 0 0 / 28%), 0 28px 72px rgb(0 0 0 / 34%)',
    primaryHighlight: 'rgb(233 240 242 / 26%)',
    primaryShadow: '0 8px 24px rgb(102 204 255 / 14%)',
    success: '#67C99B',
    surface: '#182126',
    surfaceRaised: '#202C32',
    text: '#E9F0F2',
    textMuted: '#A7B4BA',
    warning: '#F0B95A',
  },
}

export function resolveAuthThemeConfig(mode: ThemeMode): ThemeConfig {
  const tokens = authThemes[mode]
  return {
    algorithm: mode === 'dark' ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      borderRadius: 8,
      borderRadiusLG: 12,
      colorBgContainer: tokens.surface,
      colorBgElevated: tokens.surfaceRaised,
      colorBgLayout: tokens.canvas,
      colorBorder: tokens.border,
      colorError: tokens.danger,
      colorInfo: tokens.cool,
      colorPrimary: tokens.cool,
      colorSplit: tokens.border,
      colorSuccess: tokens.success,
      colorText: tokens.text,
      colorTextLightSolid: tokens.onAction,
      colorTextSecondary: tokens.textMuted,
      colorWarning: tokens.warning,
      controlHeight: 36,
      fontFamily: 'Inter, "PingFang SC", "Segoe UI", system-ui, sans-serif',
      fontSize: 14,
      wireframe: false,
    },
    components: {
      Alert: {
        borderRadiusLG: 8,
      },
      Button: {
        controlHeight: 36,
      },
      Input: {
        activeShadow: `0 0 0 2px color-mix(in srgb, ${tokens.cool} 18%, transparent)`,
        controlHeight: 36,
      },
    },
  }
}

export function resolveAuthCssVariables(mode: ThemeMode): Record<string, string> {
  const tokens = authThemes[mode]
  return {
    '--auth-border': tokens.border,
    '--auth-canvas': tokens.canvas,
    '--auth-canvas-focus': tokens.canvasFocus,
    '--auth-canvas-wash': tokens.canvasWash,
    '--auth-control': tokens.control,
    '--auth-control-hover': tokens.controlHover,
    '--auth-cool': tokens.cool,
    '--auth-cool-hover': tokens.coolHover,
    '--auth-cool-soft': tokens.coolSoft,
    '--auth-panel-highlight': tokens.panelHighlight,
    '--auth-panel-shadow': tokens.panelShadow,
    '--auth-primary-highlight': tokens.primaryHighlight,
    '--auth-primary-shadow': tokens.primaryShadow,
    '--auth-surface': tokens.surface,
    '--auth-text': tokens.text,
    '--auth-text-muted': tokens.textMuted,
  }
}

export function resolveAuthParticlePalette(mode: ThemeMode): AuthParticlePalette {
  return mode === 'dark'
    ? {
        line: 'rgb(102 204 255 / 22%)',
        particle: 'rgb(138 218 255 / 78%)',
      }
    : {
        line: 'rgb(11 107 143 / 17%)',
        particle: 'rgb(11 107 143 / 66%)',
      }
}
