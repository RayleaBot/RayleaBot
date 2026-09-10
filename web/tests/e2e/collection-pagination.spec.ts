import { expect, test, type Page } from '@playwright/test'

const backend = 'http://127.0.0.1:4010'

async function login(page: Page) {
  await page.goto('/login')
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
}

// Small valid pages exercise controls without depending on a large test DOM.
function pageOf<T>(all: T[], url: URL, text: (item: T) => string) {
  const query = (url.searchParams.get('query') ?? '').toLowerCase()
  const matches = all.filter(item => text(item).toLowerCase().includes(query))
  const cursor = Number(url.searchParams.get('cursor') ?? 0)
  return { items: matches.slice(cursor, cursor + 2), total: matches.length, ...(cursor + 2 < matches.length ? { next_cursor: String(cursor + 2) } : {}) }
}

test.beforeEach(async ({ request }) => { await request.post(`${backend}/__test/reset`, { data: { initialized: true } }) })

test('plugin catalog loads more on demand and remote search finds an unloaded plugin', async ({ page }) => {
  await login(page)
  const fixture = (await (await page.request.get('/api/plugins')).json()).items[0]
  const all = Array.from({ length: 5 }, (_, i) => ({ ...fixture, id: `paging-${i}`, name: i === 4 ? '后页目标插件' : `分页插件 ${i}` }))
  const cursors: string[] = []
  await page.route('**/api/plugins?*', route => {
    const url = new URL(route.request().url())
    cursors.push(url.searchParams.get('cursor') ?? '')
    return route.fulfill({ json: pageOf(all, url, item => `${item.id} ${item.name}`) })
  })
  await page.route('**/api/plugins', route => route.fulfill({ json: pageOf(all, new URL(route.request().url()), item => `${item.id} ${item.name}`) }))
  await page.goto('/plugins')
  await expect(page.locator('.plugin-grid-card')).toHaveCount(2)
  expect(cursors.every(cursor => !cursor)).toBe(true)
  await page.getByRole('main').getByRole('button', { name: '加载更多', exact: true }).click()
  await expect(page.locator('.plugin-grid-card')).toHaveCount(4)
  expect(cursors).toContain('2')
  await page.locator('.filter-search input').fill('后页目标')
  await expect(page.locator('.plugin-grid-card')).toHaveCount(1)
  await expect(page.locator('.plugin-grid-card')).toContainText('后页目标插件')
})

test('filtered empty whitelist uses the unfiltered rule count for its enabled warning', async ({ page }) => {
  await login(page)
  const all = Array.from({ length: 5 }, (_, i) => ({ entry_type: 'group', target_id: `9000${i}`, reason: `规则 ${i}`, created_at: '2026-09-10T00:00:00Z', scope: { kind: 'global', source_protocol: 'onebot11', source_adapter: '', bot_id: '' } }))
  await page.route('**/api/governance/whitelist*', route => {
    if (route.request().method() !== 'GET') return route.fallback()
    const result = pageOf(all, new URL(route.request().url()), item => `${item.target_id} ${item.reason}`)
    return route.fulfill({ json: { ...result, items: undefined, enabled: true, entry_count: all.length, user_entries: [], group_entries: result.items } })
  })
  await page.goto('/access-lists')
  const card = page.getByTestId('access-lists-whitelist-card')
  await expect(card.getByRole('status')).toContainText('已加载 2 / 5')
  await card.getByRole('button', { name: '加载更多', exact: true }).click()
  await expect(card.getByRole('status')).toContainText('已加载 4 / 5')
  await page.getByTestId('whitelist-search-input').fill('不存在')
  await expect(card.getByRole('status')).toContainText('已加载 0 / 0')
  await expect(card.locator('.access-lists-card-header__count')).toHaveText('5')
  await expect(page.getByText('白名单已启用且当前为空', { exact: false })).toHaveCount(0)
})

test('scheduler searches the full collection and refreshes only explicitly loaded pages after a log event', async ({ page, request }) => {
  await login(page)
  const fixture = (await (await page.request.get('/api/system/scheduler/jobs')).json()).items[0]
  const all = Array.from({ length: 5 }, (_, i) => ({ ...fixture, job_id: `paging-job-${i}`, task_name: i === 4 ? '后页任务' : `任务 ${i}` }))
  let requests = 0
  await page.route('**/api/system/scheduler/jobs*', route => {
    if (route.request().method() !== 'GET') return route.fallback()
    requests++
    return route.fulfill({ json: pageOf(all, new URL(route.request().url()), item => `${item.job_id} ${item.task_name}`) })
  })
  await page.goto('/scheduler')
  const main = page.getByRole('main')
  await expect(main.getByRole('status')).toContainText('已加载 2 / 5')
  const search = main.locator('input').first()
  await search.fill('后页任务')
  await expect(main.getByRole('status')).toContainText('已加载 1 / 1')
  await search.fill('')
  await expect(main.getByRole('status')).toContainText('已加载 2 / 5')
  await main.getByRole('button', { name: '加载更多', exact: true }).click()
  await expect(main.getByRole('status')).toContainText('已加载 4 / 5')
  const beforeRefresh = requests
  all[0].task_name = '事件后更新'
  await request.post(`${backend}/__test/push-log`, { data: { source: 'scheduler', log_id: 'paging-invalidation', message: 'fixture refresh' } })
  await expect(main.getByText('事件后更新', { exact: true }).first()).toBeVisible()
  await expect(main.getByRole('status')).toContainText('已加载 4 / 5')
  expect(requests - beforeRefresh).toBe(2)
})

