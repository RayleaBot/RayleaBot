import { expect, test } from '@playwright/test'
import { respondWith } from './fixture-responses'

test.beforeEach(async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true } })
  await page.goto('/login')
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
})

test('canceling installation keeps the inspection visible through exit and performs no install', async ({ page }) => {
  let installations = 0
  page.on('request', request => {
    if (request.method() === 'POST' && new URL(request.url()).pathname === '/api/plugins/install') installations++
  })
  await page.goto('/plugins')
  await page.locator('.plugins-toolbar').getByRole('button', { name: '安装插件', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '安装插件' })
  await dialog.getByRole('textbox').fill('C:/plugins/weather.zip')
  await dialog.getByRole('button', { name: '检查插件包' }).click()
  await expect(dialog.getByText('Weather Package（example.weather-package）')).toBeVisible()
  const closing = page.evaluate(() => new Promise<number[]>((resolve) => {
    const counts: number[] = []
    const started = performance.now()
    const sample = () => {
      const dialog = document.querySelector<HTMLElement>('[data-slot=app-dialog]')
      if (dialog) {
        const opacity = Number(getComputedStyle(dialog).opacity)
        if (opacity > 0.01 && opacity < 0.95) counts.push(dialog.querySelectorAll('.app-detail-item').length)
      }
      if (performance.now() - started > 500) resolve(counts)
      else requestAnimationFrame(sample)
    }
    requestAnimationFrame(sample)
  }))
  await dialog.getByRole('button', { name: '取消', exact: true }).click()
  const counts = await closing
  expect(counts.length).toBeGreaterThan(0)
  expect(counts.every(count => count > 0)).toBe(true)
  expect(installations).toBe(0)
  await expect(dialog).toHaveCount(0)
  await page.locator('.plugins-toolbar').getByRole('button', { name: '安装插件', exact: true }).click()
  await expect(dialog.getByRole('textbox')).toHaveValue('')
  await expect(dialog.getByRole('checkbox')).toHaveCount(0)
})

test('plugin source creation and nested removal confirmations preserve cancellation', async ({ page }) => {
  const sources = await (await page.request.get('/api/plugin-store/sources')).json()
  const created = { id: 'fixture-source', name: '测试目录', url: 'https://plugins.example/reka-catalog.json', official: false, cached: true, entry_count: 0, refreshed_at: '2026-09-07T00:00:00Z' }
  await respondWith(page.request, [
    { method: 'POST', path: '/api/plugin-store/sources', status: 201, body: created, after: [{ path: '/api/plugin-store/sources', body: { ...sources, items: [...sources.items, created], total: sources.total + 1 } }] },
    { method: 'DELETE', path: '/api/plugin-store/sources/fixture-source', status: 204, after: [{ path: '/api/plugin-store/sources', body: sources }] },
  ])
  await page.goto('/plugins/store')
  const managerTrigger = page.getByTestId('plugin-store-sources')
  await managerTrigger.click()
  const manager = page.getByRole('dialog', { name: '管理插件源', exact: true })
  await manager.getByRole('button', { name: '添加插件源' }).click()
  const editor = page.getByRole('dialog').filter({ has: page.getByRole('textbox') })
  await editor.getByRole('textbox').nth(0).fill('测试目录')
  await editor.getByRole('textbox').nth(1).fill('https://plugins.example/reka-catalog.json')
  await editor.getByRole('button', { name: '保存', exact: true }).click()
  await expect(editor).toHaveCount(0)
  await expect(managerTrigger).toBeFocused()
  await managerTrigger.click()
  const row = manager.locator('.source-row').filter({ hasText: '测试目录' })
  await expect(row).toBeVisible()
  await row.getByRole('button', { name: '删除', exact: true }).click()
  const confirmation = page.getByRole('alertdialog')
  await confirmation.getByRole('button', { name: '取消', exact: true }).click()
  await expect(confirmation).toHaveCount(0)
  await expect(row).toBeVisible()
  await row.getByRole('button', { name: '删除', exact: true }).click()
  await confirmation.getByRole('button', { name: '删除', exact: true }).click()
  await expect(row).toHaveCount(0)
})

test('menu previews retain their iframe documents across tab changes', async ({ page }) => {
  await page.goto('/menu-center')
  const root = page.getByTestId('menu-center-root-preview').locator('iframe')
  await expect(root).toBeVisible()
  const frame = await (await root.elementHandle())!.contentFrame()
  await frame!.evaluate(() => window.document.fonts.ready)
  const documentHandle = await frame!.evaluateHandle(() => window.document)
  await page.getByRole('tab', { name: '插件菜单预览' }).click()
  await expect(root).toBeHidden()
  await page.getByRole('tab', { name: '总菜单预览' }).click()
  await expect(root).toBeVisible()
  expect(await documentHandle.evaluate(node => node === window.document)).toBe(true)
})

test('selection preserves primitive types and tags allow a literal comma', async ({ page }) => {
  await page.goto('/__dev/components')
  for (const [label, expected] of [['开启（布尔）', 'true'], ['文本 true', '"true"'], ['数字 1', '1'], ['文本 1', '"1"']]) {
    await page.getByLabel('保留原始类型', { exact: true }).click()
    await page.getByRole('option', { name: label, exact: true }).click()
    await expect(page.getByTestId('showcase-typed-value')).toHaveText(expected)
  }
  const input = page.getByLabel('自由输入列表', { exact: true })
  await input.fill(',')
  await input.press('Enter')
  await expect(page.getByTestId('showcase-tags-value')).toHaveText('["help",","]')
  await page.getByRole('button', { name: '删除 ,', exact: true }).click()
  await expect(page.getByTestId('showcase-tags-value')).toHaveText('["help"]')
  await page.getByLabel('可留空数值', { exact: true }).fill('')
  await expect(page.getByTestId('showcase-optional-number')).toHaveText('null')
})

test('unknown plugin filters remain visible and can be cleared', async ({ page }) => {
  await page.goto('/commands?plugin_id=unknown-plugin')
  const selector = page.getByRole('button', { name: '按插件筛选', exact: true })
  await expect(selector).toContainText('unknown-plugin')
  await expect(page.getByRole('alert')).toBeVisible()
  await selector.click()
  await page.getByRole('dialog', { name: '选择插件', exact: true }).getByRole('button', { name: '清除选择' }).click()
  await page.keyboard.press('Escape')
  await expect(page).toHaveURL(/\/commands$/)
  await expect(page.locator('.commands-data-table')).toContainText('weather')
  await expect(page.locator('.commands-data-table')).toContainText('hello')
})
