import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true } })
  await page.goto('/login')
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
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
