/**
 * Typed definitions for the RayleaBot plugin JSONL protocol.
 *
 * All types correspond to `contracts/plugin-protocol.schema.json`.
 */
// ---------------------------------------------------------------------------
// Segment helpers
// ---------------------------------------------------------------------------
export function textSegment(text) {
    return { type: 'text', data: { text } };
}
export function imageSegment(opts) {
    return { type: 'image', data: opts };
}
export function atSegment(userId) {
    return { type: 'at', data: { user_id: userId } };
}
export function atAllSegment() {
    return { type: 'at_all' };
}
export function faceSegment(faceId) {
    return { type: 'face', data: { face_id: faceId } };
}
export function replySegment(messageId) {
    return { type: 'reply', data: { message_id: messageId } };
}
export function passthroughSegment(type, data = {}) {
    return { type, data };
}
export function markdownSegment(content) {
    return passthroughSegment('markdown', { content });
}
export function fileSegment(data) {
    return passthroughSegment('file', data);
}
export function keyboardSegment(data) {
    return passthroughSegment('keyboard', data);
}
//# sourceMappingURL=types.js.map