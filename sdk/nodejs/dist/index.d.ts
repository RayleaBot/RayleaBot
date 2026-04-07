import type { EventBody, Segment } from './types.js';
export type { Frame, Segment, EventBody } from './types.js';
export { textSegment, imageSegment, atSegment, atAllSegment, faceSegment, replySegment, } from './types.js';
export { ActionError } from './protocol.js';
type EventHandler = (event: EventBody, requestId: string) => void | Promise<void>;
interface ActionOptions {
    timeoutMs?: number;
}
export interface RayleaBotPlugin {
    onEvent(handler: EventHandler): RayleaBotPlugin;
    onEvent(eventType: string, handler: EventHandler): RayleaBotPlugin;
    onCommand(name: string, handler: EventHandler, aliases?: string[]): RayleaBotPlugin;
    subscribe(...eventTypes: string[]): RayleaBotPlugin;
    sendMessage(requestId: string, targetType: string, targetId: string, segments: Segment[]): void;
    sendReply(requestId: string, replyToEventId: string, segments: Segment[], options?: {
        fallbackToSendIfMissing?: boolean;
    }): void;
    loggerWrite(requestId: string, level: string, message: string, fields?: Record<string, unknown>, options?: ActionOptions): Promise<Record<string, unknown>>;
    storageGet(requestId: string, key: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    storageSet(requestId: string, key: string, value: unknown, options?: ActionOptions): Promise<Record<string, unknown>>;
    storageDelete(requestId: string, key: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    storageList(requestId: string, prefix?: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    storageFileRead(requestId: string, path: string, options?: ActionOptions & {
        root?: string;
    }): Promise<Record<string, unknown>>;
    storageFileWrite(requestId: string, path: string, options?: ActionOptions & {
        root?: string;
        contentText?: string;
        contentBase64?: string;
    }): Promise<Record<string, unknown>>;
    storageFileDelete(requestId: string, path: string, options?: ActionOptions & {
        root?: string;
    }): Promise<Record<string, unknown>>;
    storageFileList(requestId: string, prefix?: string, options?: ActionOptions & {
        root?: string;
    }): Promise<Record<string, unknown>>;
    httpRequest(requestId: string, method: string, url: string, options?: ActionOptions & {
        headers?: Record<string, string>;
        timeoutSeconds?: number;
        bodyText?: string;
        bodyBase64?: string;
    }): Promise<Record<string, unknown>>;
    configRead(requestId: string, keys: string[], options?: ActionOptions): Promise<Record<string, unknown>>;
    configWrite(requestId: string, values: Record<string, unknown>, options?: ActionOptions): Promise<Record<string, unknown>>;
    schedulerCreate(requestId: string, taskId: string, cron: string, options?: ActionOptions & {
        payload?: Record<string, unknown>;
    }): Promise<Record<string, unknown>>;
    exposeWebhook(requestId: string, route: string, options?: ActionOptions & {
        methods?: string[];
        authStrategy?: string;
        header?: string;
        secretRef: string;
        signaturePrefix?: string;
        sourceIps?: string[];
    }): Promise<Record<string, unknown>>;
    renderImage(requestId: string, template: string, data: Record<string, unknown>, options?: ActionOptions & {
        theme?: string;
        output?: string;
        fallbackText?: string;
    }): Promise<Record<string, unknown>>;
    run(): Promise<void>;
}
export declare function createPlugin(): RayleaBotPlugin;
//# sourceMappingURL=index.d.ts.map