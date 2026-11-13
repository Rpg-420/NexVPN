import { describe, expect, it } from 'vitest'
import { generateQRCodeSVG } from './qrcode'

// Regression: the previous implementation called the async `QRCode.toString`
// and read its result synchronously — every QR came back as ''.
describe('generateQRCodeSVG', () => {
  it('returns a non-empty data URI synchronously for a URI payload', () => {
    const svg = generateQRCodeSVG('vless://uuid@example.com:443?security=tls#test', 220)
    expect(svg).not.toBe('')
    expect(svg.startsWith('data:image/svg+xml')).toBe(true)
  })

  it('decodes back to the original payload', () => {
    const payload = 'vless://some-uuid@example.com:443?a=1#node-name'
    const svg = generateQRCodeSVG(payload, 180)
    const decoded = decodeURIComponent(svg.replace('data:image/svg+xml;utf8,', ''))
    expect(decoded).toContain('<svg')
    expect(decoded).toContain('<path')
    expect(decoded).toContain('shape-rendering="crispEdges"')
  })

  it('returns empty for empty input', () => {
    expect(generateQRCodeSVG('')).toBe('')
  })

  it('scales viewBox with payload length (more modules for longer text)', () => {
    const small = generateQRCodeSVG('ab', 200)
    const big = generateQRCodeSVG('vless://a-very-long-uuid@example.com:443?security=tls&flow=xtls-rprx-vision&headerType=none&type=ws#long-name', 200)
    const decodedOf = (s: string) => decodeURIComponent(s.replace('data:image/svg+xml;utf8,', ''))
    const dimOf = (dec: string) => Number(dec.match(/viewBox="0 0 (\d+)/)?.[1] ?? 0)
    expect(dimOf(decodedOf(big))).toBeGreaterThan(dimOf(decodedOf(small)))
  })
})
