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
const subscriptionHtml = fs.readFileSync(path.join(pluginRoot, 'web', 'index.html'), 'utf8')
const cookiesHtml = fs.readFileSync(path.join(pluginRoot, 'web', 'cookies.html'), 'utf8')
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
  tokens: [{
    id: 'bilibili-primary',
    platform: 'bilibili',
    label: '主 CK',
    secret_key: 'bili.primary',
    enabled: true,
  }],
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

function createPage(settings = richSettings, options = {}) {
  const page = options.page || 'subscriptions'
  const dom = new JSDOM(page === 'cookies' ? cookiesHtml : subscriptionHtml, {
    runScripts: 'outside-only',
    url: `https://rayleabot.local/plugin-ui/raylea.subscription-hub/web/${page === 'cookies' ? 'cookies.html' : 'index.html'}`,
  })
  const messages = []
  const secrets = options.secrets || { 'bili.primary': 'SESSDATA=old; bili_jct=old' }
  dom.window.parent = {
    postMessage(message) {
      messages.push(message)
    },
  }
  dom.window.confirm = () => true
  dom.window.eval(script)
  dom.window.dispatchEvent(new dom.window.MessageEvent('message', {
    data: {
      version: '1',
      source: 'management_host',
      type: 'host.init',
      payload: {
        title: page === 'cookies' ? 'CK 设置' : '订阅设置',
        plugin: { description: '订阅中心' },
        default_config: defaultSettings,
        settings,
        secrets,
      },
    },
  }))
  return { dom, messages }
}

function saveMessage(messages) {
  return messages.findLast((message) => message.type === 'settings.save')
}

function secretsMessage(messages) {
  return messages.findLast((message) => message.type === 'secrets.save')
}

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

test('renders Bilibili avatars, source names, and subscriber IDs', () => {
  const { dom } = createPage()
  const document = dom.window.document
  const cards = document.querySelectorAll('.subscription-card')

  assert.equal(cards.length, 2)
  assert.equal(document.querySelector('#cookie-list'), null)
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
  assert.equal(secretsMessage(messages), undefined)
  assert.deepEqual(plain(values.tokens), richSettings.tokens)
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

test('renders CK page and saves token remarks through settings and secrets', () => {
  const { dom, messages } = createPage(richSettings, { page: 'cookies' })
  const document = dom.window.document

  assert.equal(document.querySelector('#subscription-list'), null)
  assert.equal(document.querySelectorAll('.cookie-row').length, 1)
  assert.equal(document.querySelector('.cookie-platform').textContent, 'Bilibili')
  assert.equal(document.querySelector('#cookie-label-0').value, '主 CK')
  assert.equal(document.querySelector('#cookie-secret-value-0').value, 'SESSDATA=old; bili_jct=old')
  assert.equal(document.querySelector('#cookie-id-0'), null)
  assert.equal(document.querySelector('#cookie-secret-key-0'), null)

  document.querySelector('#cookie-label-0').value = '主账号'
  document.querySelector('#cookie-label-0').dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  document.querySelector('#cookie-secret-value-0').value = 'SESSDATA=new; bili_jct=new'
  document.querySelector('#cookie-secret-value-0').dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  document.querySelector('#save-button').click()

  const values = saveMessage(messages).payload.values
  assert.deepEqual(plain(values.tokens), [{
    id: 'bilibili-primary',
    platform: 'bilibili',
    label: '主账号',
    secret_key: 'bili.primary',
    enabled: true,
  }])
  assert.equal(values.tokens[0].secret_value, undefined)
  assert.deepEqual(plain(secretsMessage(messages).payload.values), {
    'bili.primary': 'SESSDATA=new; bili_jct=new',
  })
})

test('ignores CK plaintext carried by settings tokens', () => {
  const settings = {
    ...richSettings,
    tokens: [{
      ...richSettings.tokens[0],
      secret_value: 'SESSDATA=settings; bili_jct=settings',
    }],
  }
  const { dom } = createPage(settings, { page: 'cookies', secrets: {} })
  const document = dom.window.document

  assert.equal(document.querySelector('#cookie-secret-value-0').value, '')
  assert.deepEqual(plain(dom.window.__subscriptionHubSettingsPage.getDraft().tokens[0]), {
    id: 'bilibili-primary',
    platform: 'bilibili',
    label: '主 CK',
    secret_key: 'bili.primary',
    enabled: true,
    secret_value: '',
    show_secret: false,
  })
})

test('adds and deletes CK entries with platform and deleted secret keys', () => {
  const { dom, messages } = createPage(richSettings, { page: 'cookies' })
  const document = dom.window.document

  document.querySelector('#add-cookie-button').click()
  assert.equal(document.querySelectorAll('.cookie-row').length, 2)
  assert.equal(document.querySelectorAll('.cookie-platform')[1].textContent, 'Bilibili')
  assert.equal(document.querySelector('#cookie-id-1'), null)
  assert.equal(document.querySelector('#cookie-secret-key-1'), null)

  document.querySelector('#cookie-secret-value-1').value = 'SESSDATA=backup; bili_jct=backup'
  document.querySelector('#cookie-secret-value-1').dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  document.querySelectorAll('.cookie-row .button--danger')[0].click()
  document.querySelector('#save-button').click()

  const values = saveMessage(messages).payload.values
  assert.deepEqual(plain(values.tokens), [{
    id: 'bilibili-cookie-2',
    platform: 'bilibili',
    label: '备用 CK 2',
    secret_key: 'bili.cookie_2',
    enabled: true,
  }])
  assert.deepEqual(plain(secretsMessage(messages).payload.values), {
    'bili.cookie_2': 'SESSDATA=backup; bili_jct=backup',
  })
  assert.deepEqual(plain(secretsMessage(messages).payload.deleted_keys), ['bili.primary'])
})