test('account and source management can search an item that has never been loaded', async ({ page }) => {
  await login(page)
  const account = (await (await page.request.get('/api/third-party/accounts')).json()).items[0]
  const accounts = Array.from({ length: 5 }, (_, i) => ({ ...account, account_id: `paging-account-${i}`, label: i === 4 ? '后页账号' : `账号 ${i}`, profile: null }))
  await page.route('**/api/third-party/accounts*', route => {
    if (new URL(route.request().url()).pathname !== '/api/third-party/accounts') return route.fallback()
    return route.fulfill({ json: pageOf(accounts, new URL(route.request().url()), item => `${item.account_id} ${item.label}`) })
  })
  await page.goto('/third-party-accounts')
  await page.getByLabel('搜索账号', { exact: true }).fill('后页账号')
  await expect(page.getByRole('main').getByRole('status')).toContainText('已加载 1 / 1')
  await expect(page.getByRole('main').getByText('后页账号', { exact: true }).first()).toBeVisible()

  const source = (await (await page.request.get('/api/plugin-store/sources')).json()).items[0]
  const sources = Array.from({ length: 5 }, (_, i) => ({ ...source, id: i === 0 ? 'official' : `source-${i}`, name: i === 4 ? '后页来源' : `来源 ${i}`, official: i === 0 }))
  await page.route('**/api/plugin-store/sources*', route => {
    if (new URL(route.request().url()).pathname !== '/api/plugin-store/sources' || route.request().method() !== 'GET') return route.fallback()
    return route.fulfill({ json: pageOf(sources, new URL(route.request().url()), item => `${item.id} ${item.name}`) })
  })
  await page.goto('/plugins/store')
  await page.getByTestId('plugin-store-sources').click()
  await page.getByLabel('搜索插件来源', { exact: true }).fill('后页来源')
  await expect(page.getByRole('dialog').getByRole('status')).toContainText('已加载 1 / 1')
  await expect(page.getByRole('dialog').locator('.source-row')).toContainText('后页来源')
})

test('a template deep link remains selected outside the loaded or searched catalog page', async ({ page }) => {
  await login(page)
  const first = (await (await page.request.get('/api/system/render/templates')).json()).items[0]
  const detail = (await (await page.request.get(`/api/system/render/templates/${first.id}`)).json()).template
  const all = Array.from({ length: 5 }, (_, i) => ({ ...first, id: `paging-template-${i}`, name: i === 4 ? '后页模板' : `模板 ${i}` }))
  await page.route('**/api/system/render/templates**', route => {
    const url = new URL(route.request().url())
    if (url.pathname === '/api/system/render/templates') return route.fulfill({ json: pageOf(all, url, item => `${item.id} ${item.name}`) })
    const id = url.pathname.split('/')[5]
    const summary = all.find(item => item.id === id)
    if (!summary) return route.fallback()
    if (url.pathname.endsWith('/preview-html')) return route.fulfill({ json: { template_id: id, source_digest: 'a'.repeat(64), width: detail.width, height: detail.height, html: '<!doctype html><html><body>preview fixture</body></html>' } })
    return route.fulfill({ json: { template: { ...detail, ...summary } } })
  })
  await page.goto('/render/templates/paging-template-4')
  await expect(page.locator('.template-workspace__heading h2')).toHaveText('后页模板')
  await expect(page.locator('.template-catalog').getByRole('status')).toContainText('已加载 2 / 5')
  await page.locator('.template-catalog input').fill('后页模板')
  await expect(page.locator('.template-catalog').getByRole('status')).toContainText('已加载 1 / 1')
  await page.locator('.template-catalog input').fill('')
  await expect(page.locator('.template-catalog').getByRole('status')).toContainText('已加载 2 / 5')
  await expect(page).toHaveURL(/paging-template-4$/)
  await expect(page.locator('.template-workspace__heading h2')).toHaveText('后页模板')
})
