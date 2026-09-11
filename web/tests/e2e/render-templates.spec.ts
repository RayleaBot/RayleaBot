import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true } })
  await page.goto('/login')
  await page.getByLabel('管理员密钥', { exact: true }).fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
})

test('template catalog uses names, searches owners, and separates preview data from details', async ({ page }) => {
  await page.goto('/render/templates')
  await expect(page).toHaveURL(/\/render\/templates\/help.menu$/)
  await expect(page.getByRole('heading', { name: '帮助菜单', exact: true })).toBeVisible()
  await expect(page.locator('.template-catalog')).toContainText('运势')
  await expect(page.locator('.template-catalog')).toContainText('天气卡片')
  await expect(page.locator('.template-nav-item', { hasText: 'plugin.raylea' })).toHaveCount(0)
  await page.getByRole('searchbox', { name: '搜索插件或模板' }).fill('运势')
  await expect(page.locator('.template-nav-item')).toHaveCount(2)
  await page.getByRole('searchbox', { name: '搜索插件或模板' }).fill('没有这样的模板')
  await expect(page.getByText('没有匹配的模板', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '清除搜索', exact: true }).click()
  await page.getByRole('tab', { name: '模板说明', exact: true }).click()
  await page.getByText('查看标识与版本', { exact: true }).click()
  await expect(page.locator('.template-info-list')).toContainText('help.menu')
  await page.getByRole('tab', { name: '示例数据', exact: true }).click()
  await page.getByLabel('输入数据 JSON').fill('{"title":"自定义示例"}')
  await page.getByRole('button', { name: '恢复示例', exact: true }).click()
  await expect(page.getByLabel('输入数据 JSON')).toHaveValue(/帮助菜单/)
})

test('refreshing the catalog removes obsolete selections and their cached details', async ({ page }) => {
  await page.goto('/render/templates/help.menu')
  await expect(page.getByRole('heading', { name: '帮助菜单', exact: true })).toBeVisible()
  await page.route(/\/api\/system\/render\/templates(?:\?.*)?$/, async route => {
    const response = await route.fetch()
    const body = await response.json()
    const items = body.items.filter((item: { id: string }) => item.id !== 'help.menu')
    await route.fulfill({ response, json: { ...body, items, total: items.length, next_cursor: null } })
  })
  await page.route('**/api/system/render/templates/help.menu', route => route.fulfill({ status: 404, json: {
    error: { code: 'platform.resource_not_found', message: 'fixture template removed', message_key: 'errors.platform.resource_not_found', request_id: 'fixture-template-removed' },
  } }))
  await page.getByRole('button', { name: '刷新模板目录', exact: true }).click()
  await expect(page.locator('.template-nav-item[title="help.menu"]')).toHaveCount(0)
  await expect(page.getByText('原模板已不可用，已切换到当前模板。', { exact: true })).toBeVisible()
  await expect(page).not.toHaveURL(/\/help\.menu$/)
  const selected = page.locator('.template-nav-item[aria-current="page"]')
  await expect(selected).toBeVisible()
  await expect(page.getByTestId('render-template-preview-frame')).toHaveAttribute('data-template-id', (await selected.getAttribute('title'))!)
  await expect(page.getByRole('heading', { name: '帮助菜单', exact: true })).toHaveCount(0)
})

test('catalog refresh loads updated example data without a template source change', async ({ page }) => {
  await page.goto('/render/templates/help.menu')
  await expect(page.getByRole('heading', { name: '帮助菜单', exact: true })).toBeVisible()
  await page.route('**/api/system/render/templates/help.menu', async route => {
    const response = await route.fetch()
    const body = await response.json()
    body.template.preview_data_json = { title: '更新后的帮助示例' }
    await route.fulfill({ response, json: body })
  })
  const refreshedDetail = page.waitForResponse(response => response.url().endsWith('/api/system/render/templates/help.menu'))
  await page.getByRole('button', { name: '刷新模板目录', exact: true }).click()
  await refreshedDetail
  await page.getByRole('tab', { name: '示例数据', exact: true }).click()
  await page.getByRole('button', { name: '恢复示例', exact: true }).click()
  await expect(page.getByLabel('输入数据 JSON')).toHaveValue(/更新后的帮助示例/)
})

test('mobile template browsing and data editing fit the viewport', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' })
  await page.goto('/render/templates/fortune.card')
  await expect(page).toHaveURL(/\/render\/templates\/help.menu$/)
  await page.getByRole('button', { name: '选择模板', exact: true }).click()
  await page.getByRole('searchbox', { name: '搜索插件或模板' }).fill('今日运势')
  await page.getByRole('button', { name: '今日运势', exact: true }).click()
  await expect(page.getByRole('heading', { name: '今日运势', exact: true })).toBeVisible()
  await page.getByRole('tab', { name: '示例数据', exact: true }).click()
  await expect(page.getByLabel('输入数据 JSON')).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390)
})
