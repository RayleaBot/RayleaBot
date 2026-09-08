import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true } })
  await page.goto('/login')
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
})

test('saving an existing account without a cookie preserves its credential', async ({ page }) => {
  await page.goto('/third-party-accounts')
  const card = page.locator('.account-card').filter({ hasText: '测试账号昵称' }).first()
  await card.getByRole('button', { name: '编辑', exact: true }).click()
  await expect(card.getByRole('textbox', { name: 'CK', exact: true })).toHaveValue('')
  await card.getByRole('textbox', { name: '备注', exact: true }).fill('更改显示名称')
  const saved = page.waitForRequest(request => request.method() === 'PUT' && request.url().endsWith('/api/third-party/accounts/bilibili/primary'))
  await card.getByRole('button', { name: '保存', exact: true }).click()
  expect((await saved).postDataJSON()).toEqual({ label: '更改显示名称', enabled: true })
  await expect(card).not.toHaveClass(/account-card--editing/)
  await expect(card).toContainText('已配置')
})

test('account deletion preserves exit context and returns focus after cancel or success', async ({ page }) => {
  await page.goto('/third-party-accounts')
  const card = page.locator('.account-card').filter({ hasText: '测试账号昵称' }).first()
  const trigger = card.getByRole('button', { name: '删除', exact: true })
  const deletions: string[] = []
  page.on('request', request => { if (request.method() === 'DELETE' && request.url().includes('/api/third-party/accounts/')) deletions.push(request.url()) })
  await trigger.click()
  const dialog = page.getByRole('alertdialog')
  await expect(dialog).toContainText('测试账号昵称')
  const result = page.evaluate(() => new Promise<{ retained: boolean; centered: boolean; frames: number }>(resolve => {
    let retained = true; let centered = true; let frames = 0
    const started = performance.now()
    function sample() {
      const element = document.querySelector<HTMLElement>('[data-slot=app-dialog]')
      if (element) {
        const opacity = Number(getComputedStyle(element).opacity)
        if (opacity > 0.01 && opacity < 0.95) {
          frames++
          retained &&= Boolean(element.textContent?.includes('测试账号昵称'))
          const rect = element.getBoundingClientRect()
          centered &&= Math.abs(rect.x + rect.width / 2 - innerWidth / 2) <= 1 && Math.abs(rect.y + rect.height / 2 - innerHeight / 2) <= 1
        }
      }
      if (performance.now() - started > 500) resolve({ retained, centered, frames })
      else requestAnimationFrame(sample)
    }
    requestAnimationFrame(sample)
  }))
  await dialog.getByRole('button', { name: '取消', exact: true }).click()
  expect(await result).toMatchObject({ retained: true, centered: true })
  expect((await result).frames).toBeGreaterThan(0)
  await expect(trigger).toBeFocused()
  expect(deletions).toEqual([])
  await trigger.click()
  await dialog.getByRole('button', { name: '删除账号', exact: true }).click()
  await expect(card).toHaveCount(0)
  await expect(page.getByRole('button', { name: '添加 Bilibili CK' }).first()).toBeFocused()
  expect(deletions).toHaveLength(1)
})

test('selection labels follow asynchronously loaded options', async ({ page }) => {
  let release!: () => void
  const gate = new Promise<void>(resolve => { release = resolve })
  await page.route('**/api/plugin-store/sources', async route => { await gate; await route.continue() })
  await page.goto('/plugins/store')
  const source = page.getByRole('combobox').first()
  await expect(source).toHaveText('official')
  release()
  await expect(source).toHaveText('RayleaBot 官方插件')
})
