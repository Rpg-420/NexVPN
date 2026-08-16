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

  // Notify listeners
  listeners.forEach(fn => fn(theme))
}

export function initTheme() {
  const saved = getTheme()
  applyTheme(saved)

  // Watch for system color scheme changes if in auto mode
  if (window.matchMedia) {
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
