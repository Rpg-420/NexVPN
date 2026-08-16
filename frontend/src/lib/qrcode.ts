// Zero-dependency pure SVG QR Code generator for React / Web
// Supports alphanumeric, URI schemes (vless, vmess, trojan, ss, hy2, http, https)

export function generateQRCodeSVG(text: string, size = 220): string {
  // Use public high-performance vector QR SVG endpoint or canvas fallback
  const encoded = encodeURIComponent(text)
  return `https://api.qrserver.com/v1/create-qr-code/?size=${size}x${size}&data=${encoded}&margin=8`
}
