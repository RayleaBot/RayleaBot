import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'
import { renderMarkSvg, validateMark } from '../design-mark.mjs'

const mark = JSON.parse(fs.readFileSync(new URL('../../design/mark.json', import.meta.url), 'utf8'))

test('both theme exports contain only self-contained vector paths', () => {
  for (const mode of ['light', 'dark']) {
    const svg = renderMarkSvg(mark, mode)
    assert.deepEqual([...new Set([...svg.matchAll(/<([a-zA-Z]+)/g)].map(match => match[1]))].sort(), ['path', 'svg'])
    assert.doesNotMatch(svg, /(?:href|url\(|data:|base64|<image|<foreignObject)/i)
    assert.match(svg, /[cC][\d.,-]/)
    assert.equal(svg, fs.readFileSync(new URL(`../../design/mark-${mode}.svg`, import.meta.url), 'utf8'))
  }
})

test('dark artwork has its own grayscale palette instead of inverted light fills', () => {
  const lightColors = new Set(mark.themes.light.paths.map(part => part.fill))
  assert.deepEqual([...lightColors].sort(), ['#000000', '#ffffff'])
  const darkColors = new Set(mark.themes.dark.paths.map(part => part.fill))
  assert(darkColors.size > lightColors.size)
  for (const color of darkColors) {
    assert.equal(color.slice(1, 3), color.slice(3, 5))
    assert.equal(color.slice(3, 5), color.slice(5, 7))
  }
  assert.notEqual(renderMarkSvg(mark, 'dark'), renderMarkSvg(mark, 'light'))
})

test('invalid or image-based paint cannot enter the vector export', () => {
  for (const mutate of [
    value => { delete value.themes.dark },
    value => { value.viewBox = '0 0 NaN 0' },
    value => { value.themes.dark.paths[0].d = 'M0 0"/><image href="data:image/png;base64,x"/>' },
    value => { value.themes.light.paths[0].fill = 'url(image.png)' },
    value => { value.themes.dark.paths[0].opacity = NaN },
    value => { value.themes.light.outline.pathIndex = -1 },
  ]) {
    const invalid = structuredClone(mark)
    mutate(invalid)
    assert.throws(() => validateMark(invalid))
  }
  assert.throws(() => renderMarkSvg(mark, 'inverse'), /Unknown mark theme/)
})
