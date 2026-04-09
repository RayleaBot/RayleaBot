/**
 * Typed definitions for the RayleaBot plugin JSONL protocol.
 *
 * All types correspond to `contracts/plugin-protocol.schema.json`.
 */

// ---------------------------------------------------------------------------
// Common fields shared by every frame
// ---------------------------------------------------------------------------

export interface FrameCommon {
  protocol_version: '1';
  type: string;
  timestamp: number;
  plugin_id: string;
  request_id: string;
}

// ---------------------------------------------------------------------------
// Outbound message segments
// ---------------------------------------------------------------------------

export interface TextSegment {
  type: 'text';
  data: { text: string };
}

export interface ImageSegment {
  type: 'image';
  data: { file?: string; url?: string };
}

export interface AtSegment {
  type: 'at';
  data: { user_id: string };
}

export interface AtAllSegment {
  type: 'at_all';
  data?: Record<string, never>;
}

export interface FaceSegment {
  type: 'face';
  data: { face_id: string };
}

export interface ReplySegment {
  type: 'reply';
  data: { message_id: string };
}

export interface PassthroughSegment {
  type:
    | 'record'
    | 'video'
    | 'file'
    | 'json'
    | 'xml'
    | 'markdown'
    | 'music'
    | 'contact'
    | 'forward'
    | 'node'
    | 'poke'
    | 'dice'
    | 'rps'
    | 'mface'
    | 'keyboard'
    | 'shake';
  data?: Record<string, unknown>;
}

export type Segment =
  | TextSegment
  | ImageSegment
  | AtSegment
  | AtAllSegment
  | FaceSegment
  | ReplySegment
  | PassthroughSegment;

export type NonReplySegment = Exclude<Segment, ReplySegment>;

export interface OutboundMessage {
  segments: Segment[];
}

// ---------------------------------------------------------------------------
// Event sub-objects
// ---------------------------------------------------------------------------

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
  message_type?: string;
  request_type?: string;
  notice_type?: string;
  sub_type?: string;
  self_id?: string;
  user_id?: string;
  group_id?: string;
  target_id?: string;
  time?: number;
  message_id?: string;
  real_id?: string;
  message_seq?: string;
  raw_message?: string;
  font?: number;
  message_format?: string;
  sender?: OneBotSender;
  comment?: string;
  flag?: string;
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
  segments?: Array<{ type: string; data?: Record<string, unknown> }>;
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

// ---------------------------------------------------------------------------
// Protocol frames
// ---------------------------------------------------------------------------

export interface InitFrame extends FrameCommon {
  type: 'init';
  bot: Bot;
  capabilities?: string[];
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

export type Frame =
  | InitFrame
  | InitProgressFrame
  | InitAckFrame
  | EventFrame
  | ActionFrame
  | ResultFrame
  | ErrorFrame
  | PingFrame
  | PongFrame
  | ShutdownFrame;

// ---------------------------------------------------------------------------
// Segment helpers
// ---------------------------------------------------------------------------

export function textSegment(text: string): TextSegment {
  return { type: 'text', data: { text } };
}

export function imageSegment(opts: { file?: string; url?: string }): ImageSegment {
  return { type: 'image', data: opts };
}

export function atSegment(userId: string): AtSegment {
  return { type: 'at', data: { user_id: userId } };
}

export function atAllSegment(): AtAllSegment {
  return { type: 'at_all' };
}

export function faceSegment(faceId: string): FaceSegment {
  return { type: 'face', data: { face_id: faceId } };
}

export function replySegment(messageId: string): ReplySegment {
  return { type: 'reply', data: { message_id: messageId } };
}

export function passthroughSegment(type: PassthroughSegment['type'], data: Record<string, unknown> = {}): PassthroughSegment {
  return { type, data };
}

export function markdownSegment(content: string): PassthroughSegment {
  return passthroughSegment('markdown', { content });
}

export function fileSegment(data: Record<string, unknown>): PassthroughSegment {
  return passthroughSegment('file', data);
}

export function keyboardSegment(data: Record<string, unknown>): PassthroughSegment {
  return passthroughSegment('keyboard', data);
}
