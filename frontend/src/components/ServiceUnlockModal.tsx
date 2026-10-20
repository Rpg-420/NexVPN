import { useState, useEffect } from 'react'
import {
  X,
  RefreshCw,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  Bot,
  Sparkles,
  PlaySquare,
  Music,
  Send,
  MessageSquare,
  Camera,
  Twitter,
  Gamepad2,
  Github,
  Globe,
  Activity,
  Layers,
  Zap,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import { ModalShell } from '../components/ui/modal-shell'
import type { ServiceUnblockResult } from '../types'

interface ServiceUnlockModalProps {
  isOpen: boolean
  onClose: () => void
}

const getCategoryBadge = (category: string) => {
  switch (category) {
    case 'ai':
      return { label: 'AI & LLM', color: 'bg-success/20 text-success border-success/30' }
    case 'streaming':
      return { label: 'Streaming', color: 'bg-destructive/20 text-destructive border-destructive/30' }
    case 'social':
      return { label: 'Social & Chat', color: 'bg-info/20 text-info border-info/30' }
    case 'gaming':
      return { label: 'Gaming', color: 'bg-warning/20 text-warning border-warning/30' }
    case 'dev':
      return { label: 'Developer', color: 'bg-primary/20 text-primary border-primary/30' }
    default:
      return { label: category, color: 'bg-info/20 text-info border-info/30' }
  }
}

const getServiceIcon = (id: string) => {
  switch (id) {
    case 'chatgpt':
      return <Bot className="w-5 h-5 text-success" />
    case 'claude':
      return <Sparkles className="w-5 h-5 text-primary" />
    case 'youtube':
      return <PlaySquare className="w-5 h-5 text-destructive" />
    case 'spotify':
      return <Music className="w-5 h-5 text-success" />
    case 'telegram':
      return <Send className="w-5 h-5 text-primary" />
    case 'discord':
      return <MessageSquare className="w-5 h-5 text-primary" />
    case 'instagram':
      return <Camera className="w-5 h-5 text-pink-500" />
    case 'twitter':
      return <Twitter className="w-5 h-5 text-primary" />
    case 'steam':
      return <Gamepad2 className="w-5 h-5 text-warning" />
    case 'github':
      return <Github className="w-5 h-5 text-foreground" />
    default:
      return <Globe className="w-5 h-5 text-primary" />
  }
}

export default function ServiceUnlockModal({ isOpen, onClose }: ServiceUnlockModalProps) {
  const [results, setResults] = useState<ServiceUnblockResult[]>([])
  const [loading, setLoading] = useState(false)
  const [activeFilter, setActiveFilter] = useState<'all' | 'ai' | 'streaming' | 'social' | 'gaming'>('all')

  const runTest = async () => {
    setLoading(true)
    try {
      const data = await api.checkServiceUnblock()
      if (Array.isArray(data)) {
        setResults(data)
      }
    } catch (err) {
      console.error('Failed to test service unlock matrix:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (isOpen && results.length === 0) {
      runTest()
    }
  }, [isOpen])


  if (!isOpen) return null

  const filteredResults = activeFilter === 'all'
    ? results
    : results.filter(r => r.category === activeFilter)

  const unlockedCount = results.filter(r => r.status === 'unlocked').length
  const totalCount = results.length

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-2xl"
      bodyClassName="p-6"
    >
        {/* Header */}
        <div className="flex items-center justify-between pb-4 border-b border-border">
          <div className="flex items-center gap-3">
            <div className="bg-info/20 p-2.5 rounded-xl border border-border text-primary shadow-inner">
              <Activity className="w-6 h-6" />
            </div>
            <div>
              <h2 className="text-lg font-bold text-foreground flex items-center gap-2">
                {t('service_unblock_title')}
                <span className="text-xs px-2 py-0.5 rounded-full bg-info/20 border border-border text-primary font-mono">
                  Live Probe
                </span>
              </h2>
              <p className="text-xs text-muted-foreground">
                {t('service_unblock_subtitle')}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-lg hover:bg-muted text-muted-foreground hover:text-foreground transition-colors"
          >
            <X size={18} />
          </button>
        </div>

        {/* Stats summary bar */}
        <div className="grid grid-cols-3 gap-3 my-4">
          <div className="p-3 rounded-xl bg-muted border border-border flex items-center justify-between">
            <div>
              <div className="text-[11px] text-muted-foreground">{t('svc_stat_available')}</div>
              <div className="text-lg font-bold text-success flex items-center gap-1.5 mt-0.5">
                <CheckCircle2 size={16} />
                <span>{unlockedCount} / {totalCount || 10}</span>
              </div>
            </div>
            <div className="text-2xl font-black text-success/30 font-mono">
              {totalCount > 0 ? Math.round((unlockedCount / totalCount) * 100) : 0}%
            </div>
          </div>

          <div className="p-3 rounded-xl bg-muted border border-border flex items-center justify-between">
            <div>
              <div className="text-[11px] text-muted-foreground">{t('svc_stat_blocked')}</div>
              <div className="text-lg font-bold text-destructive flex items-center gap-1.5 mt-0.5">
                <XCircle size={16} />
                <span>{totalCount - unlockedCount}</span>
              </div>
            </div>
            <div className="text-2xl font-black text-destructive/30 font-mono">
              {totalCount - unlockedCount}
            </div>
          </div>

          <div className="p-3 rounded-xl bg-muted border border-border flex items-center justify-between">
            <div>
              <div className="text-[11px] text-muted-foreground">{t('svc_stat_latency')}</div>
              <div className="text-lg font-bold text-primary flex items-center gap-1.5 mt-0.5">
                <Zap size={16} />
                <span>
                  {results.filter(r => r.latency_ms > 0).length > 0
                    ? Math.round(
                        results.filter(r => r.latency_ms > 0).reduce((acc, curr) => acc + curr.latency_ms, 0) /
                          results.filter(r => r.latency_ms > 0).length
                      ) + ' ms'
                    : '--'}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Category Tabs & Retest Button */}
        <div className="flex items-center justify-between gap-2 mb-3">
          <div className="flex items-center gap-1 bg-black/40 p-1 rounded-xl border border-border overflow-x-auto text-xs">
            {(['all', 'ai', 'streaming', 'social', 'gaming'] as const).map(cat => (
              <button
                key={cat}
                onClick={() => setActiveFilter(cat)}
                className={`px-3 py-1.5 rounded-lg font-medium transition-all ${
                  activeFilter === cat
                    ? 'bg-primary/20 text-primary border border-primary/40 shadow-sm'
                    : 'text-muted-foreground hover:text-foreground hover:bg-muted'
                }`}
              >
                {cat === 'all' && t('svc_cat_all')}
                {cat === 'ai' && t('svc_cat_ai')}
                {cat === 'streaming' && t('svc_cat_streaming')}
                {cat === 'social' && t('svc_cat_social')}
                {cat === 'gaming' && t('svc_cat_gaming')}
              </button>
            ))}
          </div>

          <button
            onClick={runTest}
            disabled={loading}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-info/20 hover:bg-primary/30 border border-primary/40 text-primary text-xs font-semibold transition-all disabled:opacity-50 shrink-0"
          >
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
            <span>{loading ? t('svc_testing') : t('svc_retest')}</span>
          </button>
        </div>

        {/* Services List Matrix */}
        <div className="space-y-2 max-h-[340px] overflow-y-auto pe-1 custom-scrollbar">
          {loading && results.length === 0 ? (
            <div className="p-12 text-center text-muted-foreground space-y-3">
              <RefreshCw className="w-8 h-8 mx-auto animate-spin text-primary" />
              <div className="text-sm font-medium">{t('svc_testing_all')}</div>
            </div>
          ) : filteredResults.length === 0 ? (
            <div className="p-8 text-center text-muted-foreground text-xs">
              {t('svc_empty_category')}
            </div>
          ) : (
            filteredResults.map(item => {
              const badge = getCategoryBadge(item.category)
              const isUnlocked = item.status === 'unlocked'
              const isRestricted = item.status === 'restricted'

              return (
                <div
                  key={item.id}
                  className="flex items-center justify-between p-3 rounded-xl bg-card/[0.03] hover:bg-card/[0.06] border border-border transition-all group"
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <div className="p-2 rounded-xl bg-black/40 border border-border shrink-0">
                      {getServiceIcon(item.id)}
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold text-sm text-foreground group-hover:text-primary transition-colors truncate">
                          {item.name}
                        </span>
                        <span className={`text-[10px] px-2 py-0.5 rounded-full border ${badge.color} font-medium`}>
                          {badge.label}
                        </span>
                      </div>
                      <div className="text-xs text-muted-foreground mt-0.5 truncate">
                        {item.message}
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-3 shrink-0">
                    {item.latency_ms > 0 && (
                      <span className="text-xs font-mono text-muted-foreground bg-black/30 px-2 py-0.5 rounded-lg border border-border">
                        {item.latency_ms} ms
                      </span>
                    )}

                    {isUnlocked ? (
                      <div className="flex items-center gap-1 text-xs font-semibold px-2.5 py-1 rounded-xl bg-success/20 border border-success/30 text-success">
                        <CheckCircle2 size={14} />
                        <span>{t('svc_state_ok')}</span>
                      </div>
                    ) : isRestricted ? (
                      <div className="flex items-center gap-1 text-xs font-semibold px-2.5 py-1 rounded-xl bg-warning/20 border border-warning/30 text-warning">
                        <AlertTriangle size={14} />
                        <span>{t('svc_state_restricted')}</span>
                      </div>
                    ) : (
                      <div className="flex items-center gap-1 text-xs font-semibold px-2.5 py-1 rounded-xl bg-destructive/20 border border-destructive/30 text-destructive">
                        <XCircle size={14} />
                        <span>{t('svc_state_blocked')}</span>
                      </div>
                    )}
                  </div>
                </div>
              )
            })
          )}
        </div>

        {/* Footer info */}
        <div className="mt-4 pt-3 border-t border-border flex items-center justify-between text-[11px] text-muted-foreground">
          <div className="flex items-center gap-1.5">
            <Layers size={14} className="text-primary" />
            <span>{t('svc_footer_note')}</span>
          </div>
          <button
            onClick={onClose}
            className="px-4 py-1.5 rounded-xl bg-muted hover:bg-muted/90 text-foreground font-medium transition-colors"
          >
            {t('modal_close')}
          </button>
        </div>
    </ModalShell>
  )
}
