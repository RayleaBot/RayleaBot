import { expect, test } from '@playwright/test'

test('secondary plugin consumers reach later pages without silently loading the whole catalog', async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true } })
  const plugins = ['首批插件', '第二插件', '远端插件'].map((name, index) => ({
    id: `pagination-${index}`, name, role: 'community', state: 'running', version: '1.0.0',
    source: { root: 'plugins/installed', verified: false }, trust: { level: 'third_party' },
    commands: [{ id: `command-${index}`, name: `${name}指令`, effective_names: [`command-${index}`], description: `${name}命令说明`, usage: `command-${index}`, permission: 'everyone', trigger: { type: 'exact', names: [`command-${index}`] } }],
    command_groups: [], command_conflicts: [], help: {},
  }))
  const queries: string[] = []
  await page.route('**/api/plugins**', async route => {
    const url = new URL(route.request().url())
    if (url.pathname === '/api/plugins') {
      queries.push(url.search)
      const query = url.searchParams.get('query') ?? ''
      const matches = plugins.filter(plugin => `${plugin.id} ${plugin.name}`.includes(query))
      const offset = Number(url.searchParams.get('cursor') ?? 0)
      const items = matches.slice(offset, offset + 2)
      await route.fulfill({ json: { items, total: matches.length, ...(offset + items.length < matches.length ? { next_cursor: String(offset + items.length) } : {}) } })
      return
    }
    const plugin = plugins.find(item => url.pathname === `/api/plugins/${item.id}`)
    if (plugin) { await route.fulfill({ json: { plugin: { ...plugin, permissions: {}, webhooks: [] } } }); return }
    await route.continue()
  })
  await page.goto('/login')
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()

  await page.goto('/commands')
  const commands = page.locator('.commands-section-card')
  await expect(commands.getByRole('table').getByText('首批插件命令说明')).toBeVisible()
  await expect(commands.getByText('远端插件命令说明')).toHaveCount(0)
  expect(queries.some(query => query.includes('cursor='))).toBe(false)
  await commands.getByRole('button', { name: '加载更多', exact: true }).click()
  await expect(commands.getByRole('table').getByText('远端插件命令说明')).toBeVisible()

  await page.goto('/menu-center')
  await expect(page.getByText('预览包含已加载的运行中插件；加载更多可补全总菜单。')).toBeVisible()
  await page.getByRole('tab', { name: '插件菜单预览', exact: true }).click()
  await page.getByRole('button', { name: '预览插件', exact: true }).click()
  const picker = page.getByRole('dialog', { name: '预览插件', exact: true })
  await picker.getByRole('textbox', { name: '搜索插件名称或 ID' }).fill('远端')
  await picker.getByRole('button', { name: '远端插件（pagination-2）', exact: true }).click()
  await expect(page.getByRole('button', { name: '预览插件', exact: true })).toContainText('远端插件')

  await page.goto('/logs')
  await page.getByRole('button', { name: '更多筛选', exact: true }).click()
  await page.locator('.log-advanced-filters__panel .plugin-picker__trigger').click()
  const logPicker = page.getByRole('dialog', { name: '选择插件', exact: true })
  await logPicker.getByRole('textbox', { name: '搜索插件名称或 ID' }).fill('远端')
  await logPicker.getByRole('checkbox', { name: '远端插件（pagination-2）', exact: true }).check()
  await page.keyboard.press('Escape')
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: '应用筛选', exact: true }).click()
  await expect(page).toHaveURL(/plugin_id=pagination-2/)
})
