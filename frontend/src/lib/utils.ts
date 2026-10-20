import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'
import type { Protocol } from '../types'

/**
 * Merge Tailwind class names, resolving conflicts (later wins) while keeping
 * conditional classes readable. Standard shadcn/ui helper.
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}

/**
 * Builds a rejection handler for a fire-and-forget backend call.
 *
 * Several `api.*` calls are deliberately not awaited — the UI must not block on
 * a version string or a binary listing — but leaving the promise without a
 * handler meant a failure surfaced only as an anonymous "Uncaught (in promise)"
 * in the console, with nothing naming the call that failed. This labels it, so
 * the next report of "something failed on startup" can be acted on.
 */
export function reportFailure(label: string) {
  return (err: unknown) => {
    console.error(`[NexVPN] ${label} failed:`, err)
  }
}

export function formatBytes(bytes: number, decimals = 1): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]
}

export function formatSpeed(bytesPerSec: number): string {
  return `${formatBytes(bytesPerSec)}/s`
}

export function formatDuration(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  const pad = (n: number) => n.toString().padStart(2, '0')
  if (h > 0) return `${pad(h)}:${pad(m)}:${pad(s)}`
  return `${pad(m)}:${pad(s)}`
}

export function formatTime(isoString: string): string {
  try {
    const d = new Date(isoString)
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
  } catch {
    return isoString
  }
}

export function formatRelative(isoString: string): string {
  try {
    const d = new Date(isoString)
    const diff = Math.floor((Date.now() - d.getTime()) / 1000)
    if (diff < 60) return 'just now'
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
    return `${Math.floor(diff / 86400)}d ago`
  } catch {
    return isoString
  }
}

export function getProtocolName(p: Protocol): string {
  switch (p) {
    case 'wireguard': return 'WireGuard'
    case 'xray': return 'Xray-core'
    case 'singbox': return 'Sing-box'
    case 'openvpn': return 'OpenVPN'
    case 'hysteria2': return 'Hysteria 2'
    case 'warp': return 'Cloudflare WARP'
    default: return String(p)
  }
}

export function getProtocolEmoji(p: Protocol): string {
  switch (p) {
    case 'wireguard': return '⚡'
    case 'xray': return '🔮'
    case 'singbox': return '📦'
    case 'openvpn': return '🔒'
    case 'hysteria2': return '🚀'
    case 'warp': return '⚡'
    default: return '🌐'
  }
}

export function getProtocolColor(p: Protocol): string {
  switch (p) {
    case 'wireguard': return '#00ADD8'
    case 'xray': return '#7C3AED'
    case 'singbox': return '#10B981'
    case 'openvpn': return '#F59E0B'
    case 'hysteria2': return '#EC4899'
    case 'warp': return '#F97316'
    default: return '#3B82F6'
  }
}

const countryKeywords: Record<string, string> = {
  germany: 'DE', de: 'DE', frankfurt: 'DE', berlin: 'DE',
  france: 'FR', fr: 'FR', paris: 'FR',
  finland: 'FI', fi: 'FI', helsinki: 'FI',
  netherlands: 'NL', nl: 'NL', amsterdam: 'NL',
  united_states: 'US', usa: 'US', us: 'US', america: 'US', newyork: 'US', virginia: 'US',
  united_kingdom: 'GB', uk: 'GB', gb: 'GB', london: 'GB',
  turkey: 'TR', tr: 'TR', istanbul: 'TR',
  russia: 'RU', ru: 'RU', moscow: 'RU',
  singapore: 'SG', sg: 'SG',
  japan: 'JP', jp: 'JP', tokyo: 'JP',
  sweden: 'SE', se: 'SE', stockholm: 'SE',
  switzerland: 'CH', ch: 'CH', zurich: 'CH',
  italy: 'IT', it: 'IT', milan: 'IT', rome: 'IT',
  spain: 'ES', es: 'ES', madrid: 'ES',
  canada: 'CA', ca: 'CA', toronto: 'CA',
  uae: 'AE', ae: 'AE', dubai: 'AE',
  iran: 'IR', ir: 'IR', tehran: 'IR',
  poland: 'PL', pl: 'PL', warsaw: 'PL',
  romania: 'RO', ro: 'RO', bucharest: 'RO',
  austria: 'AT', at: 'AT', vienna: 'AT',
  norway: 'NO', no: 'NO', oslo: 'NO',
}

export function detectCountryFromText(text: string): string | undefined {
  if (!text) return undefined
  // First check if text already contains a unicode flag emoji
  const flagRegex = /\p{Regional_Indicator}{2}/u
  const match = text.match(flagRegex)
  if (match) return match[0]

  const lower = text.toLowerCase()
  for (const [key, code] of Object.entries(countryKeywords)) {
    // Exact word or bracketed match like [DE] or de-server or Germany
    const wordPattern = new RegExp(`\\b${key}\\b|[\\[\\(_-]${key}[\\]\\)_-\\d]`, 'i')
    if (wordPattern.test(lower) || lower.includes(key)) {
      return getCountryFlag(code)
    }
  }
  return undefined
}

export function getCountryFlag(countryCodeOrName?: string): string {
  if (!countryCodeOrName) return '🌐'
  const trimmed = countryCodeOrName.trim()

  // If it's already an emoji flag
  if (/\p{Regional_Indicator}{2}/u.test(trimmed)) {
    return trimmed
  }

  if (trimmed.length === 2) {
    const codePoints = trimmed
      .toUpperCase()
      .split('')
      .map(char => 127397 + char.charCodeAt(0))
    return String.fromCodePoint(...codePoints)
  }

  const detected = detectCountryFromText(trimmed)
  return detected || '🌐'
}

