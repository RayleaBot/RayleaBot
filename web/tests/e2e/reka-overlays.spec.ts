import { expect, test, type Page } from '@playwright/test'

test.beforeEach(async ({ page, request }) => {
  await request.post('http://127.0.0.1:4010/__test/reset', { data: { initialized: true } })
  await page.goto('/login')
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
})

function sampleDialog(page: Page, duration = 650) {
  return page.evaluate((milliseconds) => new Promise<Array<{ height: number; center: number; opacity: number; focused: boolean; matrixRows: number }>>((resolve) => {
    const rows: Array<{ height: number; center: number; opacity: number; focused: boolean; matrixRows: number }> = []
    const start = performance.now()
    function sample(now: number) {
      const dialog = document.querySelector<HTMLElement>('[data-slot=app-dialog][role=dialog]')
      if (dialog) {
        const rect = dialog.getBoundingClientRect()
        rows.push({
          height: rect.height,
          center: Math.max(Math.abs(rect.x + rect.width / 2 - innerWidth / 2), Math.abs(rect.y + rect.height / 2 - innerHeight / 2)),
          opacity: Number(getComputedStyle(dialog).opacity),
          focused: dialog.contains(document.activeElement),
          matrixRows: dialog.querySelectorAll('.protocol-compatibility-table tbody tr').length,
        })
      }
      if (now - start > milliseconds) resolve(rows)
      else requestAnimationFrame(sample)
    }
    requestAnimationFrame(sample)
  }), duration)
}

test('Reka dialog animates content height and retains focus through its exit', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 960 })
  await page.goto('/protocols')
  await page.getByTestId('adapter-add').click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toHaveCSS('opacity', '1')
  const resize = sampleDialog(page)
  await page.getByTestId('adapter-select-qqofficial').click()
  const frames = await resize
  expect(Math.max(...frames.map((frame) => frame.center))).toBeLessThanOrEqual(1)
  expect(new Set(frames.map((frame) => Math.round(frame.height))).size).toBeGreaterThan(3)
  await expect(page.getByLabel('AppID', { exact: true })).toBeFocused()

  const closing = sampleDialog(page)
  await page.getByRole('button', { name: '取消', exact: true }).click()
  const exitFrames = await closing
  expect(exitFrames.some((frame) => frame.opacity > 0 && frame.opacity < 0.95)).toBe(true)
  expect(exitFrames.filter((frame) => frame.opacity > 0.01).every((frame) => frame.focused)).toBe(true)
  expect(Math.max(...exitFrames.map((frame) => frame.center))).toBeLessThanOrEqual(1)
  await expect(page.locator('[data-slot=app-dialog]')).toHaveCount(0)
  await expect(page.locator('.app-dialog-overlay')).toHaveCount(0)
  await expect(page.getByTestId('adapter-add')).toBeFocused()
  await expect(page.locator('body')).not.toHaveCSS('pointer-events', 'none')
})

test('Reka overlays preserve multiselect, nested cancellation and history reopening', async ({ page }) => {
  await page.goto('/protocols')
  await page.getByTestId('adapter-add').click()
  await page.goBack()
  await page.goForward()
  await expect(page.getByRole('dialog')).toHaveCount(1)
  await page.getByTestId('adapter-select-qqofficial').click()
  await page.getByLabel('AppID', { exact: true }).fill('12345')
  await page.getByLabel('接收消息', { exact: true }).click()
  const secondOption = page.getByRole('option', { name: '频道 @ 消息', exact: true })
  await secondOption.click()
  await expect(secondOption).toHaveAttribute('aria-selected', 'true')
  await expect(page.getByRole('option', { name: '群聊与单聊消息', exact: true })).toHaveAttribute('aria-selected', 'true')
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: '取消', exact: true }).click()
  await expect(page.getByRole('button', { name: '继续编辑', exact: true })).toBeFocused()
  await page.getByRole('button', { name: '继续编辑', exact: true }).click()
  await expect(page.getByRole('alertdialog')).toHaveCount(0)
  await expect(page.getByRole('button', { name: '取消', exact: true })).toBeFocused()
  await expect(page.getByLabel('AppID', { exact: true })).toHaveValue('12345')
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: '放弃修改', exact: true }).click()
  await expect(page.locator('[data-slot=app-dialog]')).toHaveCount(0)
  await expect(page.getByTestId('adapter-add')).toBeFocused()
})

test('compatibility rows remain visible until the dialog has finished exiting', async ({ page }) => {
  await page.goto('/protocols')
  const trigger = page.getByRole('button', { name: '兼容矩阵', exact: true })
  await trigger.click()
  await expect(page.getByRole('table')).toBeVisible()
  await expect(page.getByRole('dialog')).toHaveCSS('opacity', '1')
  const rowCount = await page.locator('.protocol-compatibility-table tbody tr').count()
  const closing = sampleDialog(page)
  await page.getByRole('button', { name: '关闭弹窗' }).click()
  const frames = await closing
  const intermediate = frames.filter((frame) => frame.opacity > 0.01 && frame.opacity < 0.95)
  expect(intermediate.length).toBeGreaterThan(1)
  expect(intermediate.every((frame) => frame.matrixRows === rowCount)).toBe(true)
  await expect(page.getByRole('table')).toHaveCount(0)
  await expect(trigger).toBeFocused()
})

test('Reka form remains operable in low-height, reduced-motion and forced-colors views', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 460 })
  await page.emulateMedia({ reducedMotion: 'reduce', forcedColors: 'active' })
  await page.goto('/protocols?adapter=onebot11')
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect(page.getByTestId('adapter-save')).toBeInViewport()
  await expect(page.getByRole('button', { name: '关闭弹窗' })).toBeInViewport()
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1)
  await page.getByLabel('连接方式').click()
  await page.getByRole('option', { name: '自定义组合', exact: true }).click()
  await page.getByRole('checkbox', { name: '反向 WebSocket', exact: true }).check()
  await expect(page.getByRole('checkbox', { name: '反向 WebSocket', exact: true })).toBeChecked()
  await page.getByRole('checkbox', { name: 'HTTP API', exact: true }).click()
  await expect(page.getByRole('checkbox', { name: 'HTTP API', exact: true })).toBeChecked()
  await page.getByRole('button', { name: '关闭弹窗' }).click()
  await page.getByRole('button', { name: '放弃修改', exact: true }).click()
  await expect(page.locator('.app-dialog-overlay')).toHaveCount(0)
})

test('component showcase supports keyboard selection and contains no production navigation entry', async ({ page }) => {
  await page.goto('/__dev/components')
  await expect(page.getByRole('heading', { name: '组件与交互状态' })).toBeVisible()
  await page.getByLabel('接入协议', { exact: true }).focus()
  await page.keyboard.press('ArrowDown')
  await page.getByRole('option', { name: 'QQ 官方机器人', exact: true }).focus()
  await page.keyboard.press('Enter')
  await expect(page.getByLabel('接入协议', { exact: true })).toContainText('QQ 官方机器人')
  await page.getByRole('button', { name: '打开弹窗', exact: true }).click()
  await page.keyboard.press('Tab')
  await expect.poll(() => page.getByRole('dialog').evaluate((dialog) => dialog.contains(document.activeElement))).toBe(true)
  await page.getByRole('button', { name: '完成', exact: true }).click()
  await expect(page.getByRole('button', { name: '打开弹窗', exact: true })).toBeFocused()
})
