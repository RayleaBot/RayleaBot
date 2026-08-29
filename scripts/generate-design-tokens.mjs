import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'
import { fileURLToPath } from 'node:url'

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url))
const repositoryRoot = path.resolve(scriptDirectory, '..')
const tokenPath = path.join(repositoryRoot, 'design', 'tokens.json')
const allowlistPath = path.join(repositoryRoot, 'design', 'color-literal-allowlist.json')
const checkMode = process.argv.includes('--check')

const source = JSON.parse(fs.readFileSync(tokenPath, 'utf8'))
const mark = JSON.parse(fs.readFileSync(path.join(repositoryRoot, 'design', 'mark.json'), 'utf8'))
if (typeof mark.viewBox !== 'string' || !Array.isArray(mark.paths) || !mark.paths.length
  || mark.paths.some((part) => typeof part.d !== 'string' || !(part.opacity >= 0 && part.opacity <= 1))) {
  throw new Error('design/mark.json must contain a viewBox and finite-opacity vector paths')
}
const literalAllowlist = JSON.parse(fs.readFileSync(allowlistPath, 'utf8'))
const allowedLiteralFiles = new Set(Object.keys(literalAllowlist.files ?? {}))
const changedFiles = []
const errors = []

function getNode(tokenName) {
  return tokenName.split('.').reduce((current, part) => current?.[part], source)
}

function resolveRawValue(value, stack = []) {
  if (typeof value === 'string') {
    const reference = value.match(/^\{([^}]+)\}$/)
    if (!reference) {
      return value
    }

    const tokenName = reference[1]
    if (stack.includes(tokenName)) {
      throw new Error(`Circular design token reference: ${[...stack, tokenName].join(' -> ')}`)
    }
    return resolveToken(tokenName, [...stack, tokenName])
  }

  if (Array.isArray(value)) {
    return value.map((item) => resolveRawValue(item, stack))
  }

  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, resolveRawValue(item, stack)]))
  }

  return value
}

function resolveToken(tokenName, stack = [tokenName]) {
  const node = getNode(tokenName)
  if (!node || !Object.hasOwn(node, '$value')) {
    throw new Error(`Unknown design token: ${tokenName}`)
  }
  return resolveRawValue(node.$value, stack)
}

function toCssValue(value) {
  if (value && typeof value === 'object' && !Array.isArray(value) && 'value' in value && 'unit' in value) {
    return value.value === 0 ? '0' : `${value.value}${value.unit}`
  }
  if (value && typeof value === 'object' && !Array.isArray(value) && value.colorSpace === 'srgb' && Array.isArray(value.components)) {
    if (typeof value.hex === 'string') {
      const alpha = value.alpha ?? 1
      if (alpha < 1) {
        const alphaHex = Math.round(alpha * 255).toString(16).padStart(2, '0').toUpperCase()
        return `${value.hex}${alphaHex}`
      }
      return value.hex
    }
    const channels = value.components.map((component) => Math.round(component * 255))
    const alpha = value.alpha ?? 1
    return alpha < 1
      ? `rgb(${channels.join(' ')} / ${Number((alpha * 100).toFixed(3))}%)`
      : `rgb(${channels.join(' ')})`
  }
  if (value && typeof value === 'object' && !Array.isArray(value) && 'color' in value && 'offsetX' in value && 'offsetY' in value && 'blur' in value && 'spread' in value) {
    const dimensions = [value.offsetX, value.offsetY, value.blur]
    if (value.spread?.value !== 0) {
      dimensions.push(value.spread)
    }
    return `${value.inset ? 'inset ' : ''}${dimensions.map(toCssValue).join(' ')} ${toCssValue(value.color)}`
  }
  if (Array.isArray(value) && value.length > 0 && value.every((item) => item && typeof item === 'object' && 'color' in item && 'offsetX' in item && 'offsetY' in item && 'blur' in item && 'spread' in item)) {
    return value.map(toCssValue).join(', ')
  }
  if (Array.isArray(value) && value.length === 4 && value.every((item) => typeof item === 'number')) {
    return `cubic-bezier(${value.join(', ')})`
  }
  if (Array.isArray(value)) {
    return value.map((item) => /\s/.test(item) && !/^(system-ui|sans-serif|monospace)$/.test(item) ? `'${item}'` : item).join(', ')
  }
  return String(value)
}

function isReference(value) {
  return typeof value === 'string' && /^\{[^}]+\}$/.test(value)
}

function validateDimension(value, tokenName, supportedUnits) {
  if (!value || typeof value !== 'object' || Array.isArray(value) || typeof value.value !== 'number' || !supportedUnits.has(value.unit)) {
    errors.push(`${tokenName} is not a valid DTCG dimension`)
  }
}

