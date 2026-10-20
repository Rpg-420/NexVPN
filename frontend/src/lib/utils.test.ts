import { describe, expect, it } from 'vitest'

import {
  cn,
  detectCountryFromText,
  formatBytes,
  formatDuration,
  formatSpeed,
  getProtocolColor,
  getProtocolName,
} from './utils'

describe('cn', () => {
  it('merges classes and resolves Tailwind conflicts (later wins)', () => {
    expect(cn('p-2', 'p-4')).toBe('p-4')
    expect(cn('text-red-500', false && 'hidden', 'text-blue-500')).toBe('text-blue-500')
  })
})

describe('formatBytes', () => {
  it('formats zero and plain bytes', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(512)).toBe('512 B')
  })

  it('scales through KB/MB/GB', () => {
    expect(formatBytes(1024)).toBe('1 KB')
    expect(formatBytes(1024 * 1024)).toBe('1 MB')
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1 GB')
  })

  it('honours the decimals argument', () => {
    expect(formatBytes(1536, 0)).toBe('2 KB')
    // parseFloat drops the trailing zero: 1.50 -> 1.5. The value, not the
    // padding, is what matters here.
    expect(formatBytes(1536, 2)).toBe('1.5 KB')
  })

  it('treats a negative decimals argument as zero', () => {
    expect(formatBytes(1536, -1)).toBe('2 KB')
  })
})

describe('formatSpeed', () => {
  it('appends the per-second suffix', () => {
    expect(formatSpeed(0)).toBe('0 B/s')
    expect(formatSpeed(2048)).toBe('2 KB/s')
  })
})

describe('formatDuration', () => {
  it('renders mm:ss below an hour and hh:mm:ss above', () => {
    expect(formatDuration(45)).toBe('00:45')
    expect(formatDuration(60)).toBe('01:00')
    expect(formatDuration(3661)).toBe('01:01:01')
  })
})

describe('getProtocolName', () => {
  it('maps known protocols to human names', () => {
    expect(getProtocolName('vless' as never)).toBeTruthy()
    expect(getProtocolName('wireguard' as never)).toBeTruthy()
  })

  it('never returns an empty string for an unknown protocol', () => {
    // The name is displayed in the UI, so a blank fallback is a visible bug.
    expect(getProtocolName('totally-unknown' as never)).not.toBe('')
  })
})

describe('getProtocolColor', () => {
  it('returns a CSS color for known and unknown protocols', () => {
    for (const p of ['vless', 'vmess', 'trojan', 'shadowsocks', 'wireguard', 'unknown-x']) {
      expect(getProtocolColor(p as never)).toMatch(/^#|^rgb|^hsl|var\(/)
    }
  })
})

describe('detectCountryFromText', () => {
  it('returns an existing flag emoji unchanged', () => {
    expect(detectCountryFromText('🇩🇪 Frankfurt #3')).toBe('🇩🇪')
  })

  it('derives the flag from a bracketed or dashed country code', () => {
    expect(detectCountryFromText('[DE] Frankfurt')).toBe('🇩🇪')
    expect(detectCountryFromText('DE-Frankfurt')).toBe('🇩🇪')
  })

  it('returns undefined for empty input', () => {
    expect(detectCountryFromText('')).toBeUndefined()
  })
})
