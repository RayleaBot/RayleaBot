import assert from 'node:assert/strict'
import fs from 'node:fs'
import { createRequire } from 'node:module'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const pluginRoot = path.resolve(__dirname, '..')
const require = createRequire(path.join(pluginRoot, '..', '..', '..', 'web', 'package.json'))
const { JSDOM } = require('jsdom')
const html = fs.readFileSync(path.join(pluginRoot, 'web', 'index.html'), 'utf8')
const script = fs.readFileSync(path.join(pluginRoot, 'web', 'app.js'), 'utf8')

const defaultSettings = {
  trigger_commands: ['我的运势'],
  stats_trigger_commands: ['运势统计'],
  timezone: 'Asia/Shanghai',
  fortunes: [
    { name: '大吉', stars: '★★★★★★★', sign: '云开见月', explanation: '适合推进重要事项。' },
    { name: '吉', stars: '★★★★★★☆', sign: '春风入户', explanation: '整体顺利。' },
  ],
  special_dates: [],
  good_actions: ['整理计划'],
  bad_actions: ['熬夜'],
}

function createPage(settings = defaultSettings) {
  const dom = new JSDOM(html, {
    runScripts: 'outside-only',
    url: 'https://rayleabot.local/plugin-ui/raylea.fortune/web/index.html',
  })
  const messages = []
  dom.window.parent = {
    postMessage(message) {
      messages.push(message)
    },
  }
  dom.window.eval(script)
  dom.window.dispatchEvent(new dom.window.MessageEvent('message', {
    data: {
      version: '1',
      source: 'management_host',
      type: 'host.init',
      payload: {
        title: '运势设置',
        plugin: { description: '每日运势' },
        default_config: defaultSettings,
        settings,
      },
    },
  }))
  return { dom, messages }
}

function createUninitializedPage() {
  const dom = new JSDOM(html, {
    runScripts: 'outside-only',
    url: 'https://rayleabot.local/plugin-ui/raylea.fortune/web/index.html',
  })
  const messages = []
  dom.window.parent = {
    postMessage(message) {
      messages.push(message)
    },
  }
  dom.window.eval(script)
  return { dom, messages }
}

function pressEnter(window, element) {
  element.dispatchEvent(new window.KeyboardEvent('keydown', {
    key: 'Enter',
    bubbles: true,
    cancelable: true,
  }))
}

function saveMessage(messages) {
  return messages.findLast((message) => message.type === 'settings.save')
}

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

test('renders structured editors from host init', () => {
  const { dom } = createPage()
  const document = dom.window.document

  assert.equal(document.querySelectorAll('#fortune-trigger-list .chip').length, 1)
  assert.equal(document.querySelectorAll('#stats-trigger-list .chip').length, 1)
  assert.equal(document.querySelector('#timezone-input').value, 'Asia/Shanghai')
  assert.equal(document.querySelectorAll('#fortune-table-body tr').length, 2)
  assert.equal(document.querySelector('#fortune-count').textContent, '2')
  assert.equal(document.querySelector('#validation-summary').textContent, '可保存')
})

test('retries page ready until host init arrives', async () => {
  const { dom, messages } = createUninitializedPage()

  await new Promise((resolve) => dom.window.setTimeout(resolve, 1100))
  assert.ok(messages.filter((message) => message.type === 'page.ready').length >= 2)

  dom.window.dispatchEvent(new dom.window.MessageEvent('message', {
    data: {
      version: '1',
      source: 'management_host',
      type: 'host.init',
      payload: {
        title: '运势设置',
        plugin: { description: '每日运势' },
        default_config: defaultSettings,
        settings: defaultSettings,
      },
    },
  }))
  const countAfterInit = messages.filter((message) => message.type === 'page.ready').length
  await new Promise((resolve) => dom.window.setTimeout(resolve, 650))

  assert.equal(messages.filter((message) => message.type === 'page.ready').length, countAfterInit)
})

test('adds chip values with Enter and saves all trigger and action arrays', () => {
  const { dom, messages } = createPage()
  const document = dom.window.document

  document.querySelector('#fortune-trigger-input').value = '今日运势'
  pressEnter(dom.window, document.querySelector('#fortune-trigger-input'))
  document.querySelector('#stats-trigger-input').value = '查看运势统计'
  pressEnter(dom.window, document.querySelector('#stats-trigger-input'))
  document.querySelector('#good-actions-input').value = '备份资料'
  pressEnter(dom.window, document.querySelector('#good-actions-input'))
  document.querySelector('#bad-actions-input').value = '冲动消费'
  pressEnter(dom.window, document.querySelector('#bad-actions-input'))

  document.querySelector('#save-button').click()
  const values = saveMessage(messages).payload.values

  assert.deepEqual(plain(values.trigger_commands), ['我的运势', '今日运势'])
  assert.deepEqual(plain(values.stats_trigger_commands), ['运势统计', '查看运势统计'])
  assert.deepEqual(plain(values.good_actions), ['整理计划', '备份资料'])
  assert.deepEqual(plain(values.bad_actions), ['熬夜', '冲动消费'])
})

