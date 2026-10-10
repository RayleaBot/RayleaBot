import assert from 'node:assert/strict'

const colorPattern = /^#[0-9a-f]{6}$/i
const pathPattern = /^[MmLlHhVvCcSsQqTtAaZz0-9eE+.,\s-]+$/

export function validateMark(mark) {
  const viewBox = typeof mark?.viewBox === 'string' ? mark.viewBox.trim().split(/\s+/).map(Number) : []
  assert(viewBox.length === 4 && viewBox.every(Number.isFinite) && viewBox[2] > 0 && viewBox[3] > 0, 'Invalid mark viewBox')
  assert.deepEqual(Object.keys(mark.themes ?? {}).sort(), ['dark', 'light'], 'Mark must provide independent light and dark themes')
  for (const [mode, theme] of Object.entries(mark.themes)) {
    assert(Array.isArray(theme.paths) && theme.paths.length > 0, `Missing ${mode} mark paths`)
    assert(theme.paths.every(part => typeof part.d === 'string' && /^[Mm]/.test(part.d) && pathPattern.test(part.d)
      && colorPattern.test(part.fill) && ['nonzero', 'evenodd'].includes(part.fillRule)
      && Number.isFinite(part.opacity) && part.opacity >= 0 && part.opacity <= 1), `Invalid ${mode} vector path paint`)
    const outline = theme.outline
    assert(outline && Number.isInteger(outline.pathIndex) && theme.paths[outline.pathIndex]
      && colorPattern.test(outline.color) && Number.isFinite(outline.width) && outline.width > 0, `Invalid ${mode} mark outline`)
  }
  return mark
}

export function renderMarkSvg(mark, mode) {
  validateMark(mark)
  const theme = mark.themes[mode]
  assert(theme, `Unknown mark theme: ${mode}`)
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${mark.viewBox}">
  <path d="${theme.paths[theme.outline.pathIndex].d}" fill="none" stroke="${theme.outline.color}" stroke-width="${theme.outline.width}" stroke-linejoin="round"/>
${theme.paths.map(part => `  <path d="${part.d}" fill="${part.fill}" fill-rule="${part.fillRule}" opacity="${part.opacity}"/>`).join('\n')}
</svg>
`
}
