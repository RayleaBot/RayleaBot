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
export function recordSegment(data = {}) {
    return passthroughSegment('record', data);
}
export function videoSegment(data = {}) {
    return passthroughSegment('video', data);
}
export function fileSegment(data = {}) {
    return passthroughSegment('file', data);
}
export function flashFileSegment(data = {}) {
    return passthroughSegment('flash_file', data);
}
export function jsonSegment(data = {}) {
    return passthroughSegment('json', data);
}
export function xmlSegment(data = {}) {
    return passthroughSegment('xml', data);
}
export function markdownSegment(contentOrData = {}) {
    if (typeof contentOrData === 'string') {
        return passthroughSegment('markdown', { content: contentOrData });
    }
    return passthroughSegment('markdown', contentOrData);
}
export function musicSegment(data = {}) {
    return passthroughSegment('music', data);
}
export function contactSegment(data = {}) {
    return passthroughSegment('contact', data);
}
export function forwardSegment(data = {}) {
    return passthroughSegment('forward', data);
}
export function nodeSegment(data = {}) {
    return passthroughSegment('node', data);
}
export function pokeSegment(data = {}) {
    return passthroughSegment('poke', data);
}
export function diceSegment(data = {}) {
    return passthroughSegment('dice', data);
}
export function rpsSegment(data = {}) {
    return passthroughSegment('rps', data);
}
export function mfaceSegment(data = {}) {
    return passthroughSegment('mface', data);
}
export function keyboardSegment(data = {}) {
    return passthroughSegment('keyboard', data);
}
export function shakeSegment(data = {}) {
    return passthroughSegment('shake', data);
}
//# sourceMappingURL=types.js.map