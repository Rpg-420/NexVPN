import { useEffect, useRef, useMemo, useState, useCallback, lazy, Suspense } from 'react'
import type { GlobeMethods } from 'react-globe.gl'
import * as THREE from 'three'
import CountryFlag, { resolveCountryISO } from './CountryFlag'
import { t } from '../lib/i18n'
import { isDarkTheme, readGlobePalette } from '../lib/theme'
import { cn } from '../lib/utils'
import { Crosshair, RotateCw } from 'lucide-react'

// Lazy load 3D globe for optimal initial bundle performance
const Globe = lazy(() => import('react-globe.gl'))

interface GlobeWidgetProps {
  connected: boolean
  serverIp?: string
  serverCountry?: string
  serverName?: string
  /**
   * The client's own location, from the live IP lookup. Without it the globe has
   * nothing honest to show before a tunnel exists and used to fall back to a
   * hardcoded Iran/Tehran node — so anyone outside Iran saw the wrong flag and
   * city until a server supplied real data.
   */
  clientCountry?: string
  clientIp?: string
}

export interface LocationInfo {
  coords: [number, number] // [lat, lng]
  nameKey: string
  cityName: string
  iso: string
}

// Major World Datacenter Gateways & Capital Cities Database
const CITY_MAP: Record<string, LocationInfo> = {
  // Germany
  frankfurt: { coords: [50.1109, 8.6821], nameKey: 'country_de', cityName: 'Frankfurt (DE-CIX)', iso: 'de' },
  berlin: { coords: [52.5200, 13.4050], nameKey: 'country_de', cityName: 'Berlin', iso: 'de' },
  munich: { coords: [48.1351, 11.5820], nameKey: 'country_de', cityName: 'Munich', iso: 'de' },
  nuremberg: { coords: [49.4521, 11.0767], nameKey: 'country_de', cityName: 'Nuremberg', iso: 'de' },
  falkenstein: { coords: [50.4789, 12.3683], nameKey: 'country_de', cityName: 'Falkenstein', iso: 'de' },
  dusseldorf: { coords: [51.2277, 6.7735], nameKey: 'country_de', cityName: 'Düsseldorf', iso: 'de' },

  // Netherlands
  amsterdam: { coords: [52.3676, 4.9041], nameKey: 'country_nl', cityName: 'Amsterdam (AMS-IX)', iso: 'nl' },
  rotterdam: { coords: [51.9244, 4.4777], nameKey: 'country_nl', cityName: 'Rotterdam', iso: 'nl' },

  // United Kingdom
  london: { coords: [51.5074, -0.1278], nameKey: 'country_gb', cityName: 'London (LINX)', iso: 'gb' },
  manchester: { coords: [53.4808, -2.2426], nameKey: 'country_gb', cityName: 'Manchester', iso: 'gb' },

  // France
  paris: { coords: [48.8566, 2.3522], nameKey: 'country_fr', cityName: 'Paris', iso: 'fr' },
  roubaix: { coords: [50.6927, 3.1778], nameKey: 'country_fr', cityName: 'Roubaix (OVH)', iso: 'fr' },
  strasbourg: { coords: [48.5734, 7.7521], nameKey: 'country_fr', cityName: 'Strasbourg', iso: 'fr' },

  // Finland & Nordics
  helsinki: { coords: [60.1699, 24.9384], nameKey: 'country_fi', cityName: 'Helsinki', iso: 'fi' },
  stockholm: { coords: [59.3293, 18.0686], nameKey: 'country_se', cityName: 'Stockholm', iso: 'se' },
  oslo: { coords: [59.9139, 10.7522], nameKey: 'country_no', cityName: 'Oslo', iso: 'no' },
  copenhagen: { coords: [55.6761, 12.5683], nameKey: 'country_dk', cityName: 'Copenhagen', iso: 'dk' },
  reykjavik: { coords: [64.1466, -21.9426], nameKey: 'country_is', cityName: 'Reykjavik', iso: 'is' },

  // Central Europe
  zurich: { coords: [47.3769, 8.5417], nameKey: 'country_ch', cityName: 'Zurich', iso: 'ch' },
  geneva: { coords: [46.2044, 6.1432], nameKey: 'country_ch', cityName: 'Geneva', iso: 'ch' },
  vienna: { coords: [48.2082, 16.3738], nameKey: 'country_at', cityName: 'Vienna', iso: 'at' },
  warsaw: { coords: [52.2297, 21.0122], nameKey: 'country_pl', cityName: 'Warsaw', iso: 'pl' },
  prague: { coords: [50.0755, 14.4378], nameKey: 'country_cz', cityName: 'Prague', iso: 'cz' },
  budapest: { coords: [47.4979, 19.0402], nameKey: 'country_hu', cityName: 'Budapest', iso: 'hu' },
  brussels: { coords: [50.8503, 4.3517], nameKey: 'country_be', cityName: 'Brussels', iso: 'be' },
  luxembourg: { coords: [49.6116, 6.1319], nameKey: 'country_lu', cityName: 'Luxembourg', iso: 'lu' },

  // Southern Europe
  milan: { coords: [45.4642, 9.1900], nameKey: 'country_it', cityName: 'Milan', iso: 'it' },
  rome: { coords: [41.9028, 12.4964], nameKey: 'country_it', cityName: 'Rome', iso: 'it' },
  madrid: { coords: [40.4168, -3.7038], nameKey: 'country_es', cityName: 'Madrid', iso: 'es' },
  barcelona: { coords: [41.3851, 2.1734], nameKey: 'country_es', cityName: 'Barcelona', iso: 'es' },
  lisbon: { coords: [38.7223, -9.1393], nameKey: 'country_pt', cityName: 'Lisbon', iso: 'pt' },
  athens: { coords: [37.9838, 23.7275], nameKey: 'country_gr', cityName: 'Athens', iso: 'gr' },

  // Eastern Europe
  bucharest: { coords: [44.4268, 26.1025], nameKey: 'country_ro', cityName: 'Bucharest', iso: 'ro' },
  sofia: { coords: [42.6977, 23.3219], nameKey: 'country_bg', cityName: 'Sofia', iso: 'bg' },
  kyiv: { coords: [50.4501, 30.5234], nameKey: 'country_ua', cityName: 'Kyiv', iso: 'ua' },
  moscow: { coords: [55.7558, 37.6173], nameKey: 'country_ru', cityName: 'Moscow', iso: 'ru' },
  saint_petersburg: { coords: [59.9343, 30.3351], nameKey: 'country_ru', cityName: 'St. Petersburg', iso: 'ru' },
  vilnius: { coords: [54.6872, 25.2797], nameKey: 'country_lt', cityName: 'Vilnius', iso: 'lt' },
  riga: { coords: [56.9496, 24.1052], nameKey: 'country_lv', cityName: 'Riga', iso: 'lv' },
  tallinn: { coords: [59.4370, 24.7535], nameKey: 'country_ee', cityName: 'Tallinn', iso: 'ee' },

  // Middle East & Caucasus
  istanbul: { coords: [41.0082, 28.9784], nameKey: 'country_tr', cityName: 'Istanbul', iso: 'tr' },
  ankara: { coords: [39.9334, 32.8597], nameKey: 'country_tr', cityName: 'Ankara', iso: 'tr' },
  dubai: { coords: [25.2048, 55.2708], nameKey: 'country_ae', cityName: 'Dubai', iso: 'ae' },
  abu_dhabi: { coords: [24.4539, 54.3773], nameKey: 'country_ae', cityName: 'Abu Dhabi', iso: 'ae' },
  doha: { coords: [25.2854, 51.5310], nameKey: 'country_qa', cityName: 'Doha', iso: 'qa' },
  riyadh: { coords: [24.7136, 46.6753], nameKey: 'country_sa', cityName: 'Riyadh', iso: 'sa' },
  tel_aviv: { coords: [32.0853, 34.7818], nameKey: 'country_il', cityName: 'Tel Aviv', iso: 'il' },
  baku: { coords: [40.4093, 49.8671], nameKey: 'country_az', cityName: 'Baku', iso: 'az' },
  yerevan: { coords: [40.1792, 44.4991], nameKey: 'country_am', cityName: 'Yerevan', iso: 'am' },
  tbilisi: { coords: [41.7151, 44.8271], nameKey: 'country_ge', cityName: 'Tbilisi', iso: 'ge' },

  // Asia Pacific
  tokyo: { coords: [35.6762, 139.6503], nameKey: 'country_jp', cityName: 'Tokyo', iso: 'jp' },
  osaka: { coords: [34.6937, 135.5023], nameKey: 'country_jp', cityName: 'Osaka', iso: 'jp' },
  seoul: { coords: [37.5665, 126.9780], nameKey: 'country_kr', cityName: 'Seoul', iso: 'kr' },
  singapore: { coords: [1.3521, 103.8198], nameKey: 'country_sg', cityName: 'Singapore', iso: 'sg' },
  hong_kong: { coords: [22.3193, 114.1694], nameKey: 'country_hk', cityName: 'Hong Kong, China', iso: 'hk' },
  taipei: { coords: [25.0330, 121.5654], nameKey: 'country_tw', cityName: 'Taipei, China', iso: 'tw' },
  mumbai: { coords: [19.0760, 72.8777], nameKey: 'country_in', cityName: 'Mumbai', iso: 'in' },
  delhi: { coords: [28.6139, 77.2090], nameKey: 'country_in', cityName: 'Delhi', iso: 'in' },
  bangalore: { coords: [12.9716, 77.5946], nameKey: 'country_in', cityName: 'Bangalore', iso: 'in' },
  sydney: { coords: [-33.8688, 151.2093], nameKey: 'country_au', cityName: 'Sydney', iso: 'au' },
  melbourne: { coords: [-37.8136, 144.9631], nameKey: 'country_au', cityName: 'Melbourne', iso: 'au' },

  // North America
  new_york: { coords: [40.7128, -74.0060], nameKey: 'country_us', cityName: 'New York', iso: 'us' },
  ashburn: { coords: [39.0438, -77.4874], nameKey: 'country_us', cityName: 'Ashburn (DC)', iso: 'us' },
  los_angeles: { coords: [34.0522, -118.2437], nameKey: 'country_us', cityName: 'Los Angeles', iso: 'us' },
  chicago: { coords: [41.8781, -87.6298], nameKey: 'country_us', cityName: 'Chicago', iso: 'us' },
  miami: { coords: [25.7617, -80.1918], nameKey: 'country_us', cityName: 'Miami', iso: 'us' },
  dallas: { coords: [32.7767, -96.7970], nameKey: 'country_us', cityName: 'Dallas', iso: 'us' },
  seattle: { coords: [47.6062, -122.3321], nameKey: 'country_us', cityName: 'Seattle', iso: 'us' },
  san_jose: { coords: [37.3382, -121.8863], nameKey: 'country_us', cityName: 'Silicon Valley', iso: 'us' },
  toronto: { coords: [43.6532, -79.3832], nameKey: 'country_ca', cityName: 'Toronto', iso: 'ca' },
  montreal: { coords: [45.5017, -73.5673], nameKey: 'country_ca', cityName: 'Montreal', iso: 'ca' },
  vancouver: { coords: [49.2827, -123.1207], nameKey: 'country_ca', cityName: 'Vancouver', iso: 'ca' },

  // South America & Africa
  sao_paulo: { coords: [-23.5505, -46.6333], nameKey: 'country_br', cityName: 'São Paulo', iso: 'br' },
  johannesburg: { coords: [-26.2041, 28.0473], nameKey: 'country_za', cityName: 'Johannesburg', iso: 'za' },
  cairo: { coords: [30.0444, 31.2357], nameKey: 'country_eg', cityName: 'Cairo', iso: 'eg' },

  // Client Node
  tehran: { coords: [35.6892, 51.3890], nameKey: 'country_ir', cityName: 'Tehran (Client)', iso: 'ir' },
}

