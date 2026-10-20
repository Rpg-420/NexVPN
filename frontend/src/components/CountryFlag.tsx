import { Cloud, Shield, Link2, Globe } from 'lucide-react'
import { t } from '../lib/i18n'
import 'flag-icons/css/flag-icons.min.css'

// Comprehensive country, city, and datacenter gateway mapping
const COUNTRY_MAP: Record<string, string> = {
  // Germany
  germany: 'de', de: 'de', deu: 'de', frankfurt: 'de', fra: 'de', berlin: 'de',
  munich: 'de', muc: 'de', nuremberg: 'de', falkenstein: 'de', dusseldorf: 'de',
  آلمان: 'de', 'германия': 'de', 'deutschland': 'de',

  // Netherlands
  netherlands: 'nl', nl: 'nl', nld: 'nl', amsterdam: 'nl', ams: 'nl', rotterdam: 'nl',
  هلند: 'nl', 'нидерланды': 'nl',

  // United Kingdom
  united_kingdom: 'gb', uk: 'gb', gb: 'gb', gbr: 'gb', london: 'gb', lon: 'gb',
  manchester: 'gb', man: 'gb', انگلستان: 'gb', بریتانیا: 'gb', 'великобритания': 'gb',

  // United States
  united_states: 'us', usa: 'us', us: 'us', america: 'us',
  new_york: 'us', nyc: 'us', ashburn: 'us', iad: 'us', chicago: 'us', ord: 'us',
  los_angeles: 'us', lax: 'us', san_jose: 'us', sfo: 'us', seattle: 'us', sea: 'us',
  miami: 'us', mia: 'us', dallas: 'us', dfw: 'us', atlanta: 'us', atl: 'us',
  phoenix: 'us', phx: 'us', آمریکا: 'us', 'сша': 'us',

  // France
  france: 'fr', fr: 'fr', fra_fr: 'fr', paris: 'fr', cdg: 'fr', roubaix: 'fr',
  strasbourg: 'fr', marseille: 'fr', فرانسه: 'fr', 'франция': 'fr',

  // Finland
  finland: 'fi', fi: 'fi', fin: 'fi', helsinki: 'fi', hel: 'fi',
  فنلاند: 'fi', 'финляндия': 'fi',

  // Sweden
  sweden: 'se', se: 'se', swe: 'se', stockholm: 'se', arn: 'se',
  سوئد: 'se', 'швеция': 'se',

  // Switzerland
  switzerland: 'ch', ch: 'ch', che: 'ch', zurich: 'ch', zrh: 'ch', geneva: 'ch', gva: 'ch',
  سوئیس: 'ch', 'швейцария': 'ch',

  // Turkey
  turkey: 'tr', tr: 'tr', tur: 'tr', istanbul: 'tr', ist: 'tr', ankara: 'tr', izmir: 'tr',
  ترکیه: 'tr', 'турция': 'tr',

  // Russia
  russia: 'ru', ru: 'ru', rus: 'ru', moscow: 'ru', svo: 'ru', dme: 'ru',
  saint_petersburg: 'ru', led: 'ru', روسیه: 'ru', 'россия': 'ru',

  // Japan
  japan: 'jp', jp: 'jp', jpn: 'jp', tokyo: 'jp', nrt: 'jp', hnd: 'jp', osaka: 'jp', kix: 'jp',
  ژاپن: 'jp', 'япония': 'jp',

  // Singapore
  singapore: 'sg', sg: 'sg', sgp: 'sg', sin: 'sg',
  سنگاپور: 'sg', 'сингапур': 'sg',

  // South Korea
  korea: 'kr', kr: 'kr', kor: 'kr', seoul: 'kr', icn: 'kr',
  کره: 'kr', 'کره جنوبی': 'kr', 'корея': 'kr',

  // Canada
  canada: 'ca', ca: 'ca', can: 'ca', toronto: 'ca', yyz: 'ca',
  montreal: 'ca', yul: 'ca', vancouver: 'ca', yvr: 'ca',
  کانادا: 'ca', 'канада': 'ca',

  // UAE
  uae: 'ae', ae: 'ae', are: 'ae', dubai: 'ae', dxb: 'ae', abu_dhabi: 'ae', auh: 'ae',
  امارات: 'ae', 'دبی': 'ae', 'оаэ': 'ae',

  // Iran
  iran: 'ir', ir: 'ir', irn: 'ir', tehran: 'ir', thr: 'ir', isfahan: 'ir',
  shiraz: 'ir', tabriz: 'ir', mashhad: 'ir', ایران: 'ir', 'иран': 'ir',

  // Poland
  poland: 'pl', pl: 'pl', pol: 'pl', warsaw: 'pl', waw: 'pl', لهستان: 'pl', 'польشا': 'pl',

  // Austria
  austria: 'at', at: 'at', aut: 'at', vienna: 'at', vie: 'at', اتریش: 'at', 'австрия': 'at',

  // Italy
  italy: 'it', it: 'it', ita: 'it', milan: 'it', mxp: 'it', rome: 'it', fco: 'it',
  ایتالیا: 'it', 'италия': 'it',

  // Spain
  spain: 'es', es: 'es', esp: 'es', madrid: 'es', mad: 'es', barcelona: 'es', bcn: 'es',
  اسپانیا: 'es', 'испания': 'es',

  // Norway
  norway: 'no', no: 'no', nor: 'no', oslo: 'no', osl: 'no', نروژ: 'no', 'норвегия': 'no',

  // Denmark
  denmark: 'dk', dk: 'dk', dnk: 'dk', copenhagen: 'dk', cph: 'dk', دانمارک: 'dk',

  // Belgium
  belgium: 'be', be: 'be', bel: 'be', brussels: 'be', bru: 'be', بلژیک: 'be',

  // Romania
  romania: 'ro', ro: 'ro', rou: 'ro', bucharest: 'ro', otp: 'ro', رومانی: 'ro', 'румыния': 'ro',

  // Czech Republic
  czech: 'cz', czechia: 'cz', cz: 'cz', cze: 'cz', prague: 'cz', prg: 'cz', چک: 'cz',

  // Hungary
  hungary: 'hu', hu: 'hu', hun: 'hu', budapest: 'hu', bud: 'hu', مجارستان: 'hu',

  // Greece
  greece: 'gr', gr: 'gr', grc: 'gr', athens: 'gr', ath: 'gr', یونان: 'gr',

  // Portugal
  portugal: 'pt', pt: 'pt', prt: 'pt', lisbon: 'pt', lis: 'pt', پرتغال: 'pt',

  // Ireland
  ireland: 'ie', ie: 'ie', irl: 'ie', dublin: 'ie', dub: 'ie', ایرلند: 'ie',

  // Ukraine
  ukraine: 'ua', ua: 'ua', ukr: 'ua', kyiv: 'ua', iev: 'ua', اوکراین: 'ua',

  // Australia
  australia: 'au', au: 'au', aus: 'au', sydney: 'au', syd: 'au', melbourne: 'au', mel: 'au',
  استرالیا: 'au',

  // Brazil
  brazil: 'br', br: 'br', bra: 'br', sao_paulo: 'br', gru: 'br', rio: 'br', برزیل: 'br',

  // India
  india: 'in', in: 'in', ind: 'in', mumbai: 'in', bom: 'in', delhi: 'in', del: 'in',
  bangalore: 'in', blr: 'in', هند: 'in',

  // China / Hong Kong / Taiwan
  china: 'cn', cn: 'cn', chn: 'cn', beijing: 'cn', shanghai: 'cn', چین: 'cn',
  hong_kong: 'hk', hk: 'hk', hkg: 'hk', هنگ‌کنگ: 'hk',
  taiwan: 'tw', tw: 'tw', twn: 'tw', taipei: 'tw', tpe: 'tw', تایوان: 'tw',

  // Additional European Countries
  bulgaria: 'bg', bg: 'bg', bgr: 'bg', sofia: 'bg', بلغارستان: 'bg',
  croatia: 'hr', hr: 'hr', hrv: 'hr', zagreb: 'hr', کرواسی: 'hr',
  slovakia: 'sk', sk: 'sk', svk: 'sk', bratislava: 'sk', اسلواکی: 'sk',
  lithuania: 'lt', lt: 'lt', ltu: 'lt', vilnius: 'lt', لیتوانی: 'lt',
  latvia: 'lv', lv: 'lv', lva: 'lv', riga: 'lv', لتونی: 'lv',
  estonia: 'ee', ee: 'ee', est: 'ee', tallinn: 'ee', استونی: 'ee',
  iceland: 'is', is: 'is', isl: 'is', reykjavik: 'is', ایسلند: 'is',
  luxembourg: 'lu', lu: 'lu', lux: 'lu', لوکزامبورگ: 'lu',
  serbia: 'rs', rs: 'rs', srb: 'rs', belgrade: 'rs', صربستان: 'rs',
  albania: 'al', al: 'al', alb: 'al', tirana: 'al', آلبانی: 'al',
  cyprus: 'cy', cy: 'cy', cyp: 'cy', nicosia: 'cy', قبرس: 'cy',
  moldova: 'md', md: 'md', mda: 'md', chisinau: 'md', مولداوی: 'md',
  georgia: 'ge', ge: 'ge', geo: 'ge', tbilisi: 'ge', گرجستان: 'ge',
  armenia: 'am', am: 'am', arm: 'am', yerevan: 'am', ارمنستان: 'am',
  azerbaijan: 'az', az: 'az', aze: 'az', baku: 'az', آذربایجان: 'az',

  // Middle East & Central Asia
  israel: 'il', il: 'il', isr: 'il', tel_aviv: 'il', tlv: 'il', اسراییل: 'il',
  saudi_arabia: 'sa', sa: 'sa', sau: 'sa', riyadh: 'sa', ruh: 'sa', jeddah: 'sa', عربستان: 'sa',
  qatar: 'qa', qa: 'qa', qat: 'qa', doha: 'qa', doh: 'qa', قطر: 'qa',
  kuwait: 'kw', kw: 'kw', kwt: 'kw', kuwait_city: 'kw', کویت: 'kw',
  kazakhstan: 'kz', kz: 'kz', kaz: 'kz', almaty: 'kz', astana: 'kz', قزاقستان: 'kz',
  uzbekistan: 'uz', uz: 'uz', uzb: 'uz', tashkent: 'uz', ازبکستان: 'uz',
  iraq: 'iq', iq: 'iq', irq: 'iq', baghdad: 'iq', عراق: 'iq',

  // Africa & South America
  south_africa: 'za', za: 'za', zaf: 'za', johannesburg: 'za', jnb: 'za', cape_town: 'za', 'آفریقای جنوبی': 'za',
  egypt: 'eg', eg: 'eg', egy: 'eg', cairo: 'eg', cai: 'eg', مصر: 'eg',
  argentina: 'ar', ar: 'ar', arg: 'ar', buenos_aires: 'ar', eze: 'ar', آرژانتین: 'ar',
  chile: 'cl', cl: 'cl', chl: 'cl', santiago: 'cl', شیلی: 'cl',
  mexico: 'mx', mx: 'mx', mex: 'mx', mexico_city: 'mx', مکزیک: 'mx',
  new_zealand: 'nz', nz: 'nz', nzl: 'nz', auckland: 'nz', akl: 'nz', نیوزیلند: 'nz',

  // Special protocols
  warp: 'cf', cf: 'cf', cloudflare: 'cf', wow: 'cf',
  psiphon: 'shield', emergency: 'shield',
  chain: 'chain', relay: 'chain', 'multi-hop': 'chain',
  auto: 'un', global: 'un',
}