test('keeps explicit empty chip lists when saving', () => {
  const { dom, messages } = createPage({
    ...defaultSettings,
    trigger_commands: [],
    stats_trigger_commands: [],
    good_actions: [],
    bad_actions: [],
  })
  const document = dom.window.document

  assert.equal(document.querySelectorAll('#fortune-trigger-list .chip').length, 0)
  assert.equal(document.querySelectorAll('#stats-trigger-list .chip').length, 0)
  assert.equal(document.querySelectorAll('#good-actions-list .chip').length, 0)
  assert.equal(document.querySelectorAll('#bad-actions-list .chip').length, 0)

  document.querySelector('#timezone-input').value = 'UTC+08:00'
  document.querySelector('#timezone-input').dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  document.querySelector('#save-button').click()
  const values = saveMessage(messages).payload.values

  assert.deepEqual(plain(values.trigger_commands), [])
  assert.deepEqual(plain(values.stats_trigger_commands), [])
  assert.deepEqual(plain(values.good_actions), [])
  assert.deepEqual(plain(values.bad_actions), [])
})

test('keeps multiple special fortunes for the same date in save payload', () => {
  const { dom, messages } = createPage({
    ...defaultSettings,
    special_dates: [
      { date: '05-04', fortune_name: '大吉' },
    ],
  })
  const document = dom.window.document

  document.querySelector('#add-special-date-button').click()
  const rows = document.querySelectorAll('#special-date-table-body tr')
  assert.equal(rows.length, 2)
  rows[1].querySelector('input').value = '05-04'
  rows[1].querySelector('input').dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  rows[1].querySelector('select').value = '吉'
  rows[1].querySelector('select').dispatchEvent(new dom.window.Event('change', { bubbles: true }))

  document.querySelector('#save-button').click()

  assert.deepEqual(plain(saveMessage(messages).payload.values.special_dates), [
    { date: '05-04', fortune_name: '大吉' },
    { date: '05-04', fortune_name: '吉' },
  ])
})

test('supports fortune table add duplicate and delete operations', () => {
  const { dom, messages } = createPage()
  const document = dom.window.document

  document.querySelector('#add-fortune-button').click()
  let rows = document.querySelectorAll('#fortune-table-body tr')
  assert.equal(rows.length, 3)

  rows[2].querySelector('textarea[aria-label="签文"]').value = '新增签文'
  rows[2].querySelector('textarea[aria-label="签文"]').dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  rows[2].querySelector('textarea[aria-label="解签"]').value = '新增解签'
  rows[2].querySelector('textarea[aria-label="解签"]').dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  rows[2].querySelector('button').click()
  rows = document.querySelectorAll('#fortune-table-body tr')
  assert.equal(rows.length, 4)

  rows[0].querySelector('.button--danger').click()
  rows = document.querySelectorAll('#fortune-table-body tr')
  assert.equal(rows.length, 3)

  document.querySelector('#save-button').click()
  const values = saveMessage(messages).payload.values
  assert.equal(values.fortunes.length, 3)
  assert.equal(values.fortunes.at(-1).sign, '新增签文')
})

test('blocks save when fortune and special date rows are invalid', () => {
  const { dom, messages } = createPage({
    ...defaultSettings,
    fortunes: [{ name: '大吉', stars: '★★★★★★★', sign: '', explanation: '缺少签文' }],
    special_dates: [{ date: 'bad-date', fortune_name: '不存在' }],
  })
  const document = dom.window.document

  assert.equal(document.querySelector('#save-button').disabled, true)
  document.querySelector('#save-button').click()

  assert.equal(saveMessage(messages), undefined)
  assert.match(document.querySelector('#validation-summary').textContent, /个问题/)
})

test('accepts preset and custom timezone values in save payload', () => {
  const { dom, messages } = createPage()
  const document = dom.window.document
  const timezoneInput = document.querySelector('#timezone-input')

  timezoneInput.value = 'UTC+08:00'
  timezoneInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  document.querySelector('#save-button').click()
  assert.equal(saveMessage(messages).payload.values.timezone, 'UTC+08:00')

  dom.window.dispatchEvent(new dom.window.MessageEvent('message', {
    data: {
      version: '1',
      source: 'management_host',
      type: 'settings.changed',
      payload: { values: saveMessage(messages).payload.values },
    },
  }))
  timezoneInput.value = 'America/New_York'
  timezoneInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  document.querySelector('#save-button').click()
  assert.equal(saveMessage(messages).payload.values.timezone, 'America/New_York')
})
