import type { Frame } from './types.js';
export declare class ActionError extends Error {
    code: string;
    details: Record<string, unknown>;
    constructor(code: string, message: string, details?: Record<string, unknown>);
}
export declare function readFrame(opts?: {
    timeoutMs?: number;
}): Promise<Frame | null>;
export declare function readFrames(): AsyncGenerator<Frame>;
export declare function writeFrame(frame: Record<string, unknown>): void;
export declare function sendInitAck(pluginId: string, requestId: string, subscriptions?: string[] | null): void;
export declare function sendPong(pluginId: string, requestId: string): void;
export declare function sendResult(pluginId: string, requestId: string, data?: Record<string, unknown>): void;
export declare function sendAction(pluginId: string, requestId: string, action: string, data: Record<string, unknown>, opts?: {
    parentRequestId?: string;
}): void;
export declare function sendError(pluginId: string, requestId: string, code: string, message: string): void;
export declare function nextLocalRequestId(parentRequestId: string): string;
export declare function requestLocalAction(pluginId: string, parentRequestId: string, action: string, data: Record<string, unknown>, opts?: {
    timeoutMs?: number;
}): Promise<Record<string, unknown>>;
//# sourceMappingURL=protocol.d.ts.map