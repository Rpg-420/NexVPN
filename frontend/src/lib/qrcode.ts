import QRCode from 'qrcode'

/**
 * Generates a 100% offline, standalone SVG Data URI for any text or VPN profile URI.
 * Works completely without internet connection or external APIs.
 */
export function generateQRCodeSVG(text: string, size = 220): string {
  if (!text) return ''
  try {
    let result = ''
    // Synchronously render SVG string
    QRCode.toString(
      text,
      {
        type: 'svg',
        width: size,
        margin: 1,
        color: {
          dark: '#000000',
          light: '#ffffff',
        },
      },
      (err, svg) => {
        if (!err && svg) {
          result = `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`
        }
      }
    )
    if (result) return result
  } catch (err) {
    console.error('Offline QR generation error:', err)
  }
  return ''
}
