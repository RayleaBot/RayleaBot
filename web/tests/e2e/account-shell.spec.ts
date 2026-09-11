import { expect, test, type Page } from '@playwright/test'

async function openAccount(page: Page, mobile = false) {
  if (mobile) await page.getByRole('button', { name: '打开菜单', exact: true }).click()
  await page.getByTestId(mobile ? 'mobile-account' : 'sidebar-account').click()
  await page.getByRole('menuitem', { name: '修改账户', exact: true }).click()
  const dialog = page.getByTestId('account-credentials-dialog')
  await expect(dialog.getByLabel('当前密码', { exact: true })).toBeFocused()
  return dialog
}

test.beforeEach(async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true } })
  await page.goto('/login')
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
})

test('shell keeps account and theme at the bottom and cancels the progressively disclosed form', async ({ page }) => {
  const header = page.getByTestId('app-header')
  await expect(header.getByTestId('theme-toggle')).toHaveCount(0)
  await expect(header.getByRole('button', { name: '账号', exact: true })).toHaveCount(0)
  await expect(page.getByTestId('header-settings-direct')).toHaveCount(0)
  await expect(page.getByTestId('header-fullscreen-direct')).toHaveCount(0)
  await expect(page.getByTestId('build-version')).toHaveText('开发版本')
  const footer = await page.getByTestId('sidebar-footer').boundingBox()
  expect(footer!.y + footer!.height).toBeCloseTo(await page.evaluate(() => innerHeight), 0)
  const dialog = await openAccount(page)
  await expect(dialog.getByLabel('新用户名（可选）', { exact: true })).toHaveCount(0)
  await dialog.getByLabel('当前密码', { exact: true }).fill('fixture-draft')
  await dialog.getByRole('button', { name: '同时修改用户名' }).click()
  await expect(dialog.getByLabel('新用户名（可选）', { exact: true })).toBeVisible()
  await dialog.getByRole('button', { name: '取消', exact: true }).click()
  await expect(dialog).toHaveCount(0)
  await expect(page.getByTestId('sidebar-account')).toBeFocused()
  await expect(page.locator('body')).not.toHaveCSS('pointer-events', 'none')
  await openAccount(page)
  await expect(dialog.getByLabel('当前密码', { exact: true })).toHaveValue('')
  await expect(dialog.getByLabel('新用户名（可选）', { exact: true })).toHaveCount(0)
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  await expect(page.getByTestId('sidebar-account')).toBeFocused()
})

test('mobile drawer retains account controls and focus in reduced motion', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.emulateMedia({ reducedMotion: 'reduce' })
  const dialog = await openAccount(page, true)
  await expect(dialog).toHaveCSS('opacity', '1')
  await dialog.getByRole('button', { name: '取消', exact: true }).click()
  await expect(dialog).toHaveCount(0)
  await expect(page.getByTestId('mobile-account')).toBeFocused()
  const accountBounds = await page.getByTestId('mobile-account').boundingBox()
  expect(accountBounds!.y).toBeGreaterThan(700)
  await page.getByTestId('mobile-theme-toggle').click()
  await page.getByRole('menuitem', { name: '暗色', exact: true }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(page.locator('html')).not.toHaveAttribute('data-view-transition-kind')
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390)
})

test('theme animation runs on its snapshot and releases it after repeated changes', async ({ page }) => {
  await page.evaluate(() => {
    const nativeAnimate = Element.prototype.animate
    const observed = window as Window & { themeAnimations?: string[] }
    observed.themeAnimations = []
    Element.prototype.animate = function (keyframes, options) {
      if (typeof options === 'object' && options?.pseudoElement) {
        observed.themeAnimations!.push(options.pseudoElement)
      }
      return nativeAnimate.call(this, keyframes, options)
    }
  })
  for (const [label, mode] of [['暗色', 'dark'], ['亮色', 'light']] as const) {
    await page.getByTestId('theme-toggle').click()
    await page.getByRole('menuitem', { name: label, exact: true }).click()
    await expect(page.locator('html')).toHaveAttribute('data-theme', mode)
    await expect(page.locator('html')).not.toHaveAttribute('data-view-transition-kind')
  }
  const animations = await page.evaluate(() => (window as Window & { themeAnimations?: string[] }).themeAnimations ?? [])
  expect(animations.filter(name => name === '::view-transition-new(root)').length).toBeGreaterThanOrEqual(2)
  await page.getByRole('button', { name: '折叠侧栏', exact: true }).click()
  await openAccount(page)
})
