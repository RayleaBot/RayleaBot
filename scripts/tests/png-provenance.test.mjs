import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'
import { crc32, deflateSync } from 'node:zlib'
import { readPngProvenance, writePngProvenance } from '../png-provenance.mjs'

const original = fs.readFileSync(new URL('../../launcher/assets/appicon.png', import.meta.url))

function imageChunks(buffer) {
  const result = []
  for (let offset = 8; offset + 12 <= buffer.length;) {
    const end = offset + buffer.readUInt32BE(offset) + 12
    if (!['tEXt', 'iTXt', 'zTXt'].includes(buffer.toString('ascii', offset + 4, offset + 8))) result.push(buffer.subarray(offset, end))
    offset = end
  }
  return Buffer.concat(result)
}

function chunk(type, data) {
  const buffer = Buffer.alloc(data.length + 12)
  buffer.writeUInt32BE(data.length)
  buffer.write(type, 4)
  data.copy(buffer, 8)
  buffer.writeUInt32BE(crc32(buffer.subarray(4, -4)), buffer.length - 4)
  return buffer
}

test('replaces Unicode provenance while preserving the image and repeated writes', () => {
  const text = '来源：design/mark.json；青瓷折叶 🍃'
  const updated = writePngProvenance(original, text)
  assert.equal(readPngProvenance(updated), text)
  assert.deepEqual(imageChunks(updated), imageChunks(original))
  assert.deepEqual(writePngProvenance(updated, text), updated)
})

test('reads compressed PNG text and international text', () => {
  const text = '已授权的图标来源'
  const image = Buffer.concat([original.subarray(0, 8), imageChunks(original)])
  for (const metadata of [
    chunk('zTXt', Buffer.concat([Buffer.from('impeccable:prompt\0\0'), deflateSync(text)])),
    chunk('iTXt', Buffer.concat([Buffer.from('impeccable:prompt\0\x01\0zh\0来源\0'), deflateSync(text)])),
  ]) {
    const withMetadata = Buffer.concat([image.subarray(0, -12), metadata, image.subarray(-12)])
    assert.equal(readPngProvenance(withMetadata), text)
    assert.equal(readPngProvenance(writePngProvenance(withMetadata, 'updated')), 'updated')
  }
})

test('rejects truncated or corrupt PNG data before changing metadata', () => {
  assert.throws(() => writePngProvenance(original.subarray(0, 25), 'source'), /Truncated PNG/)
  const corrupt = Buffer.from(original)
  corrupt[20] ^= 1
  assert.throws(() => readPngProvenance(corrupt), /checksum/)
  assert.throws(() => writePngProvenance(Buffer.from('not a png'), 'source'), /signature/)
})
