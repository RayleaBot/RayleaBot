import { expect, test, type Page } from '@playwright/test'

test.beforeEach(async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true } })
  await page.goto('/login')
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
})

function observeExit(page: Page, selector: string, content: string, centered = false) {
  return page.evaluate(({ selector, content, centered }) => new Promise<{ frames: number; retained: boolean; centered: boolean }>(resolve => {
    const result = { frames: 0, retained: true, centered: true }
    const started = performance.now()
    function sample() {
      const element = document.querySelector<HTMLElement>(selector)
      if (element) {
        const opacity = Number(getComputedStyle(element).opacity)
        if (opacity > 0.01 && opacity < 0.95) {
          result.frames++
          result.retained &&= Boolean(element.textContent?.includes(content))
          if (centered) {
            const rect = element.getBoundingClientRect()
            result.centered &&= Math.abs(rect.x + rect.width / 2 - innerWidth / 2) <= 1 && Math.abs(rect.y + rect.height / 2 - innerHeight / 2) <= 1
          }
        }
      }
      if (performance.now() - started > 650) resolve(result)
      else requestAnimationFrame(sample)
    }
    requestAnimationFrame(sample)
  }), { selector, content, centered })
}

test('scheduler details retain their context and center through exit, then restore keyboard focus', async ({ page }) => {
  await page.goto('/scheduler')
  const trigger = page.locator('.scheduler-data-table').getByRole('button', { name: '查看', exact: true }).first()
  await trigger.focus()
  await page.keyboard.press('Enter')
  const dialog = page.getByRole('dialog', { name: '定时任务详情' })
  await expect(dialog).toContainText('插件事件响应超时')
  await page.waitForTimeout(300)
  const exit = observeExit(page, '[data-slot=app-dialog]', '每日早报', true)
  await page.keyboard.press('Escape')
  expect(await exit).toMatchObject({ retained: true, centered: true })
  expect((await exit).frames).toBeGreaterThan(0)
  await expect(dialog).toHaveCount(0)
  await expect(trigger).toBeFocused()
  const triggered = page.waitForResponse(response => response.request().method() === 'POST' && response.url().endsWith('/scheduler/jobs/daily_report/trigger'))
  await page.locator('.scheduler-data-table').getByRole('button', { name: '立即执行', exact: true }).click()
  expect((await triggered).ok()).toBe(true)
})

test('desktop log details remain nonmodal, keep drag position and retain data during exit', async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1200 })
  await page.goto('/logs')
  const row = page.locator('.logs-row').filter({ hasText: 'req_adapter_ignored_0001' }).first()
  await row.click()
  const detail = page.getByTestId('management-log-detail-window')
  await expect(detail).toHaveAttribute('aria-modal', 'false')
  await expect(detail).toContainText('api response echo must be a non-empty string')
  await page.waitForTimeout(300)
  const original = await detail.boundingBox()
  const handle = await detail.locator('.log-detail-window__header').boundingBox()
  await page.mouse.move(handle!.x + 20, handle!.y + 20)
  await page.mouse.down()
  await page.mouse.move(handle!.x - 28, handle!.y + 20, { steps: 5 })
  await page.mouse.up()
  const moved = await detail.boundingBox()
  expect(moved!.x).toBeLessThan(original!.x - 10)
  expect(await page.locator('.app-dialog-overlay').count()).toBe(0)
  const exit = observeExit(page, '[data-testid=management-log-detail-window]', 'api response echo must be a non-empty string')
  await detail.getByRole('button', { name: '关闭详情', exact: true }).click()
  expect((await exit).retained).toBe(true)
  expect((await exit).frames).toBeGreaterThan(0)
  await expect(detail).toHaveCount(0)
  await expect(row).toBeFocused()
  await row.click()
  await expect(detail).toBeVisible()
  await page.waitForTimeout(300)
  expect(Math.abs((await detail.boundingBox())!.x - moved!.x)).toBeLessThan(2)
})

test('template preview stays inside its centered container when the viewport narrows', async ({ page }) => {
  await page.goto('/render/templates/help.menu')
  const frame = page.getByTestId('render-template-preview-frame')
  await expect(frame).toBeVisible()
  await frame.evaluate(element => { element.dataset.resizeWitness = 'retained-document' })
  for (const width of [1440, 390, 768]) {
    await page.setViewportSize({ width, height: 960 })
    await expect.poll(() => frame.evaluate(element => {
      const rect = element.getBoundingClientRect()
      const container = element.parentElement!
      const parent = container.getBoundingClientRect()
      const style = getComputedStyle(container)
      return Math.max(
        Math.abs(rect.x - parent.x - parseFloat(style.borderLeftWidth)),
        Math.abs(parent.right - parseFloat(style.borderRightWidth) - rect.right),
      )
    })).toBeLessThanOrEqual(1)
    await expect(frame).toHaveAttribute('data-resize-witness', 'retained-document')
  }
})

test('mobile log drawer retains details until it closes and restores the selected row', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/logs')
  const row = page.locator('.logs-row').filter({ hasText: 'req_adapter_ignored_0001' }).first()
  await row.click()
  const detail = page.locator('[data-slot=app-dialog][data-placement=right]')
  await expect(detail).toContainText('api response echo must be a non-empty string')
  await page.waitForTimeout(300)
  const exit = observeExit(page, '[data-slot=app-dialog]', 'api response echo must be a non-empty string')
  await detail.getByRole('button', { name: '关闭弹窗' }).click()
  expect((await exit).retained).toBe(true)
  expect((await exit).frames).toBeGreaterThan(0)
  await expect(detail).toHaveCount(0)
  await expect(row).toBeFocused()
  await expect(page.locator('.app-dialog-overlay')).toHaveCount(0)
})

test('advanced log filters clear protocol and unknown plugin values with keyboard focus intact', async ({ page }) => {
  await page.goto('/logs?protocol=onebot11&plugin_id=removed-plugin')
  const trigger = page.locator('.log-advanced-filters__trigger')
  await trigger.click()
  const panel = page.locator('.log-advanced-filters__panel')
  await panel.getByRole('combobox', { name: '协议', exact: true }).click()
  await page.getByRole('option', { name: '全部', exact: true }).click()
  const plugins = panel.getByRole('button', { name: '插件', exact: true })
  await expect(plugins).toContainText('removed-plugin')
  await plugins.click()
  const picker = page.getByRole('dialog', { name: '选择插件', exact: true })
  await picker.getByRole('button', { name: '清除选择' }).click()
  await page.keyboard.press('Escape')
  await expect(plugins).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(panel).toHaveCount(0)
  await expect(trigger).toBeFocused()
  await page.getByRole('button', { name: '应用筛选' }).click()
  await expect.poll(() => new URL(page.url()).searchParams.has('protocol')).toBe(false)
  expect(new URL(page.url()).searchParams.has('plugin_id')).toBe(false)
})
