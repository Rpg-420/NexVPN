export type Theme =
  | 'cyber-dark'
  | 'clean-light'
  | 'neon-cyan'
  | 'emerald-matrix'
  | 'purple-nebula'
  | 'sunset-ember'
  | 'auto'

export interface ThemeOption {
  id: Theme
  nameEn: string
  nameFa: string
  accentColor: string
  bgPreview: string
  borderPreview: string
  isDark: boolean
}

export const THEMES: ThemeOption[] = [
  {
    id: 'cyber-dark',
    nameEn: 'Midnight Obsidian (Dark)',
    nameFa: 'دارک نئونی (Midnight Obsidian)',
    accentColor: '#6366f1',
    bgPreview: '#080a0f',
    borderPreview: '#6366f1',
    isDark: true,
  },
  {
    id: 'clean-light',
    nameEn: 'Clean Ceramic (Light)',
    nameFa: 'لایت مینیمال (Clean Ceramic)',
    accentColor: '#4f46e5',
    bgPreview: '#f8fafc',
    borderPreview: '#cbd5e1',
    isDark: false,
  },
  {
    id: 'neon-cyan',
    nameEn: 'Neon Cyan & Ocean (2026)',
    nameFa: 'نئون فیروزه‌ای و اقیانوسی',
    accentColor: '#06b6d4',
    bgPreview: '#03131e',
    borderPreview: '#06b6d4',
    isDark: true,
  },
  {
    id: 'emerald-matrix',
    nameEn: 'Emerald Matrix & Forest',
    nameFa: 'سبز ماتریکس و زمردی',
    accentColor: '#10b981',
    bgPreview: '#04140e',
    borderPreview: '#10b981',
    isDark: true,
  },
  {
    id: 'purple-nebula',
    nameEn: 'Purple Nebula & Amethyst',
    nameFa: 'بنفش کهکشانی و آمتیست',
    accentColor: '#a855f7',
    bgPreview: '#0f071e',
    borderPreview: '#a855f7',
    isDark: true,
  },
  {
    id: 'sunset-ember',
    nameEn: 'Sunset Ember & Gold',
    nameFa: 'غروب آفتاب و طلایی گرم',
    accentColor: '#f97316',
    bgPreview: '#150a04',
    borderPreview: '#f97316',
    isDark: true,
  },
  {
    id: 'auto',
    nameEn: 'Auto (Follow Windows OS)',
    nameFa: 'خودکار (هماهنگ با ویندوز)',
    accentColor: '#3b82f6',
    bgPreview: 'linear-gradient(135deg, #080a0f 50%, #f8fafc 50%)',
    borderPreview: '#3b82f6',
    isDark: true,
  },
]

let currentTheme: Theme = 'cyber-dark'
const listeners: Array<(theme: Theme) => void> = []

export function getTheme(): Theme {
  const saved = localStorage.getItem('nexvpn_theme') as Theme
  if (saved && THEMES.some(t => t.id === saved)) {
    currentTheme = saved
  }
  return currentTheme
}

export function applyTheme(theme: Theme) {
  currentTheme = theme
  localStorage.setItem('nexvpn_theme', theme)

  let effectiveTheme = theme
  if (theme === 'auto') {
    const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
    effectiveTheme = prefersDark ? 'cyber-dark' : 'clean-light'
  }

  document.documentElement.setAttribute('data-theme', effectiveTheme)
  document.documentElement.classList.toggle('dark', effectiveTheme !== 'clean-light')

  // Dispatch custom window event for WebGL/Canvas re-renders
  window.dispatchEvent(new CustomEvent('nexvpn:theme-changed', { detail: { theme, effectiveTheme, isDark: effectiveTheme !== 'clean-light' } }))

  // Notify listeners
  listeners.forEach(fn => fn(theme))
}

export function isDarkTheme(): boolean {
  const theme = getTheme()
  if (theme === 'clean-light') return false
  if (theme === 'auto') {
    return !(window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches)
  }
  return true
}

let systemThemeListenerAttached = false

export function initTheme() {
  const saved = getTheme()
  applyTheme(saved)

  // Watch for system color scheme changes if in auto mode. Guarded so remounting the
  // component that calls initTheme() (e.g. React StrictMode's double-invoke in dev)
  // doesn't stack up duplicate listeners forever.
  if (window.matchMedia && !systemThemeListenerAttached) {
    systemThemeListenerAttached = true
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (getTheme() === 'auto') {
        applyTheme('auto')
      }
    })
  }
}

export function onThemeChange(cb: (theme: Theme) => void) {
  listeners.push(cb)
  return () => {
    const idx = listeners.indexOf(cb)
    if (idx !== -1) listeners.splice(idx, 1)
  }
}

/**
 * Resolves a design token to a concrete CSS colour string.
 *
 * CSS variables cannot cross into WebGL/three.js, which need a hex literal for
 * materials, lights and point colours, so canvas-based visuals have to read the
 * computed value at runtime. Reading it from the live stylesheet (rather than
 * duplicating the palette in JS) is what keeps the 3D globe in step with all
 * seven themes instead of being pinned to a hardcoded neon cyan.
 *
 * @param varName token name, e.g. `--primary` (declared as `H S% L%`)
 * @param fallback used when the token is missing or unparsable
 */
export function resolveToken(varName: string, fallback: string): string {
  if (typeof document === 'undefined') return fallback
  const raw = getComputedStyle(document.documentElement).getPropertyValue(varName).trim()
  const m = raw.match(/^([\d.]+)\s+([\d.]+)%\s+([\d.]+)%$/)
  if (!m) return fallback

  const h = parseFloat(m[1])
  const s = parseFloat(m[2]) / 100
  const l = parseFloat(m[3]) / 100

  // HSL -> RGB
  const a = s * Math.min(l, 1 - l)
  const channel = (n: number) => {
    const k = (n + h / 30) % 12
    return Math.round(255 * (l - a * Math.max(-1, Math.min(k - 3, 9 - k, 1))))
  }
  return (
    '#' +
    [channel(0), channel(8), channel(4)]
      .map(v => Math.max(0, Math.min(255, v)).toString(16).padStart(2, '0'))
      .join('')
  )
}

/** Same as {@link resolveToken}, but as a numeric literal for three.js. */
export function resolveTokenHex(varName: string, fallback: string): number {
  return parseInt(resolveToken(varName, fallback).slice(1), 16)
}

/** Same as {@link resolveToken}, but as `rgba(r, g, b, a)` for canvas fills. */
export function resolveTokenRgba(varName: string, alpha: number, fallback: string): string {
  const hex = resolveToken(varName, fallback)
  const n = parseInt(hex.slice(1), 16)
  return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`
}

/**
 * The palette the 3D globe renders with, read from the active theme.
 * Call again after a theme change.
 */
export function readGlobePalette() {
  return {
    primary: resolveToken('--primary', '#6366f1'),
    success: resolveToken('--success', '#10b981'),
    /** Neutral surface, used for the globe's sphere material. */
    surface: resolveToken('--card', '#0f0f11'),
    background: resolveToken('--background', '#09090b'),
    muted: resolveToken('--muted-foreground', '#a1a1aa'),
  }
}
