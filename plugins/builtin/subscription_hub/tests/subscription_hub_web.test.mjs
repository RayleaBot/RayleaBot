import assert from 'node:assert/strict'
import fs from 'node:fs'
import { createRequire } from 'node:module'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { buildRowsFromSettings, normalizeSettings } from '../web/model.js'
import { buildSettingsPayload } from '../web/settings-payload.js'
import { normalizeTargets, targetMap } from '../web/targets.js'

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

function lastActionMessage(messages, action) {
  return messages.findLast((message) => message.type === 'plugin.action.invoke' && message.payload?.action === action)
}

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

function editFirstCard(document) {
  const card = document.querySelector('.sub-card')
  card.querySelector('button[data-action="edit-row"]').click()
  return document.querySelector('.sub-card')
}

function commonServiceInputs(document) {
  return Object.fromEntries([...document.querySelectorAll('.service-checkbox[data-target-key="common"]')]
    .map((input) => [input.value, input]))
}

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

test('settings page uses a classic script entry for sandboxed plugin iframes', () => {
  assert.match(subscriptionHtml, /<script src="\.\/app\.js"><\/script>/)
  assert.doesNotMatch(subscriptionHtml, /type="module"/)
  assert.doesNotMatch(script, /^\s*import\s/m)
})

test('pure model groups subscriptions by Bilibili UID and payload splits them back', () => {
  const settings = normalizeSettings(richSettings)
  const rows = buildRowsFromSettings(settings)
  const payload = buildSettingsPayload(settings, rows, targetMap(normalizeTargets(targetsPayload)))

  assert.equal(rows.length, 1)
  assert.equal(rows[0].targets.length, 2)
  assert.deepEqual(payload.subscriptions.map((item) => ({
    target_type: item.target_type,
    target_id: item.target_id,
    target_name: item.target_name,
    services: item.services,
    subscribers: item.subscribers,
  })), [
    {
      target_type: 'group',
      target_id: '5050',
      target_name: '测试群',
      services: ['live'],
      subscribers: [{ id: '10001' }],
    },
    {
      target_type: 'private',
      target_id: '2626',
      target_name: '测试用户',
      services: ['video'],
      subscribers: [{ id: '10001' }],
    },
  ])
})

test('pure model keeps equal IDs separated across platforms', () => {
  const settings = normalizeSettings({
    enabled: true,
    subscriptions: [{
      id: 'bilibili-123456-group-5050',
      platform: 'bilibili',
      uid: '123456',
      name: '测试 UP',
      target_type: 'group',
      target_id: '5050',
      services: ['video'],
      subscribers: [],
      enabled: true,
    }, {
      id: 'weibo-123456-group-5050',
      platform: 'weibo',
      uid: '123456',
      name: '测试微博',
      target_type: 'group',
      target_id: '5050',
      services: ['post'],
      subscribers: [],
      enabled: true,
    }],
  })
  const rows = buildRowsFromSettings(settings)
  const payload = buildSettingsPayload(settings, rows, targetMap(normalizeTargets(targetsPayload)))

  assert.equal(rows.length, 2)
  assert.deepEqual(rows.map((row) => row.platform).sort(), ['bilibili', 'weibo'])
  assert.deepEqual(payload.subscriptions.map((item) => ({
    platform: item.platform,
    uid: item.uid,
    services: item.services,
  })).sort((a, b) => a.platform.localeCompare(b.platform)), [
    { platform: 'bilibili', uid: '123456', services: ['video'] },
    { platform: 'weibo', uid: '123456', services: ['post'] },
  ])
})

test('groups subscriptions by Bilibili UID and hides raw maintenance fields', () => {
  const { dom } = createPage()
  const document = dom.window.document
  const rows = document.querySelectorAll('.sub-card')

  assert.equal(rows.length, 1)
  assert.equal(document.querySelector('#raw-json-input'), null)
  assert.equal(document.querySelector('#subscription-editor-panel'), null)
  assert.match(rows[0].textContent, /测试 UP/)
  assert.match(rows[0].textContent, /2 个推送对象/)
  assert.match(rows[0].textContent, /1 位订阅人/)
  assert.match(rows[0].textContent, /目标配置不同/)
})

