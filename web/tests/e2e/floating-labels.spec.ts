import { expect, test, type Locator, type Page } from '@playwright/test'

async function labelOffset(control: Locator) {
  return control.evaluate(async element => {
    const label = element.closest('.app-field')!.querySelector<HTMLElement>('.app-field__label')!
    getComputedStyle(label).transform
    await Promise.all(label.getAnimations().map(animation => animation.finished.catch(() => {})))
    return label.getBoundingClientRect().top - element.getBoundingClientRect().top
  })
}

async function login(page: Page) {
  await page.getByLabel('管理员密钥', { exact: true }).fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
}

test.beforeEach(async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true } })
  await page.goto('/login')
})

test('login labels float for focus, entered values and external autofill without losing validation', async ({ page }) => {
  const identifier = page.getByLabel('管理员账号', { exact: true })
  await identifier.fill('')
  await identifier.blur()
  const resting = await labelOffset(identifier)
  expect(resting).toBeGreaterThan(12)
  await identifier.focus()
  expect(await labelOffset(identifier)).toBeLessThan(resting - 5)
  await identifier.fill('admin')
  await identifier.blur()
  expect(await labelOffset(identifier)).toBeLessThan(resting - 5)
  const secret = page.getByLabel('管理员密钥', { exact: true })
  await secret.evaluate((element: HTMLInputElement) => { element.value = 'fixture-autofilled' })
  expect(await labelOffset(secret)).toBeLessThan(resting - 5)
  await secret.fill('')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(secret).toBeFocused()
  await expect(secret).toHaveAttribute('aria-invalid', 'true')
  await expect(page.locator('.app-field__description[role=alert]')).toBeVisible()
  await secret.fill('fixture-only-secret')
  await page.getByRole('button', { name: '显示密钥', exact: true }).click()
  await expect(secret).toHaveAttribute('type', 'text')
  expect(await labelOffset(secret)).toBeLessThan(resting - 5)
  await login(page)
})

test('mobile account labels fit prefix icons and preserve the account update form', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' })
  await login(page)
  await page.getByRole('button', { name: '打开菜单', exact: true }).click()
  await page.getByTestId('mobile-account').click()
  await page.getByRole('menuitem', { name: '修改账户', exact: true }).click()
  const dialog = page.getByTestId('account-credentials-dialog')
  const current = dialog.getByLabel('当前密码', { exact: true })
  await expect(current).toBeFocused()
  expect(await labelOffset(current)).toBeLessThan(12)
  const field = dialog.locator('.app-field').filter({ has: page.locator('[data-account-current-password]') })
  const prefix = await field.locator('.app-input-prefix').boundingBox()
  const label = await field.locator('.app-field__label').boundingBox()
  expect(label!.x).toBeGreaterThanOrEqual(prefix!.x + prefix!.width - 1)
  await dialog.getByRole('button', { name: '同时修改用户名' }).click()
  await expect(dialog.getByLabel('新用户名（可选）', { exact: true })).toBeVisible()
  expect(await dialog.evaluate(element => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1)
  await dialog.getByRole('button', { name: '取消', exact: true }).click()
  await expect(page.getByTestId('mobile-account')).toBeFocused()
})

test('shared floating labels handle empty selection, zero and multiline input in both themes', async ({ page }) => {
  await login(page)
  await page.goto('/__dev/components')
  for (const theme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' })
    const number = page.getByLabel('可留空数值', { exact: true })
    await number.fill('')
    await number.blur()
    const resting = await labelOffset(number)
    expect(resting).toBeGreaterThan(12)
    await number.fill('0')
    await number.blur()
    expect(await labelOffset(number)).toBeLessThan(resting - 5)
    const note = page.getByLabel('备注', { exact: true })
    await note.fill('')
    await note.blur()
    expect(await labelOffset(note)).toBeGreaterThan(12)
    await note.fill('第一行\n第二行')
    await note.blur()
    expect(await labelOffset(note)).toBeLessThan(12)
  }
  const select = page.getByLabel('待选协议', { exact: true })
  expect(await labelOffset(select)).toBeGreaterThan(12)
  await select.click()
  expect(await labelOffset(select)).toBeLessThan(12)
  await page.getByRole('option', { name: 'OneBot11', exact: true }).click()
  await select.blur()
  expect(await labelOffset(select)).toBeLessThan(12)
  await expect(page.getByLabel('只读连接标识', { exact: true })).toBeDisabled()
  await expect(page.locator('.app-field--floating').filter({ has: page.getByLabel('自由输入列表', { exact: true }) })).toHaveCount(0)
})
