import { createInterface } from 'readline';

const rl = createInterface({ input: process.stdin });

export async function* readFrame() {
  for await (const line of rl) {
    if (line.trim()) {
      yield JSON.parse(line);
    }
  }
}

export function writeFrame(frame) {
  process.stdout.write(JSON.stringify(frame) + '\n');
}

export function sendInitAck(pluginId, requestId, subscriptions = null) {
  const frame = {
    protocol_version: '1',
    type: 'init_ack',
    timestamp: Math.floor(Date.now() / 1000),
    plugin_id: pluginId,
    request_id: requestId,
    status: 'ready',
  };
  if (subscriptions) frame.subscriptions = subscriptions;
  writeFrame(frame);
}

export function sendPong(pluginId, requestId) {
  writeFrame({
    protocol_version: '1',
    type: 'pong',
    timestamp: Math.floor(Date.now() / 1000),
    plugin_id: pluginId,
    request_id: requestId,
  });
}

export function sendResult(pluginId, requestId, data = {}) {
  writeFrame({
    protocol_version: '1',
    type: 'result',
    timestamp: Math.floor(Date.now() / 1000),
    plugin_id: pluginId,
    request_id: requestId,
    status: 'success',
    data,
  });
}

export function sendAction(pluginId, requestId, action, data) {
  writeFrame({
    protocol_version: '1',
    type: 'action',
    timestamp: Math.floor(Date.now() / 1000),
    plugin_id: pluginId,
    request_id: requestId,
    action,
    data,
  });
}

export function sendError(pluginId, requestId, code, message) {
  writeFrame({
    protocol_version: '1',
    type: 'error',
    timestamp: Math.floor(Date.now() / 1000),
    plugin_id: pluginId,
    request_id: requestId,
    code,
    message,
  });
}