// Country Capital / Core Datacenter Coordinates for All ISOs
const COUNTRY_COORDINATES: Record<string, LocationInfo> = {
  DE: { coords: [50.1109, 8.6821], nameKey: 'country_de', cityName: 'Frankfurt', iso: 'de' },
  NL: { coords: [52.3676, 4.9041], nameKey: 'country_nl', cityName: 'Amsterdam', iso: 'nl' },
  GB: { coords: [51.5074, -0.1278], nameKey: 'country_gb', cityName: 'London', iso: 'gb' },
  US: { coords: [39.0438, -77.4874], nameKey: 'country_us', cityName: 'Ashburn / DC', iso: 'us' },
  FR: { coords: [48.8566, 2.3522], nameKey: 'country_fr', cityName: 'Paris', iso: 'fr' },
  FI: { coords: [60.1699, 24.9384], nameKey: 'country_fi', cityName: 'Helsinki', iso: 'fi' },
  SE: { coords: [59.3293, 18.0686], nameKey: 'country_se', cityName: 'Stockholm', iso: 'se' },
  CH: { coords: [47.3769, 8.5417], nameKey: 'country_ch', cityName: 'Zurich', iso: 'ch' },
  TR: { coords: [41.0082, 28.9784], nameKey: 'country_tr', cityName: 'Istanbul', iso: 'tr' },
  RU: { coords: [55.7558, 37.6173], nameKey: 'country_ru', cityName: 'Moscow', iso: 'ru' },
  JP: { coords: [35.6762, 139.6503], nameKey: 'country_jp', cityName: 'Tokyo', iso: 'jp' },
  SG: { coords: [1.3521, 103.8198], nameKey: 'country_sg', cityName: 'Singapore', iso: 'sg' },
  KR: { coords: [37.5665, 126.9780], nameKey: 'country_kr', cityName: 'Seoul', iso: 'kr' },
  CA: { coords: [43.6532, -79.3832], nameKey: 'country_ca', cityName: 'Toronto', iso: 'ca' },
  AE: { coords: [25.2048, 55.2708], nameKey: 'country_ae', cityName: 'Dubai', iso: 'ae' },
  IR: { coords: [35.6892, 51.3890], nameKey: 'country_ir', cityName: 'Tehran', iso: 'ir' },
  PL: { coords: [52.2297, 21.0122], nameKey: 'country_pl', cityName: 'Warsaw', iso: 'pl' },
  AT: { coords: [48.2082, 16.3738], nameKey: 'country_at', cityName: 'Vienna', iso: 'at' },
  IT: { coords: [45.4642, 9.1900], nameKey: 'country_it', cityName: 'Milan', iso: 'it' },
  ES: { coords: [40.4168, -3.7038], nameKey: 'country_es', cityName: 'Madrid', iso: 'es' },
  NO: { coords: [59.9139, 10.7522], nameKey: 'country_no', cityName: 'Oslo', iso: 'no' },
  DK: { coords: [55.6761, 12.5683], nameKey: 'country_dk', cityName: 'Copenhagen', iso: 'dk' },
  BE: { coords: [50.8503, 4.3517], nameKey: 'country_be', cityName: 'Brussels', iso: 'be' },
  RO: { coords: [44.4268, 26.1025], nameKey: 'country_ro', cityName: 'Bucharest', iso: 'ro' },
  CZ: { coords: [50.0755, 14.4378], nameKey: 'country_cz', cityName: 'Prague', iso: 'cz' },
  HU: { coords: [47.4979, 19.0402], nameKey: 'country_hu', cityName: 'Budapest', iso: 'hu' },
  GR: { coords: [37.9838, 23.7275], nameKey: 'country_gr', cityName: 'Athens', iso: 'gr' },
  PT: { coords: [38.7223, -9.1393], nameKey: 'country_pt', cityName: 'Lisbon', iso: 'pt' },
  IE: { coords: [53.3498, -6.2603], nameKey: 'country_ie', cityName: 'Dublin', iso: 'ie' },
  UA: { coords: [50.4501, 30.5234], nameKey: 'country_ua', cityName: 'Kyiv', iso: 'ua' },
  AU: { coords: [-33.8688, 151.2093], nameKey: 'country_au', cityName: 'Sydney', iso: 'au' },
  BR: { coords: [-23.5505, -46.6333], nameKey: 'country_br', cityName: 'São Paulo', iso: 'br' },
  IN: { coords: [19.0760, 72.8777], nameKey: 'country_in', cityName: 'Mumbai', iso: 'in' },
  HK: { coords: [22.3193, 114.1694], nameKey: 'country_hk', cityName: 'Hong Kong, China', iso: 'hk' },
  TW: { coords: [25.0330, 121.5654], nameKey: 'country_tw', cityName: 'Taipei, China', iso: 'tw' },
  BG: { coords: [42.6977, 23.3219], nameKey: 'country_bg', cityName: 'Sofia', iso: 'bg' },
  HR: { coords: [45.8150, 15.9819], nameKey: 'country_hr', cityName: 'Zagreb', iso: 'hr' },
  SK: { coords: [48.1486, 17.1077], nameKey: 'country_sk', cityName: 'Bratislava', iso: 'sk' },
  LT: { coords: [54.6872, 25.2797], nameKey: 'country_lt', cityName: 'Vilnius', iso: 'lt' },
  LV: { coords: [56.9496, 24.1052], nameKey: 'country_lv', cityName: 'Riga', iso: 'lv' },
  EE: { coords: [59.4370, 24.7535], nameKey: 'country_ee', cityName: 'Tallinn', iso: 'ee' },
  IS: { coords: [64.1466, -21.9426], nameKey: 'country_is', cityName: 'Reykjavik', iso: 'is' },
  LU: { coords: [49.6116, 6.1319], nameKey: 'country_lu', cityName: 'Luxembourg', iso: 'lu' },
  AL: { coords: [41.3275, 19.8187], nameKey: 'country_al', cityName: 'Tirana', iso: 'al' },
  AZ: { coords: [40.4093, 49.8671], nameKey: 'country_az', cityName: 'Baku', iso: 'az' },
  AM: { coords: [40.1792, 44.4991], nameKey: 'country_am', cityName: 'Yerevan', iso: 'am' },
  GE: { coords: [41.7151, 44.8271], nameKey: 'country_ge', cityName: 'Tbilisi', iso: 'ge' },
  KZ: { coords: [51.1694, 71.4491], nameKey: 'country_kz', cityName: 'Astana', iso: 'kz' },
  UZ: { coords: [41.2995, 69.2401], nameKey: 'country_uz', cityName: 'Tashkent', iso: 'uz' },
  ZA: { coords: [-26.2041, 28.0473], nameKey: 'country_za', cityName: 'Johannesburg', iso: 'za' },
  EG: { coords: [30.0444, 31.2357], nameKey: 'country_eg', cityName: 'Cairo', iso: 'eg' },
  IL: { coords: [32.0853, 34.7818], nameKey: 'country_il', cityName: 'Tel Aviv', iso: 'il' },
  SA: { coords: [24.7136, 46.6753], nameKey: 'country_sa', cityName: 'Riyadh', iso: 'sa' },
  QA: { coords: [25.2854, 51.5310], nameKey: 'country_qa', cityName: 'Doha', iso: 'qa' },
  KW: { coords: [29.3759, 47.9774], nameKey: 'country_kw', cityName: 'Kuwait City', iso: 'kw' },
  IQ: { coords: [33.3152, 44.3661], nameKey: 'country_iq', cityName: 'Baghdad', iso: 'iq' },
  AR: { coords: [-34.6037, -58.3816], nameKey: 'country_ar', cityName: 'Buenos Aires', iso: 'ar' },
  CL: { coords: [-33.4489, -70.6693], nameKey: 'country_cl', cityName: 'Santiago', iso: 'cl' },
  MX: { coords: [19.4326, -99.1332], nameKey: 'country_mx', cityName: 'Mexico City', iso: 'mx' },
  NZ: { coords: [-36.8485, 174.7633], nameKey: 'country_nz', cityName: 'Auckland', iso: 'nz' },
  CN: { coords: [39.9042, 116.4074], nameKey: 'country_cn', cityName: 'Beijing', iso: 'cn' },
}

/** The client node the map draws the route *from* (the app's home region). */
const CLIENT_NODE = CITY_MAP.tehran

// Great-circle Haversine Distance in Kilometers
export function calculateDistanceKm(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const R = 6371
  const dLat = ((lat2 - lat1) * Math.PI) / 180
  const dLon = ((lon2 - lon1) * Math.PI) / 180
  const a =
    Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos((lat1 * Math.PI) / 180) * Math.cos((lat2 * Math.PI) / 180) *
    Math.sin(dLon / 2) * Math.sin(dLon / 2)
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a))
  return Math.round(R * c)
}

/**
 * Resolves a gateway location from the *real* server metadata.
 *
 * Returns `null` when nothing identifies the node, so the caller can render an
 * honest "standby" state instead of inventing a location. (This used to fall
 * back to a hardcoded Frankfurt entry, which meant a disconnected app happily
 * displayed a German gateway.)
 */
export function resolveLocationInfo(
  serverCountry?: string,
  serverName?: string,
  _serverIp?: string
): LocationInfo | null {
  if (!serverCountry && !serverName) return null

  const nameHaystack = `${serverName || ''} ${serverCountry || ''}`.toLowerCase().replace(/[\-_]/g, ' ')

  // 1. Check exact city keywords first for premier accuracy
  for (const [key, loc] of Object.entries(CITY_MAP)) {
    if (nameHaystack.includes(key.replace('_', ' '))) {
      return loc
    }
  }

  // 2. Resolve ISO from flag or country name
  const iso = resolveCountryISO(serverCountry || serverName || '').toUpperCase()
  if (COUNTRY_COORDINATES[iso]) {
    return COUNTRY_COORDINATES[iso]
  }

  return null
}

