import React, { useState, useEffect } from 'react'
import {
  Layers,
  Search,
  CheckSquare,
  Square,
  Globe,
  Send,
  Gamepad2,
  Music,
  Download,
  Shield,
  Save,
  X,
  Sparkles,
  Check,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { InstalledApp, Settings } from '../types'

interface VisualAppSelectorModalProps {
  isOpen: boolean
  onClose: () => void
  settings: Settings
  onSettingsUpdated: (settings: Settings) => void
}

export const VisualAppSelectorModal: React.FC<VisualAppSelectorModalProps> = ({
  isOpen,
  onClose,
  settings,
  onSettingsUpdated,
}) => {
  const [apps, setApps] = useState<InstalledApp[]>([])
  const [selectedExes, setSelectedExes] = useState<string[]>([])
  const [routingMode, setRoutingMode] = useState<string>('bypass_selected')
  const [searchQuery, setSearchQuery] = useState('')
  const [activeCategory, setActiveCategory] = useState<string>('all')
  const [saving, setSaving] = useState(false)
  const [saveSuccess, setSaveSuccess] = useState(false)

  useEffect(() => {
    if (isOpen) {
      loadApps()
      setSelectedExes(settings.selected_apps || [])
      setRoutingMode(settings.app_routing_mode || 'bypass_selected')
      setSaveSuccess(false)
    }
  }, [isOpen, settings])

  const loadApps = async () => {
    try {
      const list = await api.getInstalledApplications()
      if (list && list.length > 0) {
        setApps(list)
      }
    } catch (err) {
      console.error('Failed to load apps:', err)
    }
  }

  if (!isOpen) return null

  const toggleApp = (exe: string) => {
    setSelectedExes(prev =>
      prev.includes(exe) ? prev.filter(e => e !== exe) : [...prev, exe]
    )
  }

  const selectAllIranian = () => {
    const iranianExes = apps.filter(a => a.category === 'iranian').map(a => a.exe_name)
    setSelectedExes(prev => Array.from(new Set([...prev, ...iranianExes])))
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      await api.saveAppRouting(routingMode, selectedExes)
      const updated = { ...settings, app_routing_mode: routingMode, selected_apps: selectedExes }
      onSettingsUpdated(updated)
      setSaveSuccess(true)
      setTimeout(() => {
        setSaveSuccess(false)
        onClose()
      }, 800)
    } catch (err) {
      console.error('Save routing error:', err)
    } finally {
      setSaving(false)
    }
  }

  const getCategoryIcon = (cat: string) => {
    switch (cat) {
      case 'browser':
        return <Globe className="w-4 h-4 text-blue-400" />
      case 'social':
        return <Send className="w-4 h-4 text-sky-400" />
      case 'game':
        return <Gamepad2 className="w-4 h-4 text-purple-400" />
      case 'media':
        return <Music className="w-4 h-4 text-pink-400" />
      case 'iranian':
        return <Shield className="w-4 h-4 text-amber-400" />
      default:
        return <Download className="w-4 h-4 text-slate-400" />
    }
  }

  const filteredApps = apps.filter(app => {
    const matchSearch =
      app.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      app.exe_name.toLowerCase().includes(searchQuery.toLowerCase())
    const matchCat = activeCategory === 'all' || app.category === activeCategory
    return matchSearch && matchCat
  })

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-md animate-in fade-in duration-200">
      <div
        className="relative w-full max-w-3xl overflow-hidden rounded-2xl border border-white/10 shadow-2xl transition-all flex flex-col max-h-[85vh]"
        style={{
          background: 'linear-gradient(145deg, rgba(20, 24, 39, 0.96), rgba(13, 16, 28, 0.98))',
        }}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-5 border-b border-white/10 shrink-0">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center w-10 h-10 rounded-xl bg-gradient-to-tr from-purple-500/20 to-indigo-500/20 border border-purple-500/30 text-purple-400">
              <Layers className="w-6 h-6" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-white flex items-center gap-2">
                {t('split_modal_header')}
                <span className="text-xs px-2 py-0.5 rounded-full bg-purple-500/20 text-purple-300 border border-purple-500/30 font-medium">
                  App Rules
                </span>
              </h3>
              <p className="text-xs text-slate-400 mt-0.5">
                {t('split_modal_sub')}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-xl text-slate-400 hover:text-white hover:bg-white/5 transition-all"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Mode Selector Tabs */}
        <div className="px-6 pt-4 pb-2 border-b border-white/5 bg-slate-900/30 shrink-0 space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <button
              onClick={() => setRoutingMode('bypass_selected')}
              className={`p-3 rounded-xl border text-right transition-all flex items-center justify-between ${
                routingMode === 'bypass_selected'
                  ? 'bg-purple-600/20 border-purple-500/50 text-white shadow-md shadow-purple-500/10'
                  : 'bg-white/5 border-white/5 text-slate-400 hover:bg-white/10 hover:text-slate-200'
              }`}
            >
              <div>
                <div className="text-xs font-bold">{t('split_tab_bypass')}</div>
                <div className="text-[11px] text-slate-400 mt-0.5">
                  {t('split_tab_bypass_desc')}
                </div>
              </div>
              {routingMode === 'bypass_selected' && (
                <div className="w-2.5 h-2.5 rounded-full bg-purple-400 shadow-sm shadow-purple-400" />
              )}
            </button>

            <button
              onClick={() => setRoutingMode('proxy_selected')}
              className={`p-3 rounded-xl border text-right transition-all flex items-center justify-between ${
                routingMode === 'proxy_selected'
                  ? 'bg-purple-600/20 border-purple-500/50 text-white shadow-md shadow-purple-500/10'
                  : 'bg-white/5 border-white/5 text-slate-400 hover:bg-white/10 hover:text-slate-200'
              }`}
            >
              <div>
                <div className="text-xs font-bold">{t('split_tab_proxy')}</div>
                <div className="text-[11px] text-slate-400 mt-0.5">
                  {t('split_tab_proxy_desc')}
                </div>
              </div>
              {routingMode === 'proxy_selected' && (
                <div className="w-2.5 h-2.5 rounded-full bg-purple-400 shadow-sm shadow-purple-400" />
              )}
            </button>
          </div>

          {/* Search & Category Bar */}
          <div className="flex items-center gap-3">
            <div className="relative flex-1">
              <Search className="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
              <input
                type="text"
                value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
                placeholder={t('split_search_placeholder')}
                className="w-full pr-9 pl-4 py-2 rounded-xl bg-slate-900/80 border border-white/10 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-purple-500 transition-all"
              />
            </div>

            <button
              onClick={selectAllIranian}
              className="flex items-center gap-1.5 px-3 py-2 rounded-xl bg-amber-500/15 hover:bg-amber-500/25 border border-amber-500/30 text-amber-300 text-xs font-bold transition-all shrink-0"
            >
              <Sparkles className="w-3.5 h-3.5" />
              {t('split_auto_iranian_btn')}
            </button>
          </div>

          {/* Categories Pills */}
          <div className="flex items-center gap-2 overflow-x-auto pb-1 text-xs">
            {[
              { id: 'all', label: t('split_cat_all') },
              { id: 'browser', label: t('split_cat_browser') },
              { id: 'social', label: t('split_cat_social') },
              { id: 'game', label: t('split_cat_game') },
              { id: 'media', label: t('split_cat_media') },
              { id: 'iranian', label: t('split_cat_iranian') },
            ].map(cat => (
              <button
                key={cat.id}
                onClick={() => setActiveCategory(cat.id)}
                className={`px-3 py-1 rounded-lg font-medium whitespace-nowrap transition-all ${
                  activeCategory === cat.id
                    ? 'bg-purple-500 text-white font-bold'
                    : 'bg-white/5 text-slate-400 hover:bg-white/10 hover:text-slate-200'
                }`}
              >
                {cat.label}
              </button>
            ))}
          </div>
        </div>

        {/* Apps Grid */}
        <div className="p-6 overflow-y-auto flex-1 grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
          {filteredApps.map(app => {
            const isSelected = selectedExes.includes(app.exe_name)
            return (
              <div
                key={app.id}
                onClick={() => toggleApp(app.exe_name)}
                className={`p-3.5 rounded-xl border cursor-pointer select-none transition-all flex items-center justify-between gap-3 ${
                  isSelected
                    ? 'bg-purple-500/15 border-purple-500/50 shadow-md shadow-purple-500/5'
                    : 'bg-slate-900/40 border-white/5 hover:border-white/15 hover:bg-slate-900/70'
                }`}
              >
                <div className="flex items-center gap-3 overflow-hidden">
                  <div className="p-2 rounded-lg bg-black/40 border border-white/5 shrink-0">
                    {getCategoryIcon(app.category)}
                  </div>
                  <div className="truncate">
                    <div className="text-xs font-bold text-slate-200 truncate flex items-center gap-1.5">
                      {app.name}
                    </div>
                    <div className="text-[11px] font-mono text-slate-400 truncate mt-0.5">
                      {app.exe_name}
                    </div>
                    {app.is_running && (
                      <span className="inline-block mt-1 text-[10px] px-1.5 py-0.2 rounded bg-emerald-500/20 text-emerald-300 font-medium">
                        {t('split_running_badge')}
                      </span>
                    )}
                  </div>
                </div>

                <div className="shrink-0">
                  {isSelected ? (
                    <CheckSquare className="w-5 h-5 text-purple-400" />
                  ) : (
                    <Square className="w-5 h-5 text-slate-600" />
                  )}
                </div>
              </div>
            )
          })}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between px-6 py-4 bg-slate-950/40 border-t border-white/10 shrink-0">
          <div className="text-xs text-slate-400">
            {t('split_selected_count', { count: selectedExes.length })}
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={onClose}
              className="px-4 py-2 rounded-xl text-xs font-semibold text-slate-400 hover:text-white transition-all"
            >
              {t('cancel')}
            </button>
            <button
              onClick={handleSave}
              disabled={saving}
              className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-gradient-to-r from-purple-500 to-indigo-600 hover:from-purple-400 hover:to-indigo-500 text-white text-xs font-bold shadow-lg shadow-purple-500/25 transition-all active:scale-95 disabled:opacity-50"
            >
              {saveSuccess ? (
                <>
                  <Check className="w-4 h-4 text-emerald-300" />
                  {t('saved')}
                </>
              ) : (
                <>
                  <Save className="w-4 h-4" />
                  {saving ? t('saving') : t('split_save_rules_btn')}
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