test('target type switch keeps targets selected across group and private lists', () => {
  const { dom } = createPage()
  const document = dom.window.document
  const row = editFirstCard(document)
  const privateButton = row.querySelector('button[data-action="target-mode"][data-mode="private"]')
  privateButton.click()

  const privateRow = document.querySelector('.sub-card')
  const privateOptions = privateRow.querySelectorAll('.target-option')
  assert.equal(privateOptions.length, 1)
  assert.equal(privateOptions[0].getAttribute('aria-selected'), 'true')
  assert.match(privateRow.textContent, /群聊 测试群/)
  assert.match(privateRow.textContent, /私聊 测试用户/)

  privateRow.querySelector('button[data-action="target-mode"][data-mode="group"]').click()
  const groupRow = document.querySelector('.sub-card')
  const groupOptions = groupRow.querySelectorAll('.target-option')
  groupOptions[1].click()

  const updatedRow = document.querySelector('.sub-card')
  assert.match(updatedRow.textContent, /群聊 测试群/)
  assert.match(updatedRow.textContent, /群聊 备用群/)
  assert.match(updatedRow.textContent, /私聊 测试用户/)
})

test('target multi-select updates current card without replacing the select', () => {
  const { dom } = createPage()
  const document = dom.window.document
  const row = editFirstCard(document)
  const list = row.querySelector('.target-select')
  list.scrollTop = 32

  const option = row.querySelectorAll('.target-option')[1]
  option.focus()
  option.click()

  const currentList = document.querySelector('.target-select')
  assert.equal(currentList, list)
  assert.equal(currentList.scrollTop, 32)
  assert.equal(currentList.querySelectorAll('.target-option')[1].getAttribute('aria-selected'), 'true')
  assert.equal(currentList.querySelectorAll('.target-option')[1].classList.contains('is-selected'), true)
  assert.equal(document.activeElement.dataset.targetKey, 'group:6060')
  assert.match(document.querySelector('.sub-card').textContent, /群聊 备用群/)
  assert.match(document.querySelector('#dirty-state').textContent, /设置有修改/)
})