export default function GlobeWidget({
  connected,
  serverIp,
  serverCountry,
  serverName,
  clientCountry,
  clientIp,
}: GlobeWidgetProps) {
  const globeRef = useRef<GlobeMethods | undefined>(undefined)
  const [globeReady, setGlobeReady] = useState(false)
  const [dimensions, setDimensions] = useState({ width: 380, height: 280 })
  const [viewMode, setViewMode] = useState<'3d' | '2d'>('3d')
  const [autoRotate, setAutoRotate] = useState(true)
  const [isDark, setIsDark] = useState(() => isDarkTheme())
  // WebGL cannot read CSS variables, so the palette is sampled from the live
  // stylesheet and re-sampled whenever the theme changes.
  const [palette, setPalette] = useState(() => readGlobePalette())
  const containerRef = useRef<HTMLDivElement>(null)

  /**
   * The globe's surface material, owned by us rather than left to three-globe.
   *
   * three-globe assigns `globeMaterial.color = null` as soon as the texture
   * loads, which is a signal it expects its own default material to handle.
   * Supplying our own and re-asserting `color` after the load keeps the diffuse
   * term predictable — otherwise the sphere's brightness depends entirely on
   * whatever specular the library happens to leave enabled, which is how the
   * globe ended up invisible on the light theme.
   */
  const globeMaterial = useMemo(
    () =>
      new THREE.MeshPhongMaterial({
        color: 0xffffff,
        shininess: 0,
        specular: new THREE.Color(0x000000),
        bumpScale: 0.3,
      }),
    []
  )

  /**
   * The whole lighting rig, per theme.
   *
   * three.js r155+ dropped legacy light units, so a Phong surface comes out as
   * `albedo × (ambient + NdotL × key / π)` and anything above 1.0 clips to white.
   *
   * Measured on the shipped assets (grid sample of the equirectangular maps,
   * Rec.709 luminance over 0–255):
   *   globe-surface-light  ocean 215 · land 149
   *   globe-surface-dark   ocean  27 · land  67
   *
   * The dark numbers explain why the dark theme was never complained about: a
   * multiplier of ~4 (ambient 1.6 + key 7.5/π) on a near-black map still lands
   * under 255, and the lit continents glow. Run the same rig on the light map and
   * 215 × 1.83 = 393 and 149 × 1.83 = 273 — *both* ocean and land clip to pure
   * white, so the sphere renders as a white disc with no graphics on a white card.
   * The old light values (1.0 / 2.6) clipped the same way; only the emissive floor
   * of 0.42 made it look intentional.
   *
   * So the light rig is chosen from that arithmetic, not by eye:
   * 0.46 + 1.55/π ≈ 0.95, which keeps the brightest texture value (215) below
   * 255 after the tint, and the 0xdde5f4 albedo multiplier does two jobs at once —
   * it pulls the near-white ocean off the 96%-luminance card so the globe has a
   * silhouette, and it deepens ocean-vs-land contrast instead of flattening it.
   *
   * `bumpScale` carries what the flat lighting gives up: relief shading is what
   * makes a low-contrast map look drawn rather than washed. `emissive` exists to
   * keep a *dark* globe's night side readable; on the light theme it is switched
   * off, because it adds a second copy of an already bright texture on top of the
   * first — the exact wash being fixed here.
   */
  const globeLighting = useMemo(
    () =>
      isDark
        ? {
          ambient: 1.6,
          key: 7.5,
          rim: 1.2,
          emissive: 0.34,
          tint: 0xffffff,
          bumpScale: 0.3,
          atmosphereAltitude: 0.16,
          ringOpacity: 0.3,
        }
        : {
          ambient: 0.46,
          key: 1.55,
          rim: 0.6,
          emissive: 0,
          tint: 0xdde5f4,
          bumpScale: 0.62,
          atmosphereAltitude: 0.1,
          ringOpacity: 0.22,
        },
    [isDark]
  )

  // Listen to active theme changes
  useEffect(() => {
    const handleThemeChange = () => {
      setIsDark(isDarkTheme())
      setPalette(readGlobePalette())
    }
    window.addEventListener('nexvpn:theme-changed', handleThemeChange)
    return () => window.removeEventListener('nexvpn:theme-changed', handleThemeChange)
  }, [])

  // Auto-resize handler
  useEffect(() => {
    if (!containerRef.current) return
    const updateSize = () => {
      if (containerRef.current) {
        setDimensions({
          width: containerRef.current.clientWidth || 380,
          height: containerRef.current.clientHeight || 280,
        })
      }
    }
    updateSize()
    window.addEventListener('resize', updateSize)
    return () => window.removeEventListener('resize', updateSize)
  }, [])

  /**
   * Where the route *starts*: the client's real location when the live IP lookup
   * knows it, and the Tehran node only as a last resort.
   *
   * The origin used to be pinned to Tehran unconditionally, which is the second
   * half of the "it always shows Iran" report — the arc, the rings, the node and
   * the label all drew from a hardcoded city rather than from where the machine
   * actually is.
   */
  const originLoc = useMemo(
    () => resolveLocationInfo(clientCountry, undefined, clientIp) ?? CLIENT_NODE,
    [clientCountry, clientIp]
  )

  const origin = originLoc
  const targetLoc = useMemo(
    () => resolveLocationInfo(serverCountry, serverName, serverIp),
    [serverCountry, serverName, serverIp]
  )

  /** The route is only drawn when we know *both* ends of it. */
  const hasRoute = connected && targetLoc !== null

  const distanceKm = useMemo(() => {
    if (!targetLoc) return 0
    return calculateDistanceKm(
      origin.coords[0],
      origin.coords[1],
      targetLoc.coords[0],
      targetLoc.coords[1]
    )
  }, [origin, targetLoc])

  // ── 3D layers ─────────────────────────────────────────────────────────────
  // Restraint is the point: the arc and the two nodes are the only coloured
  // elements, so the accent keeps meaning something.

  const arcsData = useMemo(() => {
    if (!hasRoute || !targetLoc) return []
    return [
      {
        startLat: origin.coords[0],
        startLng: origin.coords[1],
        endLat: targetLoc.coords[0],
        endLng: targetLoc.coords[1],
        color: [palette.primary, palette.success, palette.primary],
        stroke: 1.6,
        dashLength: 0.42,
        dashGap: 0.12,
        dashAnimateTime: 2600,
      },
    ]
  }, [hasRoute, targetLoc, origin, palette])

  const ringsData = useMemo(() => {
    const list: Array<Record<string, unknown>> = [
      {
        lat: origin.coords[0],
        lng: origin.coords[1],
        maxR: 3.4,
        propagationSpeed: 1.6,
        repeatPeriod: 1800,
        color: () => palette.muted,
      },
    ]
    if (hasRoute && targetLoc) {
      list.push({
        lat: targetLoc.coords[0],
        lng: targetLoc.coords[1],
        maxR: 4.2,
        propagationSpeed: 2.0,
        repeatPeriod: 1600,
        color: () => palette.success,
      })
    }
    return list
  }, [hasRoute, targetLoc, origin, palette])

  const pointsData = useMemo(() => {
    const list: Array<Record<string, unknown>> = [
      {
        lat: origin.coords[0],
        lng: origin.coords[1],
        size: 0.5,
        color: palette.muted,
        name: 'Client',
      },
    ]
    if (hasRoute && targetLoc) {
      list.push({
        lat: targetLoc.coords[0],
        lng: targetLoc.coords[1],
        size: 0.62,
        color: palette.success,
        name: targetLoc.cityName,
      })
    }
    return list
  }, [hasRoute, targetLoc, origin, palette])

  const labelsData = useMemo(() => {
    const list: Array<Record<string, unknown>> = []
    if (hasRoute && targetLoc) {
      list.push({
        lat: origin.coords[0],
        lng: origin.coords[1],
        text: origin.cityName.replace(' (Client)', ''),
        color: palette.muted,
        size: 0.62,
      })
      list.push({
        lat: targetLoc.coords[0],
        lng: targetLoc.coords[1],
        text: targetLoc.cityName,
        color: palette.success,
        size: 0.7,
      })
    }
    return list
  }, [hasRoute, targetLoc, origin, palette])

  // Camera Recenter Action
  const recenterCamera = useCallback(() => {
    if (!globeRef.current) return
    if (hasRoute && targetLoc) {
      const midLat = (origin.coords[0] + targetLoc.coords[0]) / 2
      const midLng = (origin.coords[1] + targetLoc.coords[1]) / 2
      globeRef.current.pointOfView({ lat: midLat + 6, lng: midLng, altitude: 2.35 }, 1400)
    } else {
      globeRef.current.pointOfView({ lat: 28, lng: 45, altitude: 2.5 }, 1200)
    }
  }, [hasRoute, targetLoc, origin])

  // ── Three.js scene: one thin orbital ring, no particle swarm ──────────────
  const handleGlobeReady = useCallback(() => setGlobeReady(true), [])

  /**
   * Lights, orbital ring and surface finish.
   *
   * This lives in an effect rather than inside `onGlobeReady` because that
   * callback fires exactly once per mount: changing theme swaps `globeImageUrl`
   * but never re-runs it, so the previous theme's light rig stayed in the scene.
   * A globe that came up dark and was flipped to light was therefore lit with the
   * dark theme's ~4× multiplier on a near-white map — the second half of the
   * "the light globe is a white blob" report. Everything the theme owns is
   * re-applied here.
   */
  useEffect(() => {
    if (!globeReady) return
    const scene = globeRef.current?.scene()
    if (!scene) return

    // Replace any previous decorative group (theme switch, route change).
    const stale: THREE.Object3D[] = []
    scene.traverse((child) => {
      if (child.name === 'nexvpn-orbit') stale.push(child)
    })
    stale.forEach((obj) => scene.remove(obj))

    const hex = (c: string) => parseInt(c.slice(1), 16)

    const orbitGroup = new THREE.Group()
    orbitGroup.name = 'nexvpn-orbit'

    // Key light and a soft brand-tinted rim. Intensities are bounded by the
    // clipping arithmetic in `globeLighting`, not by taste.
    orbitGroup.add(new THREE.AmbientLight(0xffffff, globeLighting.ambient))

    const keyLight = new THREE.DirectionalLight(0xffffff, globeLighting.key)
    keyLight.position.set(120, 90, 130)
    orbitGroup.add(keyLight)

    const rimLight = new THREE.DirectionalLight(hex(palette.primary), globeLighting.rim)
    rimLight.position.set(-130, -60, -110)
    orbitGroup.add(rimLight)

    // A single, thin orbital ring — the "data path" motif, kept quiet.
    const ringGeo = new THREE.RingGeometry(102.4, 103.0, 128)
    const ringMat = new THREE.MeshBasicMaterial({
      color: hex(hasRoute ? palette.success : palette.muted),
      side: THREE.DoubleSide,
      transparent: true,
      opacity: globeLighting.ringOpacity,
    })
    const ring = new THREE.Mesh(ringGeo, ringMat)
    ring.rotation.x = Math.PI / 2.4
    orbitGroup.add(ring)

    scene.add(orbitGroup)

    // Matte finish. A Phong specular at low shininess spreads into one huge
    // white blob across the lit hemisphere and swamps the texture; a globe this
    // graphic should be lit flat, like a printed map.
    scene.traverse((obj) => {
      const mesh = obj as THREE.Mesh
      if (mesh.isMesh && mesh.material && !Array.isArray(mesh.material)) {
        const mat = mesh.material as THREE.MeshPhongMaterial
        if ('shininess' in mat) {
          mat.shininess = 0
          mat.specular = new THREE.Color(0x000000)
          mat.bumpScale = globeLighting.bumpScale
        }
      }
    })

    // Re-assert the surface material: three-globe nulls `color` once the texture
    // is wired up, and without this the diffuse term collapses to whatever the
    // specular term happens to catch.
    globeMaterial.color = new THREE.Color(globeLighting.tint)
    globeMaterial.specular = new THREE.Color(0x000000)
    globeMaterial.shininess = 0
    globeMaterial.bumpScale = globeLighting.bumpScale
    if (globeLighting.emissive > 0 && globeMaterial.map) {
      // A small emissive floor so the night side stays readable instead of
      // falling to pure black, and so the texture can never vanish entirely.
      globeMaterial.emissiveMap = globeMaterial.map
      globeMaterial.emissive = new THREE.Color(0xffffff)
      globeMaterial.emissiveIntensity = globeLighting.emissive
    } else {
      globeMaterial.emissiveMap = null
      globeMaterial.emissive = new THREE.Color(0x000000)
      globeMaterial.emissiveIntensity = 0
    }
    globeMaterial.needsUpdate = true

    let raf = 0
    const animate = () => {
      ring.rotation.z += 0.0009
      raf = requestAnimationFrame(animate)
    }
    animate()

    return () => cancelAnimationFrame(raf)
  }, [globeReady, globeLighting, hasRoute, palette, globeMaterial])

  // Camera glide when the route changes
  useEffect(() => {
    if (!globeRef.current || viewMode !== '3d') return
    const controls = globeRef.current.controls()
    if (controls) {
      controls.autoRotate = autoRotate
      controls.autoRotateSpeed = hasRoute ? 0.28 : 0.5
      controls.enableZoom = true
      controls.enablePan = false
      controls.minDistance = 160
      controls.maxDistance = 520
    }
    recenterCamera()
  }, [hasRoute, targetLoc, globeReady, viewMode, autoRotate, recenterCamera])

  const texture = isDark ? '/globe/globe-surface-dark.png' : '/globe/globe-surface-light.png'

  const pill = 'flex items-center gap-2 rounded-lg border border-border bg-background/80 px-2.5 py-1.5 backdrop-blur-md shadow-xs'
  const iconBtn = (active: boolean) =>
    cn(
      'grid h-7 w-7 place-items-center rounded-md border border-border bg-background/80 backdrop-blur-md shadow-xs transition-colors',
      active ? 'text-primary' : 'text-muted-foreground hover:text-foreground'
    )

  return (
    <div
      ref={containerRef}
      className="relative flex h-[280px] w-full select-none items-center justify-center overflow-hidden rounded-xl border border-border bg-background transition-colors duration-300"
    >
      {/* Soft vignette behind the sphere, tinted with the brand token. */}
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_50%_45%,hsl(var(--primary)/0.07),transparent_68%)]" />

      {/* Top overlay: telemetry (start) + view controls (end) */}
      <div className="pointer-events-none absolute inset-x-3 top-3 z-10 flex items-start justify-between gap-2">
        <div className={cn(pill, 'pointer-events-auto min-w-0')}>
          <CountryFlag
            codeOrName={
              hasRoute && targetLoc ? targetLoc.iso : connected ? serverCountry || 'IR' : origin.iso
            }
            size="sm"
            circular
          />
          <div className="flex min-w-0 flex-col">
            <span className="truncate text-[11px] font-semibold leading-tight text-foreground">
              {hasRoute && targetLoc
                ? targetLoc.cityName
                : connected
                  ? t('globe_ready_routing')
                  : origin.cityName.replace(' (Client)', '')}
            </span>
            <span className="truncate font-mono text-[9px] text-muted-foreground">
              {hasRoute && targetLoc
                ? `${distanceKm.toLocaleString()} km${serverIp ? ` · ${serverIp}` : ''}`
                : connected
                  ? t('globe_no_tunnel')
                  : clientIp || t('globe_no_tunnel')}
            </span>
          </div>
        </div>

        <div className="pointer-events-auto flex shrink-0 items-center gap-1.5">
          {/* 3D / 2D switcher */}
          <div className="flex items-center gap-0.5 rounded-lg border border-border bg-background/80 p-0.5 backdrop-blur-md shadow-xs">
            {(['3d', '2d'] as const).map(mode => (
              <button
                key={mode}
                type="button"
                onClick={() => setViewMode(mode)}
                aria-pressed={viewMode === mode}
                className={cn(
                  'rounded-md px-2 py-1 font-mono text-[10px] font-medium transition-colors',
                  viewMode === mode
                    ? 'bg-primary/10 text-primary'
                    : 'text-muted-foreground hover:text-foreground'
                )}
                title={mode === '3d' ? '3D globe' : '2D world map'}
              >
                {mode.toUpperCase()}
              </button>
            ))}
          </div>

          {viewMode === '3d' && (
            <>
              <button type="button" onClick={recenterCamera} className={iconBtn(false)} title={t('globe_recenter')}>
                <Crosshair size={13} />
              </button>
              <button
                type="button"
                onClick={() => setAutoRotate(!autoRotate)}
                aria-pressed={autoRotate}
                className={iconBtn(autoRotate)}
                title={autoRotate ? t('globe_pause_rotation') : t('globe_resume_rotation')}
              >
                <RotateCw size={13} />
              </button>
            </>
          )}
        </div>
      </div>

      {/* Main visualization */}
      {viewMode === '3d' ? (
        <Suspense
          fallback={
            <div role="status" aria-busy="true" className="flex flex-col items-center justify-center gap-2 text-xs text-muted-foreground">
              <span className="h-5 w-5 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-primary" />
              <span>{t('globe_loading')}</span>
            </div>
          }
        >
          <Globe
            ref={globeRef}
            onGlobeReady={handleGlobeReady}
            width={dimensions.width}
            height={dimensions.height}
            backgroundColor="rgba(0,0,0,0)"
            globeImageUrl={texture}
            globeMaterial={globeMaterial}
            bumpImageUrl="/globe/earth-topology.png"
            showAtmosphere={true}
            atmosphereAltitude={globeLighting.atmosphereAltitude}
            atmosphereColor={hasRoute ? palette.success : palette.primary}
            arcsData={arcsData}
            arcColor="color"
            arcStroke="stroke"
            arcAltitude={0.22}
            arcDashLength={0.42}
            arcDashGap={0.12}
            arcDashAnimateTime={2600}
            ringsData={ringsData}
            ringColor="color"
            ringMaxRadius="maxR"
            ringPropagationSpeed="propagationSpeed"
            ringRepeatPeriod="repeatPeriod"
            ringAltitude={0.008}
            pointsData={pointsData}
            pointColor="color"
            pointAltitude={0.02}
            pointRadius="size"
            pointsMerge={false}
            labelsData={labelsData}
            labelText="text"
            labelColor="color"
            labelSize="size"
            labelAltitude={0.014}
            labelDotRadius={0.22}
            labelResolution={2}
          />
        </Suspense>
      ) : (
        <CyberRadar2D
          width={dimensions.width}
          height={dimensions.height}
          hasRoute={hasRoute}
          origin={origin}
          target={targetLoc}
          texture={texture}
          palette={palette}
        />
      )}

      {/* Bottom overlay: what you are looking at */}
      <div className="pointer-events-none absolute inset-x-3 bottom-3 z-10 flex items-center justify-between gap-3">
        <span className={cn(pill, 'font-mono text-[10px] text-muted-foreground')}>
          {viewMode === '3d' ? '3D' : '2D'}
        </span>
        <span className={cn(pill, 'min-w-0 font-mono text-[10px]')}>
          {hasRoute && targetLoc ? (
            <>
              <span className="text-muted-foreground">{origin.cityName.replace(' (Client)', '')}</span>
              <span className="text-muted-foreground/60">→</span>
              <span className="truncate font-semibold text-foreground">{targetLoc.cityName}</span>
            </>
          ) : (
            <span className="text-muted-foreground">{t('globe_no_active_route')}</span>
          )}
        </span>
      </div>
    </div>
  )
}

