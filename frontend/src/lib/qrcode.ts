import QRCode from 'qrcode'

/**
 * Generates a 100% offline, standalone SVG Data URI for any text or VPN profile URI.
 * Works completely without internet connection or external APIs.
 */
export function generateQRCodeSVG(text: string, size = 220): string {
  if (!text) return ''
  try {
    // Use synchronous API if available (qrcode >=1.5)
    const anyQR = QRCode as any
    if (typeof anyQR.toStringSync === 'function') {
      const svg = anyQR.toStringSync(text, {
        type: 'svg',
        width: size,
        margin: 1,
        color: { dark: '#000000', light: '#ffffff' },
      })
      if (svg) return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`
    }
    // Fallback to async callback (capture synchronously via deasync-style try)
    let result = ''
    QRCode.toString(
      text,
      {
        type: 'svg',
        width: size,
        margin: 1,
        color: { dark: '#000000', light: '#ffffff' },
      },
      (err: any, svg: string) => {
        if (!err && svg) result = `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`
      }
    )
    if (result) return result
  } catch (err) {
    console.error('Offline QR generation error:', err)
  }
  return ''
}
