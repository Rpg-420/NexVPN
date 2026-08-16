import React, { useState } from 'react'

interface Props {
  codeOrName?: string
  className?: string
  size?: 'sm' | 'md' | 'lg'
}

const COUNTRY_MAP: Record<string, string> = {
  germany: 'de', de: 'de', frankfurt: 'de', berlin: 'de',
  france: 'fr', fr: 'fr', paris: 'fr',
  finland: 'fi', fi: 'fi', helsinki: 'fi',
  netherlands: 'nl', nl: 'nl', amsterdam: 'nl',
  united_states: 'us', usa: 'us', us: 'us', america: 'us',
  united_kingdom: 'gb', uk: 'gb', gb: 'gb', london: 'gb',
  turkey: 'tr', tr: 'tr', istanbul: 'tr',
  russia: 'ru', ru: 'ru', moscow: 'ru',
  singapore: 'sg', sg: 'sg',
  japan: 'jp', jp: 'jp', tokyo: 'jp',
  sweden: 'se', se: 'se', stockholm: 'se',
  switzerland: 'ch', ch: 'ch', zurich: 'ch',
  italy: 'it', it: 'it', milan: 'it', rome: 'it',
  spain: 'es', es: 'es', madrid: 'es',
  canada: 'ca', ca: 'ca', toronto: 'ca',
  uae: 'ae', ae: 'ae', dubai: 'ae',
  iran: 'ir', ir: 'ir', tehran: 'ir',
  poland: 'pl', pl: 'pl', warsaw: 'pl',
  romania: 'ro', ro: 'ro', bucharest: 'ro',
  austria: 'at', at: 'at', vienna: 'at',
  norway: 'no', no: 'no', oslo: 'no',
  azerbaijan: 'az', az: 'az', baku: 'az',
}

export default function CountryFlag({ codeOrName, className = '', size = 'md' }: Props) {
  const [hasError, setHasError] = useState(false)

  if (!codeOrName) {
    return <span className="inline-block text-base leading-none">🌐</span>
  }

  const clean = codeOrName.toLowerCase().trim()
  let iso = COUNTRY_MAP[clean]

  if (!iso) {
    for (const [key, val] of Object.entries(COUNTRY_MAP)) {
      if (clean.includes(key)) {
        iso = val
        break
      }
    }
  }

  if (!iso && clean.length === 2) {
    iso = clean
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