test('all service checkbox selects all services and follows complete selection', () => {
  const { dom } = createPage({
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
  editFirstCard(document)

  let inputs = commonServiceInputs(document)
  assert.deepEqual(Object.fromEntries(Object.entries(inputs).map(([key, input]) => [key, input.checked])), {
    all: true,
    live: true,
    video: true,
    image_text: true,
    article: true,
    repost: true,
  })

  inputs.live.click()
  inputs = commonServiceInputs(document)
  assert.equal(inputs.all.checked, false)
  assert.equal(inputs.live.checked, false)
  assert.equal(inputs.video.checked, true)

  inputs.live.click()
  inputs = commonServiceInputs(document)
  assert.equal(inputs.all.checked, true)
  assert.equal(inputs.live.checked, true)
  assert.equal(inputs.repost.checked, true)

  inputs.video.click()
  inputs = commonServiceInputs(document)
  assert.equal(inputs.all.checked, false)
  assert.equal(inputs.video.checked, false)

  inputs.all.click()
  inputs = commonServiceInputs(document)
  assert.ok(Object.values(inputs).every((input) => input.checked))
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

test('saving subscriber IDs resolves identity before settings save and stores only QQ numbers', () => {
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
  assert.deepEqual(plain(values.subscriptions[0].subscribers), [{ id: '10001' }])
  assert.equal(values.subscriptions[1].target_name, '测试用户')
  assert.deepEqual(plain(values.subscriptions[1].subscribers), [{ id: '10001' }])
})

test('identity resolve failure prevents settings save', () => {
  const { dom, messages } = createPage()
  const document = dom.window.document

  document.querySelector('#save-button').click()
  const resolveMessage = lastMessage(messages, 'protocol.identities.resolve')
  assert.ok(resolveMessage)

  dispatchHost(dom, 'protocol.identities.resolved', {
    items: [],
    issues: [{ message: '订阅人身份解析失败' }],
  }, resolveMessage.request_id)

  assert.equal(saveMessage(messages), undefined)
  assert.match(document.querySelector('#status-text').textContent, /订阅人身份解析失败/)
})

test('missing protocol target prevents settings save', () => {
  const { dom, messages } = createPage({
    enabled: true,
    subscriptions: [{
      id: 'bilibili-123456-group-9999',
      platform: 'bilibili',
      uid: '123456',
      name: '测试 UP',
      target_type: 'group',
      target_id: '9999',
      target_name: '不存在的群',
      services: ['all'],
      subscribers: [],
      enabled: true,
    }],
  })
  const document = dom.window.document

  assert.equal(document.querySelector('#save-button').disabled, true)
  document.querySelector('#save-button').click()
  assert.equal(saveMessage(messages), undefined)
})

test('new row must resolve Bilibili user before saving', () => {
  const { dom, messages } = createPage(defaultSettings)
  const document = dom.window.document

  document.querySelector('#add-subscription-button').click()
  assert.equal(document.querySelector('#save-button').disabled, true)

  const input = document.querySelector('.up-query-input')
  input.value = '测试 UP'
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  document.querySelector('button[data-action="resolve-up"]').click()
  const resolveMessage = lastActionMessage(messages, 'subscription.resolve_user')
  assert.equal(resolveMessage.payload.payload.platform, 'bilibili')
  assert.equal(resolveMessage.payload.payload.query, '测试 UP')

  dispatchHost(dom, 'plugin.action.result', {
    action: 'subscription.resolve_user',
    result: {
      platform: 'bilibili',
      query: '测试 UP',
      exact: true,
      user: {
        uid: '1000001',
        name: '测试 UP',
        avatar_url: 'https://i0.hdslb.com/bfs/face/test-up.jpg',
        fans: 7000000,
      },
      candidates: [],
    },
  }, resolveMessage.request_id)

  document.querySelector('.target-option').click()
  assert.equal(document.querySelector('#save-button').disabled, false)
})

test('new Weibo row resolves profile and saves platform services', () => {
  const { dom, messages } = createPage(defaultSettings)
  const document = dom.window.document

  document.querySelector('#add-subscription-button').click()
  const platformSelect = document.querySelector('.platform-select')
  platformSelect.value = 'weibo'
  platformSelect.dispatchEvent(new dom.window.Event('change', { bubbles: true }))

  const input = document.querySelector('.up-query-input')
  input.value = '洛天依'
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  document.querySelector('button[data-action="resolve-up"]').click()
  const resolveMessage = lastActionMessage(messages, 'subscription.resolve_user')
  assert.equal(resolveMessage.payload.payload.platform, 'weibo')
  assert.equal(resolveMessage.payload.payload.query, '洛天依')
  assert.equal(document.querySelector('#save-button').disabled, true)

  dispatchHost(dom, 'plugin.action.result', {
    action: 'subscription.resolve_user',
    result: {
      platform: 'weibo',
      query: '洛天依',
      exact: true,
      user: {
        uid: '7556659984',
        name: '洛天依',
        avatar_url: 'https://tvax1.sinaimg.cn/avatar.jpg',
      },
      candidates: [],
    },
  }, resolveMessage.request_id)

  document.querySelector('.target-option').click()

  let inputs = commonServiceInputs(document)
  inputs.all.click()
  inputs = commonServiceInputs(document)
  inputs.video.click()

  assert.equal(document.querySelector('#save-button').disabled, false)
  document.querySelector('#save-button').click()

  const values = saveMessage(messages).payload.values
  assert.equal(values.subscriptions.length, 1)
  assert.equal(values.subscriptions[0].platform, 'weibo')
  assert.equal(values.subscriptions[0].uid, '7556659984')
  assert.equal(values.subscriptions[0].name, '洛天依')
  assert.equal(values.subscriptions[0].avatar_url, 'https://tvax1.sinaimg.cn/avatar.jpg')
  assert.deepEqual(plain(values.subscriptions[0].services), ['video'])
})

test('composition input resolves after Chinese IME commits', async () => {
  const { dom, messages } = createPage(defaultSettings)
  const document = dom.window.document

  document.querySelector('#add-subscription-button').click()
  const platformSelect = document.querySelector('.platform-select')
  platformSelect.value = 'weibo'
  platformSelect.dispatchEvent(new dom.window.Event('change', { bubbles: true }))

  const input = document.querySelector('.up-query-input')
  input.dispatchEvent(new dom.window.CompositionEvent('compositionstart', { bubbles: true }))
  input.value = '洛'
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  await wait(760)
  assert.equal(lastActionMessage(messages, 'subscription.resolve_user'), undefined)

  input.value = '洛天依'
  input.dispatchEvent(new dom.window.CompositionEvent('compositionend', { bubbles: true }))
  await wait(760)
  const resolveMessage = lastActionMessage(messages, 'subscription.resolve_user')
  assert.equal(resolveMessage.payload.payload.platform, 'weibo')
  assert.equal(resolveMessage.payload.payload.query, '洛天依')
})

test('resolve bridge error clears checking state on the row', () => {
  const { dom, messages } = createPage(defaultSettings)
  const document = dom.window.document

  document.querySelector('#add-subscription-button').click()
  const platformSelect = document.querySelector('.platform-select')
  platformSelect.value = 'weibo'
  platformSelect.dispatchEvent(new dom.window.Event('change', { bubbles: true }))

  const input = document.querySelector('.up-query-input')
  input.value = '我的世界'
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  document.querySelector('button[data-action="resolve-up"]').click()
  const resolveMessage = lastActionMessage(messages, 'subscription.resolve_user')

  dispatchHost(dom, 'error', {
    code: 'platform.upstream_request_failed',
    message: '三方平台用户信息读取失败',
  }, resolveMessage.request_id)

  const rowText = document.querySelector('.sub-card').textContent
  assert.match(rowText, /待校验/)
  assert.match(rowText, /三方平台用户信息读取失败/)
  assert.doesNotMatch(rowText, /校验中/)
})