function validateColor(value, tokenName) {
  const valid = value
    && typeof value === 'object'
    && !Array.isArray(value)
    && value.colorSpace === 'srgb'
    && Array.isArray(value.components)
    && value.components.length === 3
    && value.components.every((component) => typeof component === 'number' && component >= 0 && component <= 1)
    && (value.alpha === undefined || (typeof value.alpha === 'number' && value.alpha >= 0 && value.alpha <= 1))
    && (value.hex === undefined || /^#[\da-f]{6}$/i.test(value.hex))
  if (!valid) {
    errors.push(`${tokenName} is not a valid DTCG sRGB color`)
  }
}

function validateShadow(value, tokenName) {
  const shadows = Array.isArray(value) ? value : [value]
  for (const [index, shadow] of shadows.entries()) {
    const shadowName = shadows.length === 1 ? tokenName : `${tokenName}[${index}]`
    if (isReference(shadow)) {
      continue
    }
    if (!shadow || typeof shadow !== 'object' || Array.isArray(shadow)) {
      errors.push(`${shadowName} is not a valid DTCG shadow`)
      continue
    }
    if (isReference(shadow.color)) {
      // The referenced color is resolved by resolveToken before generation.
    } else {
      validateColor(shadow.color, `${shadowName}.color`)
    }
    for (const field of ['offsetX', 'offsetY', 'blur', 'spread']) {
      if (isReference(shadow[field])) {
        continue
      }
      validateDimension(shadow[field], `${shadowName}.${field}`, new Set(['px', 'rem']))
    }
    if (shadow.inset !== undefined && typeof shadow.inset !== 'boolean') {
      errors.push(`${shadowName}.inset must be a boolean`)
    }
  }
}

function validateTokenValue(value, type, tokenName) {
  if (isReference(value)) {
    return
  }
  switch (type) {
    case 'color':
      validateColor(value, tokenName)
      break
    case 'dimension':
      validateDimension(value, tokenName, new Set(['px', 'rem']))
      break
    case 'duration':
      validateDimension(value, tokenName, new Set(['ms', 's']))
      break
    case 'fontFamily':
      if (!(typeof value === 'string' || (Array.isArray(value) && value.every((item) => typeof item === 'string')))) {
        errors.push(`${tokenName} is not a valid DTCG font family`)
      }
      break
    case 'cubicBezier':
      if (!(Array.isArray(value) && value.length === 4 && value.every((item) => typeof item === 'number') && value[0] >= 0 && value[0] <= 1 && value[2] >= 0 && value[2] <= 1)) {
        errors.push(`${tokenName} is not a valid DTCG cubic Bézier value`)
      }
      break
    case 'number':
      if (typeof value !== 'number') {
        errors.push(`${tokenName} is not a valid DTCG number`)
      }
      break
    case 'shadow':
      validateShadow(value, tokenName)
      break
    default:
      errors.push(`${tokenName} uses unsupported DTCG type ${type}`)
  }
}

function validateTokenGroup(group, groupName = '', inheritedType) {
  const groupType = group.$type ?? inheritedType
  for (const [name, value] of Object.entries(group)) {
    if (name.startsWith('$')) {
      continue
    }
    const tokenName = groupName ? `${groupName}.${name}` : name
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      errors.push(`${tokenName} must be a DTCG token or group`)
      continue
    }
    if (Object.hasOwn(value, '$value')) {
      const tokenType = value.$type ?? groupType
      if (!tokenType) {
        errors.push(`${tokenName} has no DTCG type`)
        continue
      }
      validateTokenValue(value.$value, tokenType, tokenName)
      continue
    }
    validateTokenGroup(value, tokenName, value.$type ?? groupType)
  }
}

function validateDtcgSource() {
  const expectedSchema = 'https://www.designtokens.org/schemas/2025.10/format.json'
  if (source.$schema !== expectedSchema) {
    errors.push(`design/tokens.json must declare ${expectedSchema}`)
  }
  validateTokenGroup(source)
}

validateDtcgSource()

function css(tokenName) {
  return toCssValue(resolveToken(tokenName))
}

const themeFields = [
  'attention',
  'attentionSoft',
  'border',
  'borderControl',
  'brandFill',
  'brandForeground',
  'brandSoft',
  'canvas',
  'chrome',
  'chromeMuted',
  'chromeText',
  'danger',
  'dangerSoft',
  'focus',
  'onAttention',
  'onBrand',
  'success',
  'successSoft',
  'surface',
  'surfaceRaised',
  'surfaceSoft',
  'text',
  'textMuted',
  'warning',
  'warningSoft',
]

const componentFields = {
  brandFillHover: 'primary.hover',
  brandFillPressed: 'primary.pressed',
  navHover: 'navigation.hover',
  navSelected: 'navigation.selected',
  navSelectedText: 'navigation.selectedText',
  authCanvasFocus: 'auth.canvasFocus',
  authCanvasWash: 'auth.canvasWash',
  authControl: 'auth.control',
  authControlHover: 'auth.controlHover',
}

function resolvedTheme(mode) {
  const output = Object.fromEntries(themeFields.map((field) => [field, css(`semantic.${mode}.color.${field}`)]))
  for (const [field, tokenName] of Object.entries(componentFields)) {
    output[field] = css(`component.${mode}.${tokenName}`)
  }
  output.shadowSurface = css(`semantic.${mode}.shadow.surface`)
  output.shadowFloating = css(`semantic.${mode}.shadow.floating`)
  return output
}

const themes = {
  light: resolvedTheme('light'),
  dark: resolvedTheme('dark'),
}

function renderTypescriptObject(value, indent = 0, quote = "'") {
  const pad = ' '.repeat(indent)
  const childPad = ' '.repeat(indent + 2)
  return `{
${Object.entries(value).map(([key, item]) => `${childPad}${key}: ${quote}${item}${quote},`).join('\n')}
${pad}}`
}

function renderMarkDefinition() {
  return `export const rayleaMark = ${JSON.stringify(mark, null, 2)} as const\n`
}

function renderWebTokens() {
  const fields = [...themeFields, ...Object.keys(componentFields), 'shadowSurface', 'shadowFloating'].sort()
  return `// Generated by scripts/generate-design-tokens.mjs from design/tokens.json. Do not edit.

${renderMarkDefinition()}
export interface WebThemeTokens {
${fields.map((field) => `  ${field}: string`).join('\n')}
}

export const webThemes: Record<'light' | 'dark', WebThemeTokens> = {
  light: ${renderTypescriptObject(themes.light, 2)},
  dark: ${renderTypescriptObject(themes.dark, 2)},
}
`
}