// Extract ISO from flag emojis or string patterns
export function resolveCountryISO(codeOrName?: string): string {
  if (!codeOrName) return 'un'
  const text = codeOrName.trim()

  // Extract from Regional Indicator Symbol emojis (e.g. 🇩🇪 -> de)
  const codePoints = Array.from(text).map(c => c.codePointAt(0) || 0)
  for (let i = 0; i < codePoints.length - 1; i++) {
    const c1 = codePoints[i]
    const c2 = codePoints[i + 1]
    if (c1 >= 0x1f1e6 && c1 <= 0x1f1ff && c2 >= 0x1f1e6 && c2 <= 0x1f1ff) {
      const char1 = String.fromCharCode(c1 - 0x1f1e6 + 65)
      const char2 = String.fromCharCode(c2 - 0x1f1e6 + 65)
      return `${char1}${char2}`.toLowerCase()
    }
  }

  // Bracket match: [DE], (US), | AL |, DE - ...
  const bracketMatch = text.match(/[\[\(\|\s]([a-zA-Z]{2})[\]\)\|\s:-]/)
  if (bracketMatch && COUNTRY_MAP[bracketMatch[1].toLowerCase()]) {
    return bracketMatch[1].toLowerCase()
  }

  const clean = text.toLowerCase().trim()
  if (COUNTRY_MAP[clean]) return COUNTRY_MAP[clean]

  // Check prefix or contains
  for (const [key, val] of Object.entries(COUNTRY_MAP)) {
    if (clean.includes(key)) return val
  }

  if (clean.length === 2 && /^[a-z]{2}$/.test(clean)) return clean

  return 'un'
}

