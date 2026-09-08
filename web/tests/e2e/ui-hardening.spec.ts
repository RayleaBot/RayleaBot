import { expect, test } from '@playwright/test'

test.use({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true })

test.beforeEach(async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true } })
  await page.goto('/login')
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).tap()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
})

test('configuration guidance stays visible and associated on touch', async ({ page }) => {
  await page.goto('/config')
  const input = page.getByRole('textbox', { name: '监听地址', exact: true })
  const descriptionId = await input.getAttribute('aria-describedby')
  expect(descriptionId).toBeTruthy()
  const help = page.locator(`[id="${descriptionId}"]`)
  await expect(help).toBeVisible()
  await expect(help).toContainText('通配地址会被拒绝')
  await expect(page.getByText(/^脱敏字段：/)).toHaveCount(0)
})

test('touch can activate the reserved area around a compact switch', async ({ page }) => {
  await page.goto('/config')
  await page.getByRole('button', { name: /会话与高级访问/ }).tap()
  const control = page.getByRole('switch', { name: '自动续期', exact: true })
  await control.scrollIntoViewIfNeeded()
  const original = await control.getAttribute('aria-checked')
  const bounds = await control.boundingBox()
  await page.touchscreen.tap(bounds!.x + bounds!.width / 2, bounds!.y - 9)
  await expect(control).toHaveAttribute('aria-checked', original === 'true' ? 'false' : 'true')
  await page.touchscreen.tap(bounds!.x + bounds!.width / 2, bounds!.y + bounds!.height + 9)
  await expect(control).toHaveAttribute('aria-checked', original!)
})

test('credential instructions remain associated with the editable field', async ({ page }) => {
  await page.goto('/third-party-accounts')
  await page.getByRole('button', { name: '编辑', exact: true }).first().tap()
  const control = page.getByRole('textbox', { name: 'CK', exact: true })
  const descriptionId = await control.getAttribute('aria-describedby')
  expect(descriptionId).toBeTruthy()
  await expect(page.locator(`[id="${descriptionId}"]`)).toHaveText('留空时保留当前 CK。')
})

test('a missing browser resource can be prepared from its readiness issue', async ({ page }) => {
  const checks = page.locator('details.readiness-check-group')
  await expect(checks).not.toHaveAttribute('open')
  const prepare = page.getByTestId('readiness-prepare-runtime')
  const request = page.waitForRequest(request => request.method() === 'POST' && request.url().endsWith('/api/system/runtime/bootstrap'))
  await prepare.tap()
  expect((await request).postDataJSON()).toEqual({ resources: ['chromium'] })
})
