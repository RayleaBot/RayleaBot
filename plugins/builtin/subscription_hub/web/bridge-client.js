export function createBridgeClient(win, handlers = {}) {
  let requestCounter = 0

  function nextRequestId(prefix) {
    requestCounter += 1
    return `${prefix}-${Date.now()}-${requestCounter}`
  }

  function parentTargetOrigin() {
    const ancestorOrigins = win.location && win.location.ancestorOrigins
    if (ancestorOrigins && ancestorOrigins.length > 0) {
      const origin = String(ancestorOrigins[0] || '').trim()
      if (origin) {
        return origin
      }
    }
    try {
      const referrer = win.document && win.document.referrer
      if (referrer) {
        return new URL(referrer).origin
      }
    } catch {
      // Fall back to the iframe origin when no parent origin is exposed.
    }
    return win.location.origin
  }

  function send(type, payload, requestId) {
    const id = requestId || nextRequestId(type.replaceAll('.', '-'))
    win.parent.postMessage({
      version: '1',
      source: 'plugin_management_ui',
      type,
      request_id: id,
      ...(payload === undefined ? {} : { payload }),
    }, parentTargetOrigin())
    return id
  }

  function normalizeMessage(raw) {
    const message = raw || {}
    if (message.version !== '1' || message.source !== 'management_host') {
      return null
    }
    if (message.type === 'error') {
      return {
        ...message,
        error: normalizeBridgeError(message),
      }
    }
    return message
  }

  function handleMessage(event) {
    const message = normalizeMessage(event.data)
    if (!message) {
      return
    }
    if (message.error && handlers.onError) {
      handlers.onError(message)
      return
    }
    if (handlers.onMessage) {
      handlers.onMessage(message)
    }
  }

  win.addEventListener('message', handleMessage)

  return {
    nextRequestId,
    send,
    destroy() {
      win.removeEventListener('message', handleMessage)
    },
    pageReady() {
      return send('page.ready', undefined, nextRequestId('page-ready'))
    },
    reloadSettings() {
      return send('settings.reload', undefined, nextRequestId('settings-reload'))
    },
    saveSettings(values, requestId) {
      return send('settings.save', { values }, requestId || nextRequestId('settings-save'))
    },
    reloadTargets(requestId) {
      return send('protocol.targets.reload', undefined, requestId || nextRequestId('protocol-targets'))
    },
    resolveIdentities(items, requestId) {
      return send('protocol.identities.resolve', { items }, requestId || nextRequestId('protocol-identities'))
    },
    invokeAction(action, payload = {}, requestId) {
      return send('plugin.action.invoke', { action, payload }, requestId || nextRequestId('plugin-action'))
    },
    resolveBilibiliUser(query, requestId) {
      return send('plugin.action.invoke', {
        action: 'subscription.resolve_user',
        payload: { platform: 'bilibili', query },
      }, requestId || nextRequestId('subscription-resolve-user'))
    },
    resolvePlatformUser(platform, query, requestId) {
      return send('plugin.action.invoke', {
        action: 'subscription.resolve_user',
        payload: { platform, query },
      }, requestId || nextRequestId('subscription-resolve-user'))
    },
    checkNow(requestId) {
      return send('plugin.action.invoke', {
        action: 'subscription.check_now',
        payload: {},
      }, requestId || nextRequestId('subscription-check-now'))
    },
    openRenderTemplate(templateId) {
      return send('render_template.open', { template_id: templateId }, nextRequestId('open-template'))
    },
  }
}

export function normalizeBridgeError(message) {
  const payload = message && message.payload && typeof message.payload === 'object' ? message.payload : {}
  return {
    request_id: message && message.request_id ? message.request_id : '',
    code: typeof payload.code === 'string' ? payload.code : 'bridge.error',
    message: typeof payload.message === 'string' && payload.message.trim()
      ? payload.message.trim()
      : '操作失败',
    details: payload.details,
  }
}
