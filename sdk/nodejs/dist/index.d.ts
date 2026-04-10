import type { EventBody, Segment } from './types.js';
export type { Frame, Segment, EventBody } from './types.js';
export { textSegment, imageSegment, atSegment, atAllSegment, faceSegment, replySegment, passthroughSegment, markdownSegment, fileSegment, keyboardSegment, } from './types.js';
export { ActionError } from './protocol.js';
type EventHandler = (event: EventBody, requestId: string) => void | Promise<void>;
interface ActionOptions {
    timeoutMs?: number;
}
export interface RayleaBotPlugin {
    readonly commandPrefixes: string[];
    readonly primaryCommandPrefix: string;
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
    pluginList(requestId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    onebotAction(requestId: string, action: string, data?: Record<string, unknown>, options?: ActionOptions): Promise<Record<string, unknown>>;
    providerAction(requestId: string, provider: 'napcat' | 'luckylillia', action: string, data?: Record<string, unknown>, options?: ActionOptions): Promise<Record<string, unknown>>;
    messageHistoryGet(requestId: string, conversationType: 'group' | 'private', conversationId: string, options?: ActionOptions & {
        limit?: number;
    }): Promise<Record<string, unknown>>;
    groupAnnouncementCreate(requestId: string, groupId: string, content: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    fileGroupUpload(requestId: string, groupId: string, fileName: string, fileUrl: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    reactionSet(requestId: string, messageId: string, emoji: string, enabled?: boolean, options?: ActionOptions): Promise<Record<string, unknown>>;
    pokeSend(requestId: string, targetType: 'group' | 'private', targetId: string, userId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    napcatMessageEmojiLikeSet(requestId: string, messageId: string, emojiId: string, enabled?: boolean, options?: ActionOptions): Promise<Record<string, unknown>>;
    luckylilliaFriendGroupsGet(requestId: string, userId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    run(): Promise<void>;
}
export declare function createPlugin(): RayleaBotPlugin;
//# sourceMappingURL=index.d.ts.map