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
  enabled: true,
  poll_cron: '*/5 * * * *',
  poll_timeout_seconds: 12,
  dynamic_time_range_seconds: 7200,
  max_updates_per_poll: 6,
  tokens: [],
  subscriptions: [],
}

const richSettings = {
  ...defaultSettings,
  subscriptions: [{
    id: 'bilibili-123456-group-5050',
    platform: 'bilibili',
    uid: '123456',
    name: '测试 UP',
    avatar_url: 'https://i0.hdslb.com/face.jpg',
    target_type: 'group',
    target_id: '5050',
    target_name: '测试群',
    services: ['live'],
    subscribers: [{
      id: '10001',
      nickname: '测试号',
      group_nickname: '群名片',
      title: '头衔',
      role: 'admin',
      role_label: '管理员',
      avatar_url: 'https://q1.qlogo.cn/g?b=qq&nk=10001&s=100',
    }],
    enabled: true,
  }, {
    id: 'bilibili-654321-private-2626',
    platform: 'bilibili',
    uid: '654321',
    name: '无头像 UP',
    target_type: 'private',
    target_id: '2626',
    target_name: '测试用户',
    services: ['video'],
    subscribers: [{ id: '10002', nickname: '测试号' }],
    enabled: true,
  }],
}

function createPage(settings = richSettings) {
  const dom = new JSDOM(html, {
    runScripts: 'outside-only',
    url: 'https://rayleabot.local/plugin-ui/raylea.subscription-hub/web/index.html',
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
        title: '订阅设置',
        plugin: { description: '订阅中心' },
        default_config: defaultSettings,
        settings,
        secrets: {},
      },
    },
  }))
  return { dom, messages }
}

function saveMessage(messages) {
  return messages.findLast((message) => message.type === 'settings.save')
}

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

test('renders Bilibili avatars, source names, and subscriber IDs', () => {
  const { dom } = createPage()
  const document = dom.window.document
  const cards = document.querySelectorAll('.subscription-card')

  assert.equal(cards.length, 2)
  assert.equal(cards[0].querySelector('.subscription-avatar img').getAttribute('src'), 'https://i0.hdslb.com/face.jpg')
  assert.equal(cards[0].querySelector('.subscription-avatar__fallback').textContent, '测')
  assert.match(cards[0].querySelector('.subscription-card__meta').textContent, /群聊 测试群 5050/)
  assert.equal(cards[0].querySelector('.subscription-subscribers__names').textContent, '群名片（10001）')
  assert.equal(cards[1].querySelector('.subscription-avatar img'), null)
  assert.equal(cards[1].querySelector('.subscription-avatar__fallback').textContent, '无')
  assert.match(cards[1].querySelector('.subscription-card__meta').textContent, /私聊 测试用户 2626/)
  assert.equal(cards[1].querySelector('.subscription-subscribers__names').textContent, '测试号（10002）')
})

test('saves subscription display metadata without dropping fields', () => {
  const { dom, messages } = createPage()
  const document = dom.window.document

  document.querySelector('#poll-timeout-input').value = '13'
  document.querySelector('#poll-timeout-input').dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  document.querySelector('#save-button').click()

  const values = saveMessage(messages).payload.values
  assert.equal(values.poll_timeout_seconds, 13)
  assert.deepEqual(plain(values.subscriptions[0]), richSettings.subscriptions[0])
  assert.equal(values.subscriptions[1].target_name, '测试用户')
})

test('search matches source names and subscriber IDs', () => {
  const { dom } = createPage()
  const document = dom.window.document

  document.querySelector('#subscription-search-input').value = '测试用户'
  document.querySelector('#subscription-search-input').dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  assert.equal(document.querySelectorAll('.subscription-card').length, 1)
  assert.match(document.querySelector('.subscription-card__meta').textContent, /私聊 测试用户 2626/)

  document.querySelector('#subscription-search-input').value = '10001'
  document.querySelector('#subscription-search-input').dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  assert.equal(document.querySelectorAll('.subscription-card').length, 1)
  assert.equal(document.querySelector('.subscription-subscribers__names').textContent, '群名片（10001）')
})
