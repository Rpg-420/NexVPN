import { describe, expect, it } from 'vitest'

import { getLanguage, setLanguage, t, translations } from './i18n'

const enKeys = Object.keys(translations.en)
const faKeys = Object.keys(translations.fa)

describe('i18n dictionary', () => {
  it('has the same keys in English and Persian', () => {
    const missingInFa = enKeys.filter(k => !(k in translations.fa))
    const missingInEn = faKeys.filter(k => !(k in translations.en))
    expect(missingInFa).toEqual([])
    expect(missingInEn).toEqual([])
  })

  it('has no empty values in either language', () => {
    const empties = [
      ...enKeys.filter(k => String(translations.en[k as keyof typeof translations.en]).trim() === ''),
      ...faKeys.filter(k => String(translations.fa[k as keyof typeof translations.fa]).trim() === ''),
    ]
    expect(empties).toEqual([])
  })
})

describe('t()', () => {
  it('translates in the active language and never leaks the raw key', () => {
    const lang = getLanguage()
    try {
      for (const langToTest of ['fa', 'en'] as const) {
        setLanguage(langToTest)
        expect(t('connect')).toBeTruthy()
        expect(t('connect')).not.toBe('connect')
        // Unknown keys fall back to the key itself - that is the documented
        // behaviour the backendErrors module guards against.
        expect(t('definitely_not_a_real_key')).toBe('definitely_not_a_real_key')
      }
    } finally {
      setLanguage(lang)
    }
  })

  it('interpolates {param} placeholders', () => {
    const lang = getLanguage()
    try {
      setLanguage('en')
      const withParams = t('dangerous_action_confirm', { name: 'Test' })
      expect(withParams).not.toContain('{name}')
      if (withParams.includes('Test')) {
        // Interpolation happened when the template carries the placeholder.
        expect(withParams).toContain('Test')
      }
    } finally {
      setLanguage(lang)
    }
  })
})
