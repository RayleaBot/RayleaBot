import qrcode from 'qrcode-generator'

// Login URLs may contain non-ASCII query values. The encoder's default is single-byte.
qrcode.stringToBytes = text => Array.from(new TextEncoder().encode(text))

export function encodeQRCode(value: string) {
  const code = qrcode(0, 'M')
  code.addData(value)
  code.make()
  // Keep the encoder's high-contrast pixels and a four-module quiet zone for scanning.
  return code.createDataURL(4, 16)
}
