import { createInterface } from 'readline';
import type {
  Frame,
  InitAckFrame,
  PongFrame,
  ResultFrame,
  ActionFrame,
  ErrorFrame,
} from './types.js';

const rl = createInterface({ input: process.stdin });
const pendingFrames: Array<Frame | null> = [];
const waitingResolvers: Array<(frame: Frame | null) => void> = [];
let streamClosed = false;
let localRequestCounter = 0;

export class ActionError extends Error {
  code: string;
  details: Record<string, unknown>;

  constructor(code: string, message: string, details: Record<string, unknown> = {}) {
    super(message);
    this.name = 'ActionError';
    this.code = code;
    this.details = details;
  }
}

rl.on('line', (line: string) => {
  if (!line.trim()) {
    return;
  }
  const frame = JSON.parse(line) as Frame;
  if (waitingResolvers.length > 0) {
    waitingResolvers.shift()!(frame);
    return;
  }
  pendingFrames.push(frame);
});

rl.on('close', () => {
  streamClosed = true;
  if (waitingResolvers.length > 0) {
    while (waitingResolvers.length > 0) {
      waitingResolvers.shift()!(null);
    }
    return;
  }
  pendingFrames.push(null);
});

function dequeueFrame(): Frame | null | undefined {
  if (pendingFrames.length > 0) {
    return pendingFrames.shift();
  }
  if (streamClosed) {
    return null;
  }
  return undefined;
}

export async function readFrame(opts: { timeoutMs?: number } = {}): Promise<Frame | null> {
  const queued = dequeueFrame();
  if (queued !== undefined) {
    return queued;
  }

  return await new Promise<Frame | null>((resolve, reject) => {
    let timeoutHandle: ReturnType<typeof setTimeout> | null = null;
    const resolveFrame = (frame: Frame | null): void => {
      if (timeoutHandle !== null) {
        clearTimeout(timeoutHandle);
      }
      resolve(frame);
    };
    waitingResolvers.push(resolveFrame);

    if (opts.timeoutMs !== undefined) {
      timeoutHandle = setTimeout(() => {
        const index = waitingResolvers.indexOf(resolveFrame);
        if (index >= 0) {
          waitingResolvers.splice(index, 1);
        }
        reject(new Error('timed out waiting for platform frame'));
      }, opts.timeoutMs);
    }
  });
}

export async function* readFrames(): AsyncGenerator<Frame> {
  while (true) {
    const frame = await readFrame();
    if (frame === null) {
      break;
    }
    yield frame;
  }
}

export function writeFrame(frame: Record<string, unknown>): void {
  process.stdout.write(JSON.stringify(frame) + '\n');
}

export function sendInitAck(
  pluginId: string,
  requestId: string,
  subscriptions: string[] | null = null,
): void {
  const frame: Record<string, unknown> = {
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

export function sendPong(pluginId: string, requestId: string): void {
  writeFrame({
    protocol_version: '1',
    type: 'pong',
    timestamp: Math.floor(Date.now() / 1000),
    plugin_id: pluginId,
    request_id: requestId,
  });
}

export function sendResult(
  pluginId: string,
  requestId: string,
  data: Record<string, unknown> = {},
): void {
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

export function sendAction(
  pluginId: string,
  requestId: string,
  action: string,
  data: Record<string, unknown>,
): void {
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

export function sendError(
  pluginId: string,
  requestId: string,
  code: string,
  message: string,
): void {
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

export function nextLocalRequestId(parentRequestId: string): string {
  localRequestCounter += 1;
  let requestId = `local_${Date.now()}_${localRequestCounter}`;
  if (requestId === parentRequestId) {
    requestId += '_1';
  }
  return requestId;
}

export async function requestLocalAction(
  pluginId: string,
  parentRequestId: string,
  action: string,
  data: Record<string, unknown>,
  opts: { timeoutMs?: number } = {},
): Promise<Record<string, unknown>> {
  const timeoutMs = opts.timeoutMs ?? 30000;
  const requestId = nextLocalRequestId(parentRequestId);
  sendAction(pluginId, requestId, action, data);
  const deadline = Date.now() + timeoutMs;

  while (true) {
    const remaining = deadline - Date.now();
    if (remaining <= 0) {
      throw new Error(`timed out waiting for local action response: ${action}`);
    }

    const frame = await readFrame({ timeoutMs: remaining });
    if (frame === null) {
      throw new Error('stdin closed while waiting for local action response');
    }

    if (frame.type === 'ping') {
      sendPong(pluginId, frame.request_id);
      continue;
    }

    if (frame.type === 'shutdown') {
      throw new Error('received shutdown while waiting for local action response');
    }

    if (frame.request_id !== requestId) {
      throw new Error(`unexpected frame while waiting for local action response: ${frame.type}`);
    }

    if (frame.type === 'result') {
      return frame.data ?? {};
    }

    if (frame.type === 'error') {
      throw new ActionError(
        frame.code ?? 'plugin.internal_error',
        frame.message ?? 'local action failed',
        frame.details ?? {},
      );
    }

    throw new Error(`unexpected frame type while waiting for local action response: ${frame.type}`);
  }
}
