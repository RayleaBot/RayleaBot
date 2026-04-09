import { createInterface } from 'readline';
import type { ErrorFrame, Frame, ResultFrame } from './types.js';

const rl = createInterface({ input: process.stdin });
const pendingFrames: Array<Frame | null> = [];
const waitingResolvers: Array<(frame: Frame | null) => void> = [];
const pendingRequests = new Map<
  string,
  {
    resolve: (frame: ResultFrame) => void;
    reject: (error: Error) => void;
  }
>();
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

function enqueueFrame(frame: Frame | null): void {
  if (waitingResolvers.length > 0) {
    waitingResolvers.shift()!(frame);
    return;
  }
  pendingFrames.push(frame);
}

function closeFrameStream(): void {
  if (waitingResolvers.length > 0) {
    while (waitingResolvers.length > 0) {
      waitingResolvers.shift()!(null);
    }
    return;
  }
  pendingFrames.push(null);
}

function rejectPendingRequests(error: Error): void {
  if (pendingRequests.size === 0) {
    return;
  }
  const activeRequests = Array.from(pendingRequests.values());
  pendingRequests.clear();
  for (const pending of activeRequests) {
    pending.reject(error);
  }
}

function isLocalActionResponse(frame: Frame): frame is ResultFrame | ErrorFrame {
  return frame.type === 'result' || frame.type === 'error';
}

rl.on('line', (line: string) => {
  if (!line.trim()) {
    return;
  }
  const frame = JSON.parse(line) as Frame;

  if (isLocalActionResponse(frame)) {
    const pending = pendingRequests.get(frame.request_id);
    if (pending) {
      pendingRequests.delete(frame.request_id);
      if (frame.type === 'result') {
        pending.resolve(frame);
      } else {
        pending.reject(
          new ActionError(
            frame.code ?? 'plugin.internal_error',
            frame.message ?? 'local action failed',
            frame.details ?? {},
          ),
        );
      }
      return;
    }
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
  opts: { parentRequestId?: string } = {},
): void {
  const frame: Record<string, unknown> = {
    protocol_version: '1',
    type: 'action',
    timestamp: Math.floor(Date.now() / 1000),
    plugin_id: pluginId,
    request_id: requestId,
    action,
    data,
  };
  if (opts.parentRequestId) {
    frame.parent_request_id = opts.parentRequestId;
  }
  writeFrame(frame);
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

  return await new Promise<Record<string, unknown>>((resolve, reject) => {
    let timeoutHandle: ReturnType<typeof setTimeout> | null = null;

    const cleanup = (): void => {
      if (timeoutHandle !== null) {
        clearTimeout(timeoutHandle);
      }
      pendingRequests.delete(requestId);
    };

    pendingRequests.set(requestId, {
      resolve: (frame: ResultFrame): void => {
        cleanup();
        resolve(frame.data ?? {});
      },
      reject: (error: Error): void => {
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
