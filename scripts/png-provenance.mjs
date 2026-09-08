import { crc32, inflateSync } from 'node:zlib'

const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])
const keyword = 'impeccable:prompt'
const textTypes = new Set(['tEXt', 'zTXt', 'iTXt'])

function chunks(buffer) {
  if (!buffer.subarray(0, 8).equals(signature)) throw new Error('Invalid PNG signature')
  const result = []
  let offset = 8
  while (offset + 12 <= buffer.length) {
    const length = buffer.readUInt32BE(offset)
    const end = offset + length + 12
    if (end > buffer.length) throw new Error('Truncated PNG chunk')
    const type = buffer.toString('ascii', offset + 4, offset + 8)
    const data = buffer.subarray(offset + 8, end - 4)
    if (crc32(buffer.subarray(offset + 4, end - 4)) !== buffer.readUInt32BE(end - 4)) {
      throw new Error(`Invalid PNG checksum: ${type}`)
    }
    result.push({ type, data, raw: buffer.subarray(offset, end) })
    if (type === 'IEND') return result
    offset = end
  }
  throw new Error('PNG has no complete IEND chunk')
}

function isProvenance(chunk) {
  const separator = chunk.data.indexOf(0)
  return textTypes.has(chunk.type) && separator >= 0 && chunk.data.toString('latin1', 0, separator) === keyword
}

export function readPngProvenance(buffer) {
  const chunk = chunks(buffer).find(isProvenance)
  if (!chunk) return null
  let data = chunk.data.subarray(chunk.data.indexOf(0) + 1)
  if (chunk.type === 'zTXt') {
    if (data[0] !== 0) throw new Error('Unsupported PNG text compression')
    data = inflateSync(data.subarray(1))
  } else if (chunk.type === 'iTXt') {
    const compressed = data[0]
    if (compressed > 1 || data[1] !== 0) throw new Error('Unsupported PNG international text compression')
    data = data.subarray(2)
    for (let index = 0; index < 2; index++) {
      const separator = data.indexOf(0)
      if (separator < 0) throw new Error('Invalid PNG international text')
      data = data.subarray(separator + 1)
    }
    if (compressed) data = inflateSync(data)
  }
  return data.toString('utf8')
}

export function writePngProvenance(buffer, provenance) {
  if (typeof provenance !== 'string' || !provenance.trim()) throw new Error('PNG provenance must be nonempty text')
  const text = Buffer.from(`${keyword}\0${provenance}`, 'utf8')
  const metadata = Buffer.alloc(text.length + 12)
  metadata.writeUInt32BE(text.length)
  metadata.write('tEXt', 4, 'ascii')
  text.copy(metadata, 8)
  metadata.writeUInt32BE(crc32(metadata.subarray(4, -4)), metadata.length - 4)
  const parts = [signature]
  for (const chunk of chunks(buffer)) {
    if (isProvenance(chunk)) continue
    if (chunk.type === 'IEND') parts.push(metadata)
    parts.push(chunk.raw)
  }
  return Buffer.concat(parts)
}
