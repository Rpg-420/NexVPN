import { afterEach, describe, expect, it } from 'vitest'

import { translateBackendError } from './backendErrors'
import { getLanguage, setLanguage } from './i18n'

afterEach(() => {
  // Leave the global language exactly as the suite found it.
  setLanguage(getLanguage())
})

describe('translateBackendError', () => {
  it('returns an empty string for empty input', () => {
    expect(translateBackendError('')).toBe('')
    expect(translateBackendError(null)).toBe('')
    expect(translateBackendError(undefined)).toBe('')
    expect(translateBackendError('   ')).toBe('')
  })

  it('translates known backend prefixes instead of leaking English', () => {
    // The point of the module: a Persian UI must not show these English
    // backend strings verbatim.
    const original = getLanguage()
    setLanguage('fa')
    try {
      const translated = translateBackendError('Core process crashed: exit status 2')
      expect(translated).not.toBe('Core process crashed: exit status 2')
      // Never show the raw i18n key either.
      expect(translated).not.toBe('err_core_crashed')
      expect(translated.length).toBeGreaterThan(0)
    } finally {
      setLanguage(original)
    }
  })

  it('matches by stable prefix while keeping the dynamic tail out of the match', () => {
    const a = translateBackendError('Connection failed on Xray: boom')
    const b = translateBackendError('Connection failed on Sing-box: other')
    expect(a).toBe(b)
  })

  it('passes unknown messages through untouched', () => {
    expect(translateBackendError('some novel backend error')).toBe('some novel backend error')
  })

  it('trims surrounding whitespace', () => {
    expect(translateBackendError('  padded message  ')).toBe('padded message')
  })
})