// ─── 2D world map subcomponent ───────────────────────────────────────────────
function CyberRadar2D({
  width,
  height,
  hasRoute,
  origin,
  target,
  texture,
  palette,
}: {
  width: number
  height: number
  hasRoute: boolean
  origin: LocationInfo
  target: LocationInfo | null
  texture: string
  /** Theme palette; SVG cannot resolve the CSS tokens itself. */
  palette: ReturnType<typeof readGlobePalette>
}) {
  // Letterbox the 2:1 map inside whatever box we were given.
  const mapW = width
  const mapH = Math.min(width / 2, height)
  const mapX = 0
  const mapY = (height - mapH) / 2

  // Equirectangular projection into the map box.
  const project = (lat: number, lng: number): [number, number] => [
    mapX + ((lng + 180) / 360) * mapW,
    mapY + ((90 - lat) / 180) * mapH,
  ]

  const [originX, originY] = project(origin.coords[0], origin.coords[1])
  const [targetX, targetY] = target ? project(target.coords[0], target.coords[1]) : [0, 0]

  // Quadratic Bezier control point, lifted above the two endpoints.
  const midX = (originX + targetX) / 2
  const midY = Math.min(originY, targetY) - mapH * 0.22
  const arc = `M ${originX} ${originY} Q ${midX} ${midY} ${targetX} ${targetY}`

  return (
    <div className="relative flex h-full w-full items-center justify-center overflow-hidden">
      <svg width={width} height={height} className="h-full w-full" xmlns="http://www.w3.org/2000/svg">
        {/* The real equirectangular texture, rather than hand-drawn blobs. */}
        <image
          href={texture}
          x={mapX}
          y={mapY}
          width={mapW}
          height={mapH}
          preserveAspectRatio="none"
          opacity={0.9}
        />

        {hasRoute && target && (
          <>
            <path d={arc} fill="none" stroke={palette.primary} strokeWidth={3} strokeOpacity={0.18} strokeLinecap="round" />
            <path
              d={arc}
              fill="none"
              stroke={palette.primary}
              strokeWidth={1.5}
              strokeLinecap="round"
              strokeDasharray="5 6"
            >
              <animate attributeName="stroke-dashoffset" from="0" to="-22" dur="1.1s" repeatCount="indefinite" />
            </path>
          </>
        )}

        {/* Client node */}
        <g transform={`translate(${originX}, ${originY})`}>
          <circle r="2.5" fill={palette.muted} />
          <circle r="2.5" fill="none" stroke={palette.muted} strokeWidth="1">
            <animate attributeName="r" from="2.5" to="9" dur="2.4s" repeatCount="indefinite" />
            <animate attributeName="opacity" from="0.5" to="0" dur="2.4s" repeatCount="indefinite" />
          </circle>
        </g>

        {/* Gateway node */}
        {hasRoute && target && (
          <g transform={`translate(${targetX}, ${targetY})`}>
            <circle r="3.5" fill={palette.success} />
            <circle r="3.5" fill="none" stroke={palette.success} strokeWidth="1">
              <animate attributeName="r" from="3.5" to="12" dur="2s" repeatCount="indefinite" />
              <animate attributeName="opacity" from="0.6" to="0" dur="2s" repeatCount="indefinite" />
            </circle>
          </g>
        )}
      </svg>
    </div>
  )
}