function renderLauncherTokens() {
  const fields = [...themeFields, ...Object.keys(componentFields), 'shadowSurface', 'shadowFloating'].sort()
  return `// Generated by scripts/generate-design-tokens.mjs from design/tokens.json. Do not edit.

${renderMarkDefinition()}
export interface GeneratedLauncherThemeTokens {
${fields.map((field) => `  ${field}: string;`).join('\n')}
}

export const launcherGeneratedThemes: Record<'light' | 'dark', GeneratedLauncherThemeTokens> = {
  light: ${renderTypescriptObject(themes.light, 2, '"').replace(/,(?=\n\s*})/g, ',')},
  dark: ${renderTypescriptObject(themes.dark, 2, '"').replace(/,(?=\n\s*})/g, ',')},
};
`
}

function renderWebFavicon() {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${mark.viewBox}">
  <style>
    :root { color: ${themes.light.brandForeground}; }
    @media (prefers-color-scheme: dark) { :root { color: ${themes.dark.brandForeground}; } }
  </style>
${mark.paths.map((part) => `  <path d="${part.d}" fill="currentColor" opacity="${part.opacity}"/>`).join('\n')}
</svg>
`
}

function renderTypographyCss() {
  return `/* Generated by scripts/generate-design-tokens.mjs. Font license: fonts/OFL-Noto-Sans-SC.txt. */
@import "../templates/help.menu/assets/fonts/noto-sans-sc/result.css";

:root {
  --font-display: ${css('base.font.family.display')};
}
`
}

function renderThemeVariables(mode) {
  const theme = themes[mode]
  return `  color-scheme: ${mode};
  --bg: ${theme.canvas};
  --surface: ${theme.surface};
  --surface-strong: ${theme.surface};
  --surface-raised: ${theme.surfaceRaised};
  --surface-soft: ${theme.surfaceSoft};
  --surface-accent: ${theme.brandSoft};
  --surface-attention: ${theme.attentionSoft};
  --surface-success: ${theme.successSoft};
  --surface-warning: ${theme.warningSoft};
  --surface-danger: ${theme.dangerSoft};
  --surface-inverse: ${theme.chrome};
  --text: ${theme.text};
  --muted: ${theme.textMuted};
  --text-inverse: ${theme.chromeText};
  --brand-fill: ${theme.brandFill};
  --brand-fill-hover: ${theme.brandFillHover};
  --brand-fill-pressed: ${theme.brandFillPressed};
  --brand-foreground: ${theme.brandForeground};
  --brand-stroke: ${theme.brandForeground};
  --brand-soft: ${theme.brandSoft};
  --on-brand: ${theme.onBrand};
  --focus: ${theme.focus};
  --chrome: ${theme.chrome};
  --chrome-text: ${theme.chromeText};
  --chrome-muted: ${theme.chromeMuted};
  --nav-hover: ${theme.navHover};
  --nav-selected: ${theme.navSelected};
  --nav-selected-text: ${theme.navSelectedText};
  --attention: ${theme.attention};
  --attention-soft: ${theme.attentionSoft};
  --on-attention: ${theme.onAttention};
  --success: ${theme.success};
  --success-soft: ${theme.successSoft};
  --warning: ${theme.warning};
  --warning-soft: ${theme.warningSoft};
  --danger: ${theme.danger};
  --danger-soft: ${theme.dangerSoft};
  --info: ${theme.textMuted};
  --text-accent: ${theme.text};
  --text-attention: ${theme.attention};
  --text-success: ${theme.success};
  --text-warning: ${theme.warning};
  --text-danger: ${theme.danger};
  --border: ${theme.border};
  --border-strong: ${theme.borderControl};
  --border-accent: ${theme.borderControl};
  --border-attention: ${theme.attention};
  --border-success: ${theme.success};
  --border-warning: ${theme.warning};
  --border-danger: ${theme.danger};
  --shadow-xs: ${theme.shadowSurface};
  --shadow-sm: ${theme.shadowSurface};
  --shadow: ${theme.shadowSurface};
  --shadow-lg: ${theme.shadowFloating};
  --shadow-card: ${theme.shadowSurface};
  --shadow-elevated: ${theme.shadowFloating};
  --shadow-floating: ${theme.shadowFloating};
  --app-page-bg: ${theme.canvas};
  --app-card-bg: ${theme.surface};
  --app-border: ${theme.border};
  --app-primary: ${theme.brandForeground};
  --app-success: ${theme.success};
  --app-warning: ${theme.warning};
  --app-danger: ${theme.danger};
  --app-text: ${theme.text};
  --app-text-secondary: ${theme.textMuted};
  --sider-bg: ${theme.chrome};
  --sider-brand-bg: ${theme.chrome};
  --sider-brand-border: ${theme.border};
  --sider-brand-text: ${theme.chromeText};
  --sider-brand-subtle: ${theme.chromeMuted};
  --sider-menu-text: ${theme.chromeText};
  --sider-menu-hover-bg: ${theme.navHover};
  --sider-menu-active: ${theme.navSelectedText};
  --sider-menu-active-bg: ${theme.navSelected};
  --primary: ${theme.brandForeground};
  --accent: ${theme.text};
  --accent-soft: ${theme.brandSoft};
  --foreground: ${theme.text};
  --fg: ${theme.text};
  --fg-light: ${theme.textMuted};
  --text-secondary: ${theme.textMuted};
  --theme-text: ${theme.text};
  --border-subtle: ${theme.border};
  --color-border-subtle: ${theme.border};
  --app-background: ${theme.canvas};
  --app-bg-card: ${theme.surface};
  --control-surface: ${theme.surfaceRaised};
  --code-surface: ${theme.surfaceSoft};
  --code-text: ${theme.text};`
}

function renderWebScss() {
  return `// Generated by scripts/generate-design-tokens.mjs from design/tokens.json. Do not edit.

:root,
[data-theme='light'] {
  --font-sans: ${css('base.font.family.sans')};
  --font-mono: ${css('base.font.family.mono')};
  --font-size-xs: ${css('base.font.size.xs')};
  --font-size-sm: ${css('base.font.size.sm')};
  --font-size-md: ${css('base.font.size.md')};
  --font-size-lg: ${css('base.font.size.lg')};
  --font-size-xl: ${css('base.font.size.xl')};
  --font-size-display: ${css('base.font.size.display')};
  --font-size-hero: ${css('base.font.size.hero')};
  --app-border-radius: ${css('base.radius.md')};
  --app-card-radius: ${css('base.radius.lg')};
  --app-content-max-width: none;
  --app-control-height: 36px;
  --app-font-size: ${css('base.font.size.md')};
  --app-layout-gap: ${css('base.space.lg')};
  --app-page-gap: ${css('base.space.lg')};
  --app-page-header-gap: ${css('base.space.lg')};
  --app-page-toolbar-gap: ${css('base.space.lg')};
  --app-shell-padding-inline: ${css('base.space.xl')};
  --app-shell-padding-block: ${css('base.space.lg')};
  --motion-fast: ${css('base.motion.duration.feedback')};
  --motion-content: ${css('base.motion.duration.content')};
  --motion-overlay: ${css('base.motion.duration.overlay')};
  --motion-workspace: ${css('base.motion.duration.workspace')};
  --motion-easing: ${css('base.motion.easing.standard')};
  --motion-workspace-easing: ${css('base.motion.easing.workspace')};
  --z-sticky: ${css('base.layer.sticky')};
  --z-menu: ${css('base.layer.menu')};
  --z-drawer: ${css('base.layer.drawer')};
  --z-modal: ${css('base.layer.modal')};
  --z-toast: ${css('base.layer.toast')};
  --z-emergency: ${css('base.layer.emergency')};
  --space-xs: ${css('base.space.xs')};
  --space-sm: ${css('base.space.sm')};
  --space-md: ${css('base.space.md')};
  --space-lg: ${css('base.space.lg')};
  --space-xl: ${css('base.space.xl')};
  --space-2xl: ${css('base.space.xxl')};
  --radius-xs: ${css('base.radius.xs')};
  --radius-sm: ${css('base.radius.sm')};
  --radius-md: ${css('base.radius.md')};
  --radius-lg: ${css('base.radius.lg')};
  --radius-xl: ${css('base.radius.lg')};
  --radius-full: ${css('base.radius.full')};
${renderThemeVariables('light')}
}

[data-theme='dark'] {
${renderThemeVariables('dark')}
}
`
}

function renderDesignFrontmatter() {
  const colorLines = Object.entries(renderColorMeta()).map(([name, item]) => `  ${name}: ${JSON.stringify(item.canonical)}`)
  const typeRoles = [
    ['headline', 'display', 'display'], ['title', 'display', 'xl'], ['section', 'display', 'lg'],
    ['body', 'sans', 'md'], ['label', 'sans', 'sm'], ['mono', 'mono', 'sm'],
  ]
  const typeLines = typeRoles.flatMap(([role, family, size]) => [
    `  ${role}:`, `    fontFamily: ${JSON.stringify(css(`base.font.family.${family}`))}`,
    `    fontSize: ${JSON.stringify(css(`base.font.size.${size}`))}`,
  ])
  const radiusLines = ['xs', 'sm', 'md', 'lg', 'full'].map((name) => `  ${name}: ${JSON.stringify(css(`base.radius.${name}`))}`)
  const spaceLines = ['xs', 'sm', 'md', 'lg', 'xl', 'xxl'].map((name) => `  ${name}: ${JSON.stringify(css(`base.space.${name}`))}`)
  return ['---', 'name: RayleaBot', 'description: 雾白表面、青瓷绿与紧凑工作区组成的自托管机器人管理界面',
    'colors:', ...colorLines, 'typography:', ...typeLines, 'rounded:', ...radiusLines, 'spacing:', ...spaceLines,
    'components:',
    '  button-primary:', '    backgroundColor: "{colors.light-primary}"', '    textColor: "{colors.light-on-brand}"', '    rounded: "{rounded.md}"', '    height: "36px"',
    '  button-primary-hover:', '    backgroundColor: "{colors.light-primary-hover}"',
    '  button-primary-active:', '    backgroundColor: "{colors.light-primary-pressed}"',
    '  button-attention:', '    backgroundColor: "{colors.light-attention}"', '    textColor: "{colors.light-on-attention}"', '    rounded: "{rounded.md}"', '    height: "36px"',
    '  input:', '    backgroundColor: "{colors.light-surface-raised}"', '    textColor: "{colors.light-text}"', '    rounded: "{rounded.md}"', '    height: "36px"',
    '  navigation-item:', '    backgroundColor: "{colors.light-nav-selected}"', '    textColor: "{colors.light-nav-selected-text}"', '    rounded: "{rounded.md}"',
    '  status-chip:', '    backgroundColor: "{colors.light-success-soft}"', '    textColor: "{colors.light-success}"', '    rounded: "{rounded.full}"',
    '  section-surface:', '    backgroundColor: "{colors.light-surface}"', '    textColor: "{colors.light-text}"', '    rounded: "{rounded.lg}"',
    '  attention-callout:', '    backgroundColor: "{colors.light-attention-soft}"', '    textColor: "{colors.light-attention}"', '    rounded: "{rounded.lg}"',
    '  data-row:', '    backgroundColor: "{colors.light-surface}"', '    textColor: "{colors.light-text}"',
    '---'].join('\n')
}

function colorMetaEntry(role, displayName, canonical, tonalRamp) {
  return { role, displayName, canonical, tonalRamp }
}

function renderColorMeta() {
  const celadonRamp = ['1000', '900', '800', '700', '600', '500', '400', '300', '200', '100', '50'].map((step) => css(`base.color.celadon.${step}`))
  const neutralRamp = [
    themes.dark.canvas,
    themes.dark.surface,
    themes.dark.surfaceRaised,
    themes.dark.border,
    themes.dark.borderControl,
    themes.light.textMuted,
    themes.light.border,
    themes.light.canvas,
    themes.light.surface,
    themes.light.surfaceRaised,
  ]
  const entries = {}
  for (const step of ['50', '100', '200', '300', '400', '500', '600', '700', '800', '900', '1000']) {
    entries[`celadon-${step}`] = colorMetaEntry('brand-base', `青瓷 ${step}`, css(`base.color.celadon.${step}`), celadonRamp)
  }
  const labels = {
    canvas: '画布', surface: '表面', surfaceRaised: '抬升表面', text: '正文', textMuted: '辅文', border: '结构边界',
    borderControl: '控件边界', brandFill: '主操作', brandFillHover: '主操作悬停', brandFillPressed: '主操作按下', onBrand: '主操作内容',
    brandForeground: '品牌前景', brandSoft: '中性淡面', focus: '焦点', chrome: '导航表面',
    navSelected: '导航选中', navSelectedText: '导航选中文字',
    attention: '人工关注', attentionSoft: '人工关注淡面', onAttention: '人工关注内容',
    success: '成功', successSoft: '成功淡面', warning: '警告', danger: '危险',
  }
  for (const mode of ['light', 'dark']) {
    for (const [field, label] of Object.entries(labels)) {
      const role = ['brandFill', 'brandFillHover', 'brandFillPressed', 'brandForeground'].includes(field)
        ? 'primary'
        : ['attention', 'success', 'warning', 'danger'].includes(field)
          ? `semantic-${field}`
          : 'neutral'
      const ramp = role === 'primary' ? celadonRamp : neutralRamp
      const aliases = { borderControl: 'control-border', brandFill: 'primary', brandFillHover: 'primary-hover', brandFillPressed: 'primary-pressed' }
      const name = aliases[field] ?? field.replace(/[A-Z]/g, (letter) => `-${letter.toLowerCase()}`)
      entries[`${mode}-${name}`] = colorMetaEntry(role, `${mode === 'light' ? '浅色' : '暗色'}${label}`, themes[mode][field], ramp)
    }
  }
  return entries
}

function renderImpeccableComponents() {
  const light = themes.light
  const dark = themes.dark
  return [
    {
      name: 'Primary Button',
      kind: 'button',
      refersTo: 'button-primary',
      description: '当前工作流唯一突出的主操作。',
      html: '<button class="ds-btn-primary">保存设置</button>',
      css: `.ds-btn-primary { height: 36px; padding: 8px 14px; border: 1px solid ${light.brandFill}; border-radius: 8px; background: ${light.brandFill}; color: ${light.onBrand}; font: 600 14px/1.4 "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif; cursor: pointer; transition: background-color 160ms cubic-bezier(0.16, 1, 0.3, 1), border-color 160ms cubic-bezier(0.16, 1, 0.3, 1); } .ds-btn-primary:hover { background: ${light.brandFillHover}; border-color: ${light.brandFillHover}; } .ds-btn-primary:focus-visible { outline: 2px solid ${light.focus}; outline-offset: 2px; } .ds-btn-primary:active { background: ${light.brandFillPressed}; border-color: ${light.brandFillPressed}; } .ds-btn-primary:disabled { cursor: not-allowed; opacity: 0.58; } @media (prefers-color-scheme: dark) { .ds-btn-primary { border-color: ${dark.brandFill}; background: ${dark.brandFill}; color: ${dark.onBrand}; } .ds-btn-primary:hover { border-color: ${dark.brandFillHover}; background: ${dark.brandFillHover}; } .ds-btn-primary:focus-visible { outline-color: ${dark.focus}; } .ds-btn-primary:active { border-color: ${dark.brandFillPressed}; background: ${dark.brandFillPressed}; } }`,
    },
    {
      name: 'Attention Button',
      kind: 'button',
      refersTo: 'button-attention',
      description: '需要人工判断或确认的明确操作。',
      html: '<button class="ds-btn-attention">确认继续</button>',
      css: `.ds-btn-attention { height: 36px; padding: 8px 14px; border: 1px solid ${light.attention}; border-radius: 8px; background: ${light.attention}; color: ${light.onAttention}; font: 600 14px/1.4 "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif; cursor: pointer; } .ds-btn-attention:focus-visible { outline: 2px solid ${light.focus}; outline-offset: 2px; } @media (prefers-color-scheme: dark) { .ds-btn-attention { border-color: ${dark.attention}; background: ${dark.attention}; color: ${dark.onAttention}; } .ds-btn-attention:focus-visible { outline-color: ${dark.focus}; } }`,
    },
    {
      name: 'Text Input',
      kind: 'input',
      refersTo: 'input',
      description: '带持续标签、控件边界和清晰焦点的标准输入框。',
      html: '<label class="ds-field"><span>服务地址</span><input value="http://127.0.0.1:8080"></label>',
      css: `.ds-field { display: grid; gap: 6px; color: ${light.text}; font: 600 13px/1.4 "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif; } .ds-field input { width: 100%; height: 36px; padding: 8px 12px; border: 1px solid ${light.borderControl}; border-radius: 8px; background: ${light.surfaceRaised}; color: ${light.text}; font: 400 14px/1.55 "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif; } .ds-field input:focus-visible { outline: 2px solid ${light.focus}; outline-offset: 2px; } @media (prefers-color-scheme: dark) { .ds-field { color: ${dark.text}; } .ds-field input { border-color: ${dark.borderControl}; background: ${dark.surfaceRaised}; color: ${dark.text}; } .ds-field input:focus-visible { outline-color: ${dark.focus}; } }`,
    },
    {
      name: 'Selected Navigation Item',
      kind: 'nav',
      refersTo: 'navigation-item',
      description: '使用完整中性色面、高对比文字和可见焦点表达当前工作区。',
      html: '<a class="ds-nav-item" href="#" aria-current="page">系统状态</a>',
      css: `.ds-nav-item { display: inline-flex; align-items: center; min-height: 40px; padding: 8px 12px; border-radius: 8px; background: ${light.navSelected}; color: ${light.navSelectedText}; font: 600 14px/1.4 "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif; text-decoration: none; } .ds-nav-item:focus-visible { outline: 2px solid ${light.chromeMuted}; outline-offset: 2px; } @media (prefers-color-scheme: dark) { .ds-nav-item { background: ${dark.navSelected}; color: ${dark.navSelectedText}; } .ds-nav-item:focus-visible { outline-color: ${dark.chromeMuted}; } }`,
    },
    {
      name: 'Status Chip',
      kind: 'chip',
      refersTo: 'status-chip',
      description: '同时使用语义色、文字和形状表达状态。',
      html: '<span class="ds-status-chip"><svg width="14" height="14" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2"><path d="m5 12 4 4L19 6"/></svg>运行正常</span>',
      css: `.ds-status-chip { display: inline-flex; align-items: center; min-height: 24px; padding: 4px 8px; border: 1px solid ${light.success}; border-radius: 999px; background: ${light.successSoft}; color: ${light.success}; font: 600 13px/1.4 "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif; } @media (prefers-color-scheme: dark) { .ds-status-chip { border-color: ${dark.success}; background: ${dark.successSoft}; color: ${dark.success}; } }`,
    },
    {
      name: 'Section Surface',
      kind: 'card',
      refersTo: 'section-surface',
      description: '只承载具有独立任务边界的内容。',
      html: '<section class="ds-section-surface"><h3>运行环境</h3><p>检查托管运行时与模板资源。</p></section>',
      css: `.ds-section-surface { max-width: 560px; padding: 16px; border: 1px solid ${light.border}; border-radius: 12px; background: ${light.surface}; color: ${light.text}; font: 400 14px/1.55 "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif; } .ds-section-surface h3 { margin: 0 0 8px; font-family: "Noto Sans SC", "Microsoft YaHei UI", sans-serif; font-size: 18px; } .ds-section-surface p { margin: 0; color: ${light.textMuted}; } @media (prefers-color-scheme: dark) { .ds-section-surface { border-color: ${dark.border}; background: ${dark.surface}; color: ${dark.text}; } .ds-section-surface p { color: ${dark.textMuted}; } }`,
    },
    {
      name: 'Attention Callout',
      kind: 'custom',
      refersTo: 'attention-callout',
      description: '解释需要人工判断的事项并提供直接操作。',
      html: '<aside class="ds-attention"><strong>需要人工确认</strong><p>继续前请检查插件来源和声明能力。</p></aside>',
      css: `.ds-attention { display: grid; gap: 4px; padding: 12px 16px; border: 1px solid ${light.attention}; border-radius: 12px; background: ${light.attentionSoft}; color: ${light.attention}; font: 400 14px/1.55 "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif; } .ds-attention p { margin: 0; color: ${light.text}; } @media (prefers-color-scheme: dark) { .ds-attention { border-color: ${dark.attention}; background: ${dark.attentionSoft}; color: ${dark.attention}; } .ds-attention p { color: ${dark.text}; } }`,
    },
    {
      name: 'Data Row',
      kind: 'custom',
      refersTo: 'data-row',
      description: '紧凑展示对象、状态和右侧操作。',
      html: '<div class="ds-data-row" role="row" tabindex="0"><strong>subscription_hub</strong><span>运行中</span><button>查看</button></div>',
      css: `.ds-data-row { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: 12px; min-height: 44px; padding: 10px 12px; border: 1px solid ${light.border}; border-radius: 8px; background: ${light.surface}; color: ${light.text}; font: 400 14px/1.4 "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif; } .ds-data-row:hover { border-color: ${light.border}; background: ${light.brandSoft}; } .ds-data-row:focus-visible { outline: 2px solid ${light.focus}; outline-offset: 2px; } .ds-data-row button { border: 0; background: transparent; color: ${light.text}; font: 600 13px/1.4 "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif; } @media (prefers-color-scheme: dark) { .ds-data-row { border-color: ${dark.border}; background: ${dark.surface}; color: ${dark.text}; } .ds-data-row:hover { border-color: ${dark.border}; background: ${dark.brandSoft}; } .ds-data-row:focus-visible { outline-color: ${dark.focus}; } .ds-data-row button { color: ${dark.text}; } }`,
    },
  ]
}

