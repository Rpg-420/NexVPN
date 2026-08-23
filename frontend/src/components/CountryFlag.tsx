import React, { useState } from 'react'

interface Props {
  codeOrName?: string
  className?: string
  size?: 'sm' | 'md' | 'lg'
}

const COUNTRY_MAP: Record<string, string> = {
  germany: 'de', de: 'de', frankfurt: 'de', berlin: 'de', آلمان: 'de', 'германия': 'de',
  france: 'fr', fr: 'fr', paris: 'fr', فرانسه: 'fr', 'франция': 'fr',
  finland: 'fi', fi: 'fi', helsinki: 'fi', فنلاند: 'fi', 'финляндия': 'fi',
  netherlands: 'nl', nl: 'nl', amsterdam: 'nl', هلند: 'nl', 'нидерланды': 'nl',
  united_states: 'us', usa: 'us', us: 'us', america: 'us', آمریکا: 'us', 'сша': 'us',
  united_kingdom: 'gb', uk: 'gb', gb: 'gb', london: 'gb', انگلستان: 'gb', 'великобритания': 'gb',
  turkey: 'tr', tr: 'tr', istanbul: 'tr', ترکیه: 'tr', 'турция': 'tr',
  russia: 'ru', ru: 'ru', moscow: 'ru', روسیه: 'ru', 'россия': 'ru',
  singapore: 'sg', sg: 'sg', سنگاپور: 'sg', 'сингапур': 'sg',
  japan: 'jp', jp: 'jp', tokyo: 'jp', ژاپن: 'jp', 'япония': 'jp',
  sweden: 'se', se: 'se', stockholm: 'se', سوئد: 'se', 'швеция': 'se',
  switzerland: 'ch', ch: 'ch', zurich: 'ch', سوئیس: 'ch', 'швейцария': 'ch',
  italy: 'it', it: 'it', milan: 'it', rome: 'it', ایتالیا: 'it', 'италия': 'it',
  spain: 'es', es: 'es', madrid: 'es', اسپانیا: 'es', 'испания': 'es',
  canada: 'ca', ca: 'ca', toronto: 'ca', کانادا: 'ca', 'канада': 'ca',
  uae: 'ae', ae: 'ae', dubai: 'ae', امارات: 'ae', 'оаэ': 'ae',
  iran: 'ir', ir: 'ir', tehran: 'ir', ایران: 'ir', 'иран': 'ir',
  poland: 'pl', pl: 'pl', warsaw: 'pl', لهستان: 'pl', 'польша': 'pl',
  romania: 'ro', ro: 'ro', bucharest: 'ro', رومانی: 'ro', 'румыния': 'ro',
  austria: 'at', at: 'at', vienna: 'at', اتریش: 'at', 'австрия': 'at',
  norway: 'no', no: 'no', oslo: 'no', نروژ: 'no', 'норвегия': 'no',
  azerbaijan: 'az', az: 'az', baku: 'az', آذربایجان: 'az',
  korea: 'kr', kr: 'kr', seoul: 'kr', کره: 'kr',
  australia: 'au', au: 'au', sydney: 'au', استرالیا: 'au',
  brazil: 'br', br: 'br', برزیل: 'br',
  india: 'in', in: 'in', mumbai: 'in', هند: 'in',
  china: 'cn', cn: 'cn', beijing: 'cn', چین: 'cn',
  hong_kong: 'hk', hk: 'hk', هنگ‌کنگ: 'hk',
}

// Helper to extract ISO code from emoji flag pairs (Regional Indicator Symbols)
function extractFlagEmojiISO(text: string): string | null {
  if (!text) return null
  const codePoints = Array.from(text).map(c => c.codePointAt(0) || 0)
  for (let i = 0; i < codePoints.length - 1; i++) {
    const c1 = codePoints[i]
    const c2 = codePoints[i + 1]
    // Regional Indicator Symbol range: 0x1F1E6 (A) to 0x1F1FF (Z)
    if (c1 >= 0x1f1e6 && c1 <= 0x1f1ff && c2 >= 0x1f1e6 && c2 <= 0x1f1ff) {
      const char1 = String.fromCharCode(c1 - 0x1f1e6 + 65)
      const char2 = String.fromCharCode(c2 - 0x1f1e6 + 65)
      return `${char1}${char2}`.toLowerCase()
    }
  }
  return null
}

export default function CountryFlag({ codeOrName, className = '', size = 'md' }: Props) {
  const [hasError, setHasError] = useState(false)

  if (!codeOrName) {
    return <span className="inline-block text-base leading-none">🌐</span>
  }

  // 1. Try extracting from flag emoji
  let iso = extractFlagEmojiISO(codeOrName)

  // 2. Try bracket ISO code like [DE], [TR], (US)
  if (!iso) {
    const bracketMatch = codeOrName.match(/[\[\(]([a-zA-Z]{2})[\]\)]/)
    if (bracketMatch) {
      iso = bracketMatch[1].toLowerCase()
    }
  }

  // 3. Try lookup in dictionary
  if (!iso) {
    const clean = codeOrName.toLowerCase().trim()
    iso = COUNTRY_MAP[clean]

    if (!iso) {
      for (const [key, val] of Object.entries(COUNTRY_MAP)) {
        if (clean.includes(key)) {
          iso = val
          break
        }
      }
    }

    if (!iso && clean.length === 2 && /^[a-z]{2}$/.test(clean)) {
      iso = clean
    }
  }

  if (!iso || hasError) {
    return <span className="inline-block text-base leading-none">🌐</span>
  }

  const dimClasses =
    size === 'sm'
      ? 'w-4 h-3 rounded-[2px]'
      : size === 'lg'
      ? 'w-7 h-5 rounded-[4px]'
      : 'w-5 h-3.5 rounded-[3px]'

  return (
    <img
      src={`https://flagcdn.com/40x30/${iso}.png`}
      alt={iso.toUpperCase()}
      loading="lazy"
      onError={() => setHasError(true)}
      className={`inline-block object-cover shadow-sm ring-1 ring-white/10 shrink-0 ${dimClasses} ${className}`}
    />
  )
}
