import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true } })
  await page.goto('/login')
  await page.getByLabel('管理员密钥', { exact: true }).fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
})

test('plugin catalog restores updated icons and exposes entries after the first page', async ({ page }) => {
  let repaired = false
  await page.route('https://icons.example.test/**', async route => {
    const broken = route.request().url().endsWith('old.svg')
    await route.fulfill({ status: broken ? 404 : 200, contentType: 'image/svg+xml', body: broken ? 'missing image' : '<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40"><rect width="40" height="40" fill="green"/></svg>' })
  })
  await page.route('**/api/plugin-store/plugins?*', async route => {
    const response = await route.fetch()
    const original = await response.json()
    const cursor = Number(new URL(route.request().url()).searchParams.get('cursor') || 0)
    const items = Array.from({ length: cursor ? 1 : 100 }, (_, index) => ({ ...original.items[0], id: `fixture-${cursor + index}`, name: `插件 ${cursor + index}`, icon_url: cursor + index === 0 ? `https://icons.example.test/${repaired ? 'new' : 'old'}.svg` : undefined }))
    await route.fulfill({ response, json: { ...original, items, total: 101, next_cursor: cursor ? '' : '100' } })
  })
  await page.goto('/plugins/store')
  const cards = page.locator('.store-plugin-card')
  await expect(cards).toHaveCount(100)
  await expect(cards.first().locator('.plugin-icon .raylea-mark')).toBeVisible()
  repaired = true
  await page.getByTestId('plugin-store-refresh').click()
  await expect(cards.first().locator('.plugin-icon img')).toHaveAttribute('src', /new\.svg$/)
  await page.getByRole('button', { name: '加载更多插件', exact: true }).click()
  await expect(cards).toHaveCount(101)
  await expect(page.getByRole('heading', { name: '插件 100', exact: true })).toBeVisible()
})

test('scheduler icons and log plugin names use the installed plugin identity', async ({ page }) => {
  await page.goto('/scheduler')
  const identity = page.locator('.scheduler-cell-plugin-task').first()
  await expect(identity.locator('.plugin-name')).toHaveText('Weather')
  await expect(identity.locator('.plugin-icon img')).toHaveAttribute('src', '/api/plugins/weather/icon')
  await page.goto('/logs')
  await expect(page.locator('.logs-row__sub[title="weather"]').first()).toHaveText('Weather')
})
