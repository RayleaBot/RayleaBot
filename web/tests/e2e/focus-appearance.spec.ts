import { expect, test, type Locator } from '@playwright/test'

async function focusStyle(control: Locator) {
  await control.focus()
  await expect(control).toBeFocused()
  return control.evaluate(async element => {
    await Promise.all(element.getAnimations().map(animation => animation.finished.catch(() => {})))
    const style = getComputedStyle(element)
    return { visible: element.matches(':focus-visible'), outline: style.outlineStyle, offset: parseFloat(style.outlineOffset), shadow: style.boxShadow, border: style.borderColor }
  })
}

test('shared controls keep keyboard focus inside their boundary across themes', async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true } })
  await page.goto('/login')
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  await page.goto('/__dev/components')
  await page.keyboard.press('Tab')

  for (const colorScheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme })
    await expect(page.locator('html')).toHaveAttribute('data-theme', colorScheme)
    for (const name of ['连接名称', '访问令牌', '接入协议', '连接超时（秒）', '备注', '无效地址']) {
      const style = await focusStyle(page.getByLabel(name, { exact: true }))
      expect(style.visible).toBe(true)
      expect(style.outline).toBe('none')
      const shadows = style.shadow.split(/,(?![^(]*\))/)
      expect(shadows.every(shadow => shadow.includes('inset'))).toBe(true)
    }
    for (const control of [page.getByRole('button', { name: '次要操作', exact: true }), page.getByRole('checkbox'), page.getByRole('switch')]) {
      const style = await focusStyle(control)
      expect(style.visible).toBe(true)
      expect(style.outline).toBe('solid')
      expect(style.offset).toBeLessThan(0)
      expect(style.shadow).toBe('none')
    }
    const tags = page.getByLabel('自由输入列表', { exact: true })
    await tags.focus()
    await expect(tags).toHaveCSS('outline-style', 'none')
    await expect(page.locator('.app-tags-input')).toHaveCSS('box-shadow', /inset/)
  }

  await page.emulateMedia({ forcedColors: 'active' })
  const forcedStyle = await focusStyle(page.getByLabel('连接名称', { exact: true }))
  expect(forcedStyle.outline).toBe('solid')
  expect(forcedStyle.offset).toBeLessThan(0)
  expect(forcedStyle.shadow).toBe('none')
})
