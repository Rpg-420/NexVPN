import QRCode from 'qrcode'

/**
 * Generates a 100% offline, standalone SVG Data URI for any text or VPN profile URI.
 * Works completely without internet connection or external APIs.
 *
 * Implemented with the package's synchronous `create()` API + a hand-built SVG:
 * `QRCode.toString` is async (its callback fires on a later tick), so the old
 * "read the result synchronously" fallback always returned ''.
 */
export function generateQRCodeSVG(text: string, size = 220): string {
  if (!text) return ''
  try {
    const qr = QRCode.create(text, { errorCorrectionLevel: 'M' })
    if (!qr || !qr.modules || !qr.modules.size) return ''
    const count = qr.modules.size
    const data = qr.modules.data
    if (!data || !data.length) return ''

    const scale = 4 // viewBox units per module
    const quiet = 2 // quiet-zone margin, in modules
    const dim = (count + quiet * 2) * scale
    let path = ''
    for (let y = 0; y < count; y++) {
      for (let x = 0; x < count; x++) {
        if (data[y * count + x]) {
          path += `M${(x + quiet) * scale},${(y + quiet) * scale}h${scale}v${scale}h${-scale}z`
        }
      }
    }
    const svg =
      `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" ` +
      `viewBox="0 0 ${dim} ${dim}" shape-rendering="crispEdges">` +
      `<rect width="${dim}" height="${dim}" fill="#ffffff"/>` +
      `<path d="${path}" fill="#000000"/>` +
      `</svg>`
    return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`
  } catch (err) {
    console.error('Offline QR generation error:', err)
  }
  return ''
}