export function cleanServerName(name?: string): string {
  if (!name) return ''
  let clean = name.replace(/\p{Regional_Indicator}{2}/gu, '')
  clean = clean.replace(/[\u{1F300}-\u{1F9FF}]/gu, '')
  clean = clean.replace(/^[[\[\(\|\s]*[a-zA-Z]{2}[\]\)\|\s:-]+/, '')
  return clean.trim() || name
}

export function getCountryDisplayName(iso: string, _lang?: 'fa' | 'en'): string {
  const key = `country_${iso.toLowerCase()}`
  const translated = t(key)
  if (translated !== key) return translated
  return iso.toUpperCase()
}

export interface CountryFlagProps {
  codeOrName?: string
  className?: string
  size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl'
  circular?: boolean
}

export default function CountryFlag({
  codeOrName,
  className = '',
  size = 'md',
  circular = true,
}: CountryFlagProps) {
  const iso = resolveCountryISO(codeOrName)

  // Standard calibrated dimensions
  const sizeMap = {
    xs: {
      wh: 'w-[18px] h-[18px] min-w-[18px]',
      rect: 'w-[20px] h-[14px] min-w-[20px]',
      icon: 10,
    },
    sm: {
      wh: 'w-[22px] h-[22px] min-w-[22px]',
      rect: 'w-[24px] h-[16px] min-w-[24px]',
      icon: 12,
    },
    md: {
      wh: 'w-[28px] h-[28px] min-w-[28px]',
      rect: 'w-[30px] h-[20px] min-w-[30px]',
      icon: 15,
    },
    lg: {
      wh: 'w-[36px] h-[36px] min-w-[36px]',
      rect: 'w-[38px] h-[25px] min-w-[38px]',
      icon: 18,
    },
    xl: {
      wh: 'w-[48px] h-[48px] min-w-[48px]',
      rect: 'w-[52px] h-[34px] min-w-[52px]',
      icon: 24,
    },
  }

  const currentSize = sizeMap[size] || sizeMap.md
  const dimClass = circular ? currentSize.wh : currentSize.rect
  const shapeClass = circular ? 'rounded-full aspect-square' : 'rounded-[5px]'
  const tooltip = `${getCountryDisplayName(iso, 'fa')} (${iso.toUpperCase()})`

  // Protocol / Special Custom Circular Badges
  if (iso === 'cf') {
    return (
      <span
        className={`relative inline-flex items-center justify-center shrink-0 overflow-hidden ${dimClass} ${shapeClass} bg-warning/30 ring-1 ring-warning/50 text-warning shadow-[0_2px_8px_rgba(245,158,11,0.25)] transition-transform duration-200 hover:scale-110 ${className}`}
        title="Cloudflare WARP"
      >
        <Cloud size={currentSize.icon} className="relative z-10 text-warning" />
        <span className="absolute inset-0 bg-gradient-to-tr from-transparent via-white/10 to-white/35 pointer-events-none" />
      </span>
    )
  }

  if (iso === 'shield') {
    return (
      <span
        className={`relative inline-flex items-center justify-center shrink-0 overflow-hidden ${dimClass} ${shapeClass} bg-success/30 ring-1 ring-success/50 text-success shadow-[0_2px_8px_rgba(16,185,129,0.25)] transition-transform duration-200 hover:scale-110 ${className}`}
        title="Psiphon Emergency"
      >
        <Shield size={currentSize.icon} className="relative z-10 text-success" />
        <span className="absolute inset-0 bg-gradient-to-tr from-transparent via-white/10 to-white/35 pointer-events-none" />
      </span>
    )
  }

  if (iso === 'chain') {
    return (
      <span
        className={`relative inline-flex items-center justify-center shrink-0 overflow-hidden ${dimClass} ${shapeClass} bg-primary/30 ring-1 ring-primary/50 text-primary shadow-[0_2px_8px_rgba(168,85,247,0.25)] transition-transform duration-200 hover:scale-110 ${className}`}
        title="Multi-hop Proxy Chain"
      >
        <Link2 size={currentSize.icon} className="relative z-10 text-primary" />
        <span className="absolute inset-0 bg-gradient-to-tr from-transparent via-white/10 to-white/35 pointer-events-none" />
      </span>
    )
  }

  if (iso === 'un') {
    return (
      <span
        className={`relative inline-flex items-center justify-center shrink-0 overflow-hidden ${dimClass} ${shapeClass} bg-card ring-1 ring-border text-primary transition-transform duration-200 hover:scale-110 ${className}`}
        title={tooltip}
      >
        <Globe size={currentSize.icon} className="relative z-10 text-primary" />
        <span className="absolute inset-0 bg-gradient-to-tr from-transparent via-white/10 to-white/30 pointer-events-none" />
      </span>
    )
  }

  return (
    <span
      className={`relative inline-flex items-center justify-center shrink-0 overflow-hidden ${dimClass} ${shapeClass} ring-1 ring-white/20 bg-card shadow-[0_2px_8px_rgba(0,0,0,0.4)] transition-all duration-200 hover:scale-110 hover:ring-primary/60 ${className}`}
      title={tooltip}
    >
      {/* 
        Official 1x1 vector flag from flag-icons:
        - `fi.fis.fi-${iso}` targets 1x1 SVG with clean square aspect ratio
        - `!w-full !h-full !bg-cover !bg-center` ensures circular clipping without black letterboxing
      */}
      <span
        aria-label={iso.toUpperCase()}
        className={`fi fis fi-${iso} !w-full !h-full !bg-cover !bg-center block pointer-events-none ${shapeClass}`}
      />

      {/* 3D Glass / Specular overlay matching Flaticon circular flags */}
      <span className="absolute inset-0 rounded-full bg-gradient-to-tr from-transparent via-white/5 to-white/35 pointer-events-none ring-1 ring-inset ring-black/20" />
    </span>
  )
}
