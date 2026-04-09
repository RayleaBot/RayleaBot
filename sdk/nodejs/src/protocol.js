import { createInterface } from 'readline';

const rl = createInterface({ input: process.stdin });
const pendingFrames = [];
const waitingResolvers = [];
const pendingRequests = new Map();
let streamClosed = false;
let localRequestCounter = 0;

export class ActionError extends Error {
  constructor(code, message, details = {}) {
    super(message);
    this.name = 'ActionError';
    this.code = code;
    this.details = details;
  }
}

rl.on('line', (line) => {
  if (!line.trim()) {
    return;
  }
  const frame = JSON.parse(line);
  if ((frame.type === 'result' || frame.type === 'error') && pendingRequests.has(frame.request_id)) {
    const pending = pendingRequests.get(frame.request_id);
    pendingRequests.delete(frame.request_id);
    if (frame.type === 'result') {
      pending.resolve(frame);
    } else {
      pending.reject(new ActionError(frame.code ?? 'plugin.internal_error', frame.message ?? 'local action failed', frame.details ?? {}));
    }
    return;
  }
  enqueueFrame(frame);
  if (frame.type === 'shutdown') {
    rejectPendingRequests(new Error('received shutdown while waiting for local action response'));
  }
});

rl.on('close', () => {
  streamClosed = true;
  rejectPendingRequests(new Error('stdin closed while waiting for local action response'));
  closeFrameStream();
});

function enqueueFrame(frame) {
  if (waitingResolvers.length > 0) {
    waitingResolvers.shift()(frame);
    return;
  }
  pendingFrames.push(frame);
}

function closeFrameStream() {
  if (waitingResolvers.length > 0) {
    while (waitingResolvers.length > 0) {
      waitingResolvers.shift()(null);
    }
    return;
  }
  pendingFrames.push(null);
}

function rejectPendingRequests(error) {
  if (pendingRequests.size === 0) {
    return;
  }
  const activeRequests = Array.from(pendingRequests.values());
  pendingRequests.clear();
  for (const pending of activeRequests) {
    pending.reject(error);
  }
}

function dequeueFrame() {
  if (pendingFrames.length > 0) {
    return pendingFrames.shift();
  }
  if (streamClosed) {
    return null;
  }
  return undefined;
}

export async function readFrame({ timeoutMs } = {}) {
  const queued = dequeueFrame();
  if (queued !== undefined) {
    return queued;
  }

  return await new Promise((resolve, reject) => {
    let timeoutHandle = null;
    const resolveFrame = (frame) => {
      if (timeoutHandle !== null) {
        clearTimeout(timeoutHandle);
      }
      resolve(frame);
    };
    waitingResolvers.push(resolveFrame);

    if (timeoutMs !== undefined) {
      timeoutHandle = setTimeout(() => {
        const index = waitingResolvers.indexOf(resolveFrame);
        if (index >= 0) {
          waitingResolvers.splice(index, 1);
        }
        reject(new Error('timed out waiting for platform frame'));
      }, timeoutMs);
    }
  });
}

export async function* readFrames() {
  while (true) {
    const frame = await readFrame();
    if (frame === null) {
      break;
    }
    yield frame;
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
  if (subscriptions) {
    frame.subscriptions = subscriptions;
  }
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

export function sendAction(pluginId, requestId, action, data, { parentRequestId } = {}) {
  const frame = {
    protocol_version: '1',
    type: 'action',
    timestamp: Math.floor(Date.now() / 1000),
    plugin_id: pluginId,
    request_id: requestId,
    action,
    data,
  };
  if (parentRequestId) {
    frame.parent_request_id = parentRequestId;
  }
  writeFrame(frame);
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

export function nextLocalRequestId(parentRequestId) {
  localRequestCounter += 1;
  let requestId = `local_${Date.now()}_${localRequestCounter}`;
  if (requestId === parentRequestId) {
    requestId += '_1';
  }
  return requestId;
}

export async function requestLocalAction(pluginId, parentRequestId, action, data, { timeoutMs = 30000 } = {}) {
  const requestId = nextLocalRequestId(parentRequestId);
  return await new Promise((resolve, reject) => {
    let timeoutHandle = null;

    const cleanup = () => {
      if (timeoutHandle !== null) {
        clearTimeout(timeoutHandle);
      }
      pendingRequests.delete(requestId);
    };

    pendingRequests.set(requestId, {
      resolve: (frame) => {
        cleanup();
        resolve(frame.data ?? {});
      },
      reject: (error) => {
        cleanup();
        reject(error);
      },
    });

    if (timeoutMs > 0) {
      timeoutHandle = setTimeout(() => {
        if (!pendingRequests.has(requestId)) {
          return;
        }
        cleanup();
        reject(new Error(`timed out waiting for local action response: ${action}`));
      }, timeoutMs);
    }

    try {
      sendAction(pluginId, requestId, action, data, { parentRequestId });
    } catch (error) {
      cleanup();
      reject(error instanceof Error ? error : new Error(String(error)));
    }
  });
}
