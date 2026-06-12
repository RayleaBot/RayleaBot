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
const script = fs.readFileSync(path.join(pluginRoot, 'web', 'app.js'), 'utf8')

const defaultSettings = {
  enabled: true,
  subscriptions: [],
}

const targetsPayload = {
  protocol: 'onebot11',
  available: true,
  groups: [
    { target_type: 'group', target_id: '5050', target_name: '测试群' },
    { target_type: 'group', target_id: '6060', target_name: '备用群' },
  ],
  private_users: [
    { target_type: 'private', target_id: '2626', nickname: '测试用户' },
  ],
  issues: [],
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
    target_name: '旧群名',
    services: ['live'],
    subscribers: [{ id: '10001', nickname: '旧昵称' }],
    enabled: true,
  }, {
    id: 'bilibili-123456-private-2626',
    platform: 'bilibili',
    uid: '123456',
    name: '测试 UP',
    avatar_url: 'https://i0.hdslb.com/face.jpg',
    target_type: 'private',
    target_id: '2626',
    target_name: '旧私聊名',
    services: ['video'],
    subscribers: [{ id: '10001', nickname: '旧昵称' }],
    enabled: true,
  }],
}

function dispatchHost(dom, type, payload, requestId = 'host-test') {
  dom.window.dispatchEvent(new dom.window.MessageEvent('message', {
    data: {
      version: '1',
      source: 'management_host',
      type,
      request_id: requestId,
      payload,
    },
  }))
}

function createPage(settings = richSettings) {
  const dom = new JSDOM(subscriptionHtml, {
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
  dispatchHost(dom, 'host.init', {
    title: '订阅设置',
    plugin: { description: '订阅中心' },
    default_config: defaultSettings,
    settings,
  })
  dispatchHost(dom, 'protocol.targets.changed', targetsPayload)
  return { dom, messages }
}

function saveMessage(messages) {
  return messages.findLast((message) => message.type === 'settings.save')
}

function lastMessage(messages, type) {
  return messages.findLast((message) => message.type === type)
}

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

test('groups subscriptions by Bilibili UID and hides raw maintenance fields', () => {
  const { dom } = createPage()
  const document = dom.window.document
  const rows = document.querySelectorAll('.subscription-row')

  assert.equal(rows.length, 1)
  assert.equal(document.querySelector('#raw-json-input'), null)
  assert.equal(document.querySelector('#subscription-editor-panel'), null)
  assert.match(rows[0].textContent, /测试 UP/)
  assert.match(rows[0].textContent, /群聊 测试群/)
  assert.match(rows[0].textContent, /私聊 测试用户/)
  assert.match(rows[0].textContent, /目标配置不同/)
})

test('target type switch keeps targets selected across group and private lists', () => {
  const { dom } = createPage()
  const document = dom.window.document
  const row = document.querySelector('.subscription-row')
  const privateButton = row.querySelector('button[data-action="target-mode"][data-mode="private"]')
  privateButton.click()

  const privateRow = document.querySelector('.subscription-row')
  const select = privateRow.querySelector('.target-select')
  assert.equal([...select.options].length, 1)
  assert.equal(select.options[0].selected, true)
  assert.match(privateRow.textContent, /群聊 测试群/)
  assert.match(privateRow.textContent, /私聊 测试用户/)

  privateRow.querySelector('button[data-action="target-mode"][data-mode="group"]').click()
  const groupRow = document.querySelector('.subscription-row')
  const groupSelect = groupRow.querySelector('.target-select')
  groupSelect.options[1].selected = true
  groupSelect.dispatchEvent(new dom.window.Event('change', { bubbles: true }))

  const updatedRow = document.querySelector('.subscription-row')
  assert.match(updatedRow.textContent, /群聊 测试群/)
  assert.match(updatedRow.textContent, /群聊 备用群/)
  assert.match(updatedRow.textContent, /私聊 测试用户/)
})

test('saving without subscriber IDs keeps system subscription and refreshes target names', () => {
  const { dom, messages } = createPage({
    enabled: true,
    subscriptions: [{
      id: 'bilibili-123456-group-5050',
      platform: 'bilibili',
      uid: '123456',
      name: '测试 UP',
      target_type: 'group',
      target_id: '5050',
      target_name: '旧群名',
      services: ['all'],
      subscribers: [],
      enabled: true,
    }],
  })
  const document = dom.window.document

  document.querySelector('#save-button').click()

  const values = saveMessage(messages).payload.values
  assert.equal(values.enabled, true)
  assert.equal(values.subscriptions.length, 1)
  assert.equal(values.subscriptions[0].target_name, '测试群')
  assert.deepEqual(plain(values.subscriptions[0].subscribers), [])
  assert.equal(lastMessage(messages, 'protocol.identities.resolve'), undefined)
})

test('saving subscriber IDs resolves display identity before settings save', () => {
  const { dom, messages } = createPage()
  const document = dom.window.document

  document.querySelector('#save-button').click()
  const resolveMessage = lastMessage(messages, 'protocol.identities.resolve')
  assert.ok(resolveMessage)
  assert.deepEqual(plain(resolveMessage.payload.items), [
    { target_type: 'group', target_id: '5050', user_id: '10001' },
    { target_type: 'private', target_id: '2626', user_id: '10001' },
  ])

  dispatchHost(dom, 'protocol.identities.resolved', {
    items: [
      {
        target_type: 'group',
        target_id: '5050',
        user_id: '10001',
        nickname: '测试号',
        group_nickname: '群名片',
        role: 'admin',
        role_label: '管理员',
        title: '头衔',
        avatar_url: 'https://q1.qlogo.cn/g?b=qq&nk=10001&s=640',
      },
      {
        target_type: 'private',
        target_id: '2626',
        user_id: '10001',
        nickname: '测试号',
        avatar_url: 'https://q1.qlogo.cn/g?b=qq&nk=10001&s=640',
      },
    ],
    issues: [],
  }, resolveMessage.request_id)

  const values = saveMessage(messages).payload.values
  assert.equal(values.subscriptions[0].target_name, '测试群')
  assert.equal(values.subscriptions[0].subscribers[0].group_nickname, '群名片')
  assert.equal(values.subscriptions[1].target_name, '测试用户')
  assert.equal(values.subscriptions[1].subscribers[0].nickname, '测试号')
})

test('new row must resolve Bilibili user before saving', () => {
  const { dom, messages } = createPage(defaultSettings)
  const document = dom.window.document

  document.querySelector('#add-subscription-button').click()
  assert.equal(document.querySelector('#save-button').disabled, true)

  const input = document.querySelector('.up-query-input')
  input.value = '洛天依'
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  document.querySelector('button[data-action="resolve-up"]').click()
  const resolveMessage = lastMessage(messages, 'bilibili.user.resolve')
  assert.equal(resolveMessage.payload.query, '洛天依')

  dispatchHost(dom, 'bilibili.user.resolved', {
    query: '洛天依',
    exact: true,
    user: {
      uid: '36081646',
      name: '洛天依',
      avatar_url: 'https://i0.hdslb.com/bfs/face/luotianyi.jpg',
      fans: 7000000,
    },
    candidates: [],
  }, resolveMessage.request_id)

  const select = document.querySelector('.target-select')
  select.options[0].selected = true
  select.dispatchEvent(new dom.window.Event('change', { bubbles: true }))
  assert.equal(document.querySelector('#save-button').disabled, false)
})