function renderImpeccableNarrative() {
  const markdown = fs.readFileSync(path.join(repositoryRoot, 'DESIGN.md'), 'utf8').replace(/\r\n/g, '\n')
  const overview = markdown.match(/## Overview\n([\s\S]*?)\n## Colors/)[1]
  const body = overview.split('**Key Characteristics:**')[0]
  const characteristics = overview.split('**Key Characteristics:**')[1].split('\n\n')[1]
  const guardrails = markdown.split("## Do's and Don'ts")[1]
  const list = (value) => value.split('\n').filter((line) => line.startsWith('- ')).map((line) => line.slice(2))
  const sectionNames = { Colors: 'colors', Typography: 'typography', 'Elevation & Depth': 'elevation', Components: 'components' }
  const rules = []
  let section = ''
  for (const line of markdown.split('\n')) {
    if (line.startsWith('## ')) section = sectionNames[line.slice(3)] ?? ''
    const match = line.match(/^\*\*(The .+? Rule)\.\*\* (.+)$/)
    if (match) rules.push({ name: match[1], body: match[2], section })
  }
  return {
    northStar: body.match(/\*\*Creative North Star: "(.+)"\*\*/)[1],
    overview: body.replace(/\*\*Creative North Star: .+?\*\*/, '').trim(),
    keyCharacteristics: list(characteristics),
    rules,
    dos: list(guardrails.split('### Do:')[1].split("### Don't:")[0]),
    donts: list(guardrails.split("### Don't:")[1]),
  }
}

function updateDesignDocument(current) {
  const frontmatter = renderDesignFrontmatter()
  if (!current.startsWith('---')) {
    throw new Error('DESIGN.md must start with YAML frontmatter')
  }
  return `${current.replace(/^---\r?\n[\s\S]*?\r?\n---/, frontmatter).trimEnd()}\n`
}

function updateImpeccable(current) {
  const document = JSON.parse(current)
  document.extensions ??= {}
  document.extensions.colorMeta = renderColorMeta()
  document.extensions.typographyMeta = {
    headline: { displayName: '页面标题', purpose: 'Noto Sans SC 紧凑页面标题。' },
    title: { displayName: '分区标题', purpose: 'Noto Sans SC 工作区和分区标题。' },
    section: { displayName: '组标题', purpose: 'Noto Sans SC 面板与字段组标题。' },
    body: { displayName: '正文', purpose: '系统无衬线栈承载说明、表单与标准控件。' },
    label: { displayName: '标签', purpose: '控件、表头与紧凑状态。' },
    mono: { displayName: '等宽数据', purpose: '日志、路径、标识符和代码。' },
  }
  document.extensions.layers = { sticky: 100, menu: 200, drawer: 300, modal: 400, toast: 500, emergency: 600 }
  document.extensions.shadows = [
    { name: 'surface-light', value: themes.light.shadowSurface, purpose: '浅色主题中的独立任务表面。' },
    { name: 'floating-light', value: themes.light.shadowFloating, purpose: '浅色主题中的菜单、抽屉和对话框。' },
    { name: 'surface-dark', value: themes.dark.shadowSurface, purpose: '暗色主题中的独立任务表面。' },
    { name: 'floating-dark', value: themes.dark.shadowFloating, purpose: '暗色主题中的菜单、抽屉和对话框。' },
  ]
  document.extensions.motion = (document.extensions.motion ?? []).map((item) => (
    ['auth-particle-network', 'auth-surface-entry'].includes(item.name)
      ? { ...item, name: 'auth-surface-entry', value: '240ms', purpose: '认证页只在进入时呈现轻量表面过渡，折叶为静态矢量，空闲时没有持续绘制循环；reduced-motion 下即时呈现。' }
      : item
  ))
  document.components = renderImpeccableComponents()
  document.narrative = renderImpeccableNarrative()
  return `${JSON.stringify(document, null, 2)}\n`
}

function stageOutput(relativePath, expectedContent) {
  const absolutePath = path.join(repositoryRoot, relativePath)
  const current = fs.existsSync(absolutePath) ? fs.readFileSync(absolutePath, 'utf8') : ''
  if (current === expectedContent) {
    return
  }
  if (checkMode) {
    errors.push(`${relativePath} is not generated from design/tokens.json`)
    return
  }
  fs.mkdirSync(path.dirname(absolutePath), { recursive: true })
  fs.writeFileSync(absolutePath, expectedContent, 'utf8')
  changedFiles.push(relativePath)
}

function parseHex(value) {
  const match = value.match(/^#([\da-f]{2})([\da-f]{2})([\da-f]{2})$/i)
  if (!match) {
    throw new Error(`Contrast checks require an opaque six-digit color, received ${value}`)
  }
  return match.slice(1).map((component) => Number.parseInt(component, 16) / 255)
}

function relativeLuminance(value) {
  return parseHex(value)
    .map((channel) => channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4)
    .reduce((total, channel, index) => total + channel * [0.2126, 0.7152, 0.0722][index], 0)
}

function contrastRatio(foreground, background) {
  const [lighter, darker] = [relativeLuminance(foreground), relativeLuminance(background)].sort((left, right) => right - left)
  return (lighter + 0.05) / (darker + 0.05)
}

function assertContrast(label, foreground, background, minimum, documented) {
  const actual = contrastRatio(foreground, background)
  if (actual + 0.005 < minimum || Math.abs(actual - documented) > 0.03) {
    errors.push(`${label} contrast is ${actual.toFixed(2)}:1; expected ${documented.toFixed(2)}:1 and at least ${minimum}:1`)
  }
}

function validateContrast() {
  assertContrast('Light primary action', themes.light.onBrand, themes.light.brandFill, 4.5, 5.87)
  assertContrast('Dark primary action', themes.dark.onBrand, themes.dark.brandFill, 4.5, 8.22)
  assertContrast('Light muted text', themes.light.textMuted, themes.light.surface, 4.5, 5.74)
  assertContrast('Dark muted text', themes.dark.textMuted, themes.dark.surface, 4.5, 7.43)
  assertContrast('Light control boundary', themes.light.borderControl, themes.light.surface, 3, 3.45)
  assertContrast('Dark control boundary', themes.dark.borderControl, themes.dark.surface, 3, 4.52)
  assertContrast('Light focus ring', themes.light.focus, themes.light.surface, 3, 7.46)
  assertContrast('Dark focus ring', themes.dark.focus, themes.dark.surface, 3, 9.16)
}

function walkFiles(startPath) {
  if (!fs.existsSync(startPath)) {
    return []
  }
  const stats = fs.statSync(startPath)
  if (stats.isFile()) {
    return [startPath]
  }
  return fs.readdirSync(startPath, { withFileTypes: true }).flatMap((entry) => {
    if (entry.name === 'node_modules' || entry.name === 'dist' || entry.name === '.git') {
      return []
    }
    return walkFiles(path.join(startPath, entry.name))
  })
}

function normalizedRelativePath(absolutePath) {
  return path.relative(repositoryRoot, absolutePath).replaceAll('\\', '/')
}

function scanRetiredColors() {
  const retiredColors = ['#66ccff', '#0a6e94', '#122032', '#264763', '#7fd6ff', '#d6f5ff']
  const scanRoots = ['DESIGN.md', '.impeccable/design.json', 'design', 'docs/design', 'web/src', 'launcher/src', 'templates']
  for (const absolutePath of scanRoots.flatMap((relativePath) => walkFiles(path.join(repositoryRoot, relativePath)))) {
    const relativePath = normalizedRelativePath(absolutePath)
    if (allowedLiteralFiles.has(relativePath)) {
      continue
    }
    const content = fs.readFileSync(absolutePath, 'utf8').toLowerCase()
    for (const retiredColor of retiredColors) {
      if (content.includes(retiredColor)) {
        errors.push(`${relativePath} still contains retired color ${retiredColor.toUpperCase()}`)
      }
    }
  }
}

function scanProductColorLiterals() {
  const generatedFiles = new Set([
    'web/src/preferences/theme-tokens.generated.ts',
    'web/src/styles/_theme-tokens.generated.scss',
    'launcher/src/shared/launcher-theme-tokens.generated.ts',
  ])
  const extensions = new Set(['.css', '.html', '.scss', '.ts', '.tsx', '.vue'])
  for (const absolutePath of ['web/src', 'launcher/src', 'templates'].flatMap((relativePath) => walkFiles(path.join(repositoryRoot, relativePath)))) {
    const relativePath = normalizedRelativePath(absolutePath)
    if (generatedFiles.has(relativePath) || allowedLiteralFiles.has(relativePath) || !extensions.has(path.extname(absolutePath))) {
      continue
    }
    const lines = fs.readFileSync(absolutePath, 'utf8').split(/\r?\n/)
    lines.forEach((line, index) => {
      const matches = [
        ...(line.match(/#[\da-f]{6}(?:[\da-f]{2})?\b/ig) ?? []),
        ...(line.match(/\b(?:rgb|rgba|hsl|hsla)\s*\(/ig) ?? []),
        ...(line.match(/(?<![-\w])(?:white|black)(?![-\w])/ig) ?? []),
      ]
      if (matches.length > 0) {
        errors.push(`${relativePath}:${index + 1} bypasses semantic tokens with ${matches.join(', ')}`)
      }
    })
  }
}

function validateLiteralAllowlist() {
  for (const [relativePath, reason] of Object.entries(literalAllowlist.files ?? {})) {
    if (!fs.existsSync(path.join(repositoryRoot, relativePath))) {
      errors.push(`${relativePath} is listed in the color literal allowlist but does not exist`)
    }
    if (typeof reason !== 'string' || reason.trim().length < 12) {
      errors.push(`${relativePath} needs a specific color literal allowlist reason`)
    }
  }
}

validateContrast()
validateLiteralAllowlist()

stageOutput('web/src/preferences/theme-tokens.generated.ts', renderWebTokens())
stageOutput('web/src/styles/_theme-tokens.generated.scss', renderWebScss())
stageOutput('web/public/favicon.svg', renderWebFavicon())
stageOutput('launcher/src/shared/launcher-theme-tokens.generated.ts', renderLauncherTokens())
stageOutput('design/typography.generated.css', renderTypographyCss())
const fontLicense = fs.readFileSync(path.join(repositoryRoot, 'templates/help.menu/assets/fonts/noto-sans-sc/OFL.txt'), 'utf8')
stageOutput('web/public/fonts/OFL-Noto-Sans-SC.txt', fontLicense)
stageOutput('launcher/src/renderer/public/fonts/OFL-Noto-Sans-SC.txt', fontLicense)

const designPath = path.join(repositoryRoot, 'DESIGN.md')
stageOutput('DESIGN.md', updateDesignDocument(fs.readFileSync(designPath, 'utf8')))

const impeccablePath = path.join(repositoryRoot, '.impeccable', 'design.json')
stageOutput('.impeccable/design.json', updateImpeccable(fs.readFileSync(impeccablePath, 'utf8')))

if (checkMode) {
  scanRetiredColors()
  scanProductColorLiterals()
}

if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join('\n'))
  process.exitCode = 1
} else if (checkMode) {
  console.log('Design tokens, contrast checks, and color drift checks are current.')
} else if (changedFiles.length > 0) {
  console.log(`Generated ${changedFiles.join(', ')}`)
} else {
  console.log('Design token outputs are already current.')
}
