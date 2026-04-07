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
//# sourceMappingURL=types.js.map