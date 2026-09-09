import { expect, test } from '@playwright/test'

test.use({
  timezoneId: 'America/Los_Angeles',
  // Native scrollbar interaction needs the same visible controls as a normal browser.
  launchOptions: { ignoreDefaultArgs: ['--hide-scrollbars'] },
})

test.beforeEach(async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true } })
  await page.goto('/login')
  await page.getByLabel('管理员密钥', { exact: true }).fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
})

test('timezone picker covers Windows offsets, supports the keyboard, and preserves the active zone until restart', async ({ page }) => {
  await page.clock.setFixedTime(new Date('2026-07-15T12:00:00Z'))
  await page.goto('/config')
  await page.getByRole('tab', { name: '账号与调度' }).click()
  const trigger = page.getByRole('button', { name: '时区', exact: true })
  await expect(trigger).toContainText('UTC+08:00')
  await expect(trigger).toContainText('上海')
  await trigger.focus()
  await page.keyboard.press('ArrowDown')
  const search = page.getByRole('combobox', { name: '搜索时区' })
  await expect(search).toBeFocused()
  await search.fill('Antarctica')
  await expect(page.getByRole('option')).toHaveCount(0)
  for (const [offset, id] of [['UTC-12', 'Etc/GMT+12'], ['UTC+14', 'Pacific/Kiritimati'], ['UTC+4:30', 'Asia/Kabul'], ['UTC+8:45', 'Australia/Eucla'], ['UTC+12:45', 'Pacific/Chatham']]) {
    await search.fill(offset)
    await expect(page.locator(`[role=option][title="${id}"]`)).toBeVisible()
  }
  await search.fill('UTC+5:45')
  await expect(page.getByRole('option')).toHaveCount(1)
  await expect(page.getByRole('option').first()).toContainText('UTC+05:45')
  await search.fill('Europe/Paris')
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('Enter')
  await expect(trigger).toContainText('巴黎')
  await expect(trigger).toBeFocused()
  const saved = page.waitForResponse(response => response.request().method() === 'PUT' && response.url().endsWith('/api/config'))
  await page.getByRole('button', { name: '保存更改', exact: true }).click()
  const response = await saved
  expect(response.request().postDataJSON().scheduler.timezone).toBe('Europe/Paris')
  const result = await response.json()
  expect(result.effective_timezone).toBe('Asia/Shanghai')
  expect(result.apply_effects.restart_required_fields).toContain('scheduler.timezone')
})

test('the timezone list has a visible draggable scrollbar and shrinks for a single search result', async ({ page }) => {
  await page.goto('/config')
  await page.getByRole('tab', { name: '账号与调度' }).click()
  await page.getByRole('button', { name: '时区', exact: true }).click()
  const viewport = page.locator('.timezone-select__viewport')
  const initial = await viewport.boundingBox()
  expect(initial).not.toBeNull()
  const scrollbar = await viewport.evaluate(element => ({
    visible: getComputedStyle(element, '::-webkit-scrollbar').display,
    gutter: element.getBoundingClientRect().width - element.clientWidth,
  }))
  expect(scrollbar.visible).toBe('block')
  expect(scrollbar.gutter).toBeGreaterThan(0)
  await page.mouse.move(initial!.x + 60, initial!.y + 60)
  await page.mouse.wheel(0, -10_000)
  await expect.poll(() => viewport.evaluate(element => element.scrollTop)).toBe(0)
  await page.mouse.move(initial!.x + initial!.width - 5, initial!.y + 18)
  await page.mouse.down()
  await page.mouse.move(initial!.x + initial!.width - 5, initial!.y + 160, { steps: 8 })
  await page.mouse.up()
  await expect.poll(() => viewport.evaluate(element => element.scrollTop)).toBeGreaterThan(200)
  await page.getByRole('combobox', { name: '搜索时区' }).fill('Europe/Paris')
  await expect(page.getByRole('option')).toHaveCount(1)
  await expect.poll(async () => (await viewport.boundingBox())!.height).toBeLessThan(100)
})

test('timezone transitions preserve intermediate sizes, exit cleanly, and tolerate rapid reopening', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'no-preference' })
  await page.goto('/config')
  await page.getByRole('tab', { name: '账号与调度' }).click()
  const trigger = page.getByRole('button', { name: '时区', exact: true })
  await trigger.click()
  const panel = page.locator('.timezone-select__content')
  const viewport = page.locator('.timezone-select__viewport')
  const search = page.getByRole('combobox', { name: '搜索时区' })
  await expect.poll(() => panel.evaluate(element => getComputedStyle(element).opacity)).toBe('1')
  const resizing = viewport.evaluate(element => new Promise<number[]>(resolve => {
    const heights: number[] = []
    const started = performance.now()
    const sample = () => {
      heights.push(element.clientHeight)
      if (performance.now() - started < 450) requestAnimationFrame(sample)
      else resolve(heights)
    }
    requestAnimationFrame(sample)
  }))
  await search.fill('Europe/Paris')
  const heights = await resizing
  expect(heights.some(height => height > 60 && height < 340)).toBe(true)
  await expect.poll(() => viewport.evaluate(element => element.clientHeight)).toBe(52)
  const exiting = panel.evaluate(element => new Promise<number[]>(resolve => {
    const opacities: number[] = []
    const started = performance.now()
    const sample = () => {
      if (element.isConnected) opacities.push(Number(getComputedStyle(element).opacity))
      if (performance.now() - started < 350) requestAnimationFrame(sample)
      else resolve(opacities)
    }
    requestAnimationFrame(sample)
  }))
  await page.keyboard.press('Escape')
  expect((await exiting).some(opacity => opacity > 0 && opacity < 1)).toBe(true)
  await expect(panel).toHaveCount(0)
  await expect(trigger).toBeFocused()
  await trigger.click()
  await page.keyboard.press('Escape')
  await trigger.click()
  await expect(search).toBeFocused()
  await expect(panel).not.toHaveAttribute('inert')
  await expect.poll(() => panel.evaluate(element => getComputedStyle(element).opacity)).toBe('1')
  await expect(search).toHaveValue('')
  await search.fill('Asia/Kabul')
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('Enter')
  await expect(trigger).toContainText('喀布尔')
  await expect(panel).toHaveCount(0)
})

test('reduced motion keeps timezone selection and resizing immediate', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/config')
  await page.getByRole('tab', { name: '账号与调度' }).click()
  const trigger = page.getByRole('button', { name: '时区', exact: true })
  await trigger.click()
  const panel = page.locator('.timezone-select__content')
  await expect(page.getByRole('combobox', { name: '搜索时区' })).toBeFocused()
  expect(await panel.evaluate(element => getComputedStyle(element).transform)).toBe('none')
  await page.getByRole('combobox', { name: '搜索时区' }).fill('Asia/Kabul')
  await expect.poll(() => page.locator('.timezone-select__viewport').evaluate(element => element.clientHeight)).toBe(52)
  await page.keyboard.press('Escape')
  await expect(panel).toHaveCount(0)
  await expect(trigger).toBeFocused()
})

test('an existing uncommon timezone remains selected without creating an edit', async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true, timezone: 'Asia/Katmandu' } })
  await page.goto('/login')
  await page.getByLabel('管理员密钥', { exact: true }).fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  await page.goto('/config')
  await page.getByRole('tab', { name: '账号与调度' }).click()
  await page.getByRole('button', { name: '时区', exact: true }).click()
  const retained = page.locator('[role=option][title="Asia/Katmandu"]')
  await expect(retained).toContainText('当前配置')
  await expect(retained).toHaveAttribute('data-state', 'checked')
  await page.keyboard.press('Escape')
  await expect(page.getByRole('button', { name: '保存更改', exact: true })).toBeDisabled()
})

test('history calendar inputs and outgoing queries use the server timezone instead of the browser timezone', async ({ page }) => {
  await page.goto('/logs/history?start_at=2026-01-15T20%3A30%3A00Z&end_at=2026-01-15T21%3A30%3A00Z')
  const start = page.getByLabel('开始时间', { exact: true })
  const end = page.getByLabel('结束时间', { exact: true })
  await expect(start).toHaveValue('2026-01-16T04:30')
  await expect(end).toHaveValue('2026-01-16T05:30')
  await start.fill('2026-01-16T06:30')
  await end.fill('2026-01-16T07:30')
  const requested = page.waitForRequest(request => request.url().includes('/api/logs?') && request.url().includes('scope=history'))
  await page.getByRole('button', { name: '应用筛选', exact: true }).click()
  const url = new URL((await requested).url())
  expect(url.searchParams.get('start_at')).toBe('2026-01-15T22:30:00Z')
  expect(url.searchParams.get('end_at')).toBe('2026-01-15T23:30:00Z')
})
