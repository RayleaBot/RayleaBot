/**
 * Typed definitions for the RayleaBot plugin JSONL protocol.
 *
 * All types correspond to `contracts/plugin-protocol.schema.json`.
 */
export interface FrameCommon {
    protocol_version: '1';
    type: string;
    timestamp: number;
    plugin_id: string;
    request_id: string;
}
export interface TextSegment {
    type: 'text';
    data: {
        text: string;
    };
}
export interface ImageSegment {
    type: 'image';
    data: {
        file?: string;
        url?: string;
    };
}
export interface AtSegment {
    type: 'at';
    data: {
        user_id: string;
    };
}
export interface AtAllSegment {
    type: 'at_all';
    data?: Record<string, never>;
}
export interface FaceSegment {
    type: 'face';
    data: {
        face_id: string;
    };
}
export interface ReplySegment {
    type: 'reply';
    data: {
        message_id: string;
    };
}
export type PassthroughSegmentType = 'record' | 'video' | 'file' | 'flash_file' | 'json' | 'xml' | 'markdown' | 'music' | 'contact' | 'forward' | 'node' | 'poke' | 'dice' | 'rps' | 'mface' | 'keyboard' | 'shake';
export interface PassthroughSegment {
    type: PassthroughSegmentType;
    data?: Record<string, unknown>;
}
export type Segment = TextSegment | ImageSegment | AtSegment | AtAllSegment | FaceSegment | ReplySegment | PassthroughSegment;
export type NonReplySegment = Exclude<Segment, ReplySegment>;
export interface OutboundMessage {
    segments: Segment[];
}
export interface Bot {
    id: string;
    nickname?: string;
}
export interface Actor {
    id: string;
    nickname?: string;
    role?: string;
}
export interface EventTarget {
    type: string;
    id: string;
    name?: string;
}
export interface OneBotSender {
    user_id?: string;
    nickname?: string;
    card?: string;
    role?: string;
    title?: string;
    sex?: string;
    age?: number;
}
export interface OneBotPayload {
    post_type?: string;
    meta_event_type?: string;
    message_type?: string;
    request_type?: string;
    notice_type?: string;
    sub_type?: string;
    self_id?: string;
    user_id?: string;
    group_id?: string;
    target_id?: string;
    time?: number;
    interval?: number;
    message_id?: string;
    real_id?: string;
    message_seq?: string;
    raw_message?: string;
    font?: number;
    message_format?: string;
    sender?: OneBotSender;
    comment?: string;
    flag?: string;
    status?: Record<string, unknown>;
}
export interface EventPayload {
    command?: string | null;
    args?: string[];
    message_id?: string;
    sub_type?: string;
    operator_id?: string;
    onebot?: OneBotPayload;
}
export interface EventMessage {
    segments?: Segment[];
    plain_text?: string;
}
export interface EventBody {
    event_id: string;
    source_protocol: string;
    source_adapter: string;
    event_type: string;
    timestamp: number;
    actor?: Actor;
    target?: EventTarget;
    payload?: EventPayload;
    message?: EventMessage;
    raw_payload?: Record<string, unknown>;
}
export interface InitFrame extends FrameCommon {
    type: 'init';
    bot: Bot;
    capabilities?: string[];
    command_prefixes: string[];
}
export interface InitProgressFrame extends FrameCommon {
    type: 'init_progress';
    summary: string;
}
export interface InitAckFrame extends FrameCommon {
    type: 'init_ack';
    status: 'ready' | 'error';
    subscriptions?: string[];
    error_message?: string;
}
export interface EventFrame extends FrameCommon {
    type: 'event';
    event: EventBody;
}
export interface ActionFrame extends FrameCommon {
    type: 'action';
    parent_request_id?: string;
    action: string;
    data: Record<string, unknown>;
}
export interface ResultFrame extends FrameCommon {
    type: 'result';
    status: 'success';
    data: Record<string, unknown>;
}
export interface ErrorFrame extends FrameCommon {
    type: 'error';
    code: string;
    message: string;
    details?: Record<string, unknown>;
}
export interface PingFrame extends FrameCommon {
    type: 'ping';
}
export interface PongFrame extends FrameCommon {
    type: 'pong';
}
export interface ShutdownFrame extends FrameCommon {
    type: 'shutdown';
    reason: 'stop' | 'restart' | 'reload';
}
export type Frame = InitFrame | InitProgressFrame | InitAckFrame | EventFrame | ActionFrame | ResultFrame | ErrorFrame | PingFrame | PongFrame | ShutdownFrame;
export declare function textSegment(text: string): TextSegment;
export declare function imageSegment(opts: {
    file?: string;
    url?: string;
}): ImageSegment;
export declare function atSegment(userId: string): AtSegment;
export declare function atAllSegment(): AtAllSegment;
export declare function faceSegment(faceId: string): FaceSegment;
export declare function replySegment(messageId: string): ReplySegment;
export declare function passthroughSegment(type: PassthroughSegmentType, data?: Record<string, unknown>): PassthroughSegment;
export declare function recordSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function videoSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function fileSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function flashFileSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function jsonSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function xmlSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function markdownSegment(content: string): PassthroughSegment;
export declare function markdownSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function musicSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function contactSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function forwardSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function nodeSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function pokeSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function diceSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function rpsSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function mfaceSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function keyboardSegment(data?: Record<string, unknown>): PassthroughSegment;
export declare function shakeSegment(data?: Record<string, unknown>): PassthroughSegment;
//# sourceMappingURL=types.d.ts.map