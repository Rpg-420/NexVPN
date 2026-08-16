import { X, Play, Square, Gauge, ArrowDown, ArrowUp, Activity, Zap, AlertCircle, CheckCircle2, RotateCcw } from 'lucide-react'
import { t } from '../lib/i18n'
import { useSpeedTest } from '../hooks/useSpeedTest'

interface Props {
  isOpen: boolean
  onClose: () => void
}

export default function SpeedTestModal({ isOpen, onClose }: Props) {
  const { metrics, isRunning, startTest, stopTest, reset } = useSpeedTest(isOpen)

  if (!isOpen) return null

  // Speed metric to emphasize in center display
  const currentSpeed =
    metrics.phase === 'upload' ? metrics.upload_mbps : metrics.download_mbps

  // Circular gauge SVG calculations (Half-Circle Gauge with 180-degree sweep)
  const radius = 84
  const circumference = Math.PI * radius // semicircle length ~ 263.89
  const strokeDashoffset = circumference - (metrics.progress / 100) * circumference

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
      <div className="bg-slate-900/95 border border-slate-700/80 rounded-2xl w-full max-w-lg overflow-hidden shadow-2xl shadow-cyan-950/40 flex flex-col transition-all">
        
        {/* Modal Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800/90 bg-slate-900/80">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-400 shadow-inner">
              <Gauge className={`w-5 h-5 ${isRunning ? 'animate-pulse' : ''}`} />
            </div>
            <div>
              <h3 className="text-base font-bold text-slate-100 flex items-center gap-2">
                {t('speedtest_title')}
              </h3>
              <p className="text-xs text-slate-400">{t('speedtest_subtitle')}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-200 p-1.5 rounded-lg hover:bg-slate-800 transition"
            aria-label="Close"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Speedometer Center Area */}
        <div className="p-6 space-y-6 flex flex-col items-center">
          
          {/* Radial SVG Semi-Circle Gauge */}
          <div className="relative w-64 h-40 flex items-center justify-center overflow-visible select-none">
            <svg
              className="w-64 h-64 overflow-visible"
              viewBox="0 0 200 120"
            >
              <defs>
                {/* Neon Cyan to Indigo Gradient for Progress */}
                <linearGradient id="gaugeGradient" x1="0%" y1="0%" x2="100%" y2="0%">
                  <stop offset="0%" stopColor="#06b6d4" />
                  <stop offset="50%" stopColor="#3b82f6" />
                  <stop offset="100%" stopColor="#8b5cf6" />
                </linearGradient>

                {/* Glow Filter */}
                <filter id="gaugeGlow" x="-20%" y="-20%" width="140%" height="140%">
                  <feGaussianBlur stdDeviation="3" result="glow" />
                  <feComposite in="SourceGraphic" in2="glow" operator="over" />
                </filter>
              </defs>

              {/* Background Track Arc (180 deg) */}
              <path
                d="M 16 100 A 84 84 0 0 1 184 100"
                fill="none"
                className="stroke-slate-800"
                strokeWidth="12"
                strokeLinecap="round"
              />

              {/* Active Animated Progress Arc */}
              <path
                d="M 16 100 A 84 84 0 0 1 184 100"
                fill="none"
                stroke="url(#gaugeGradient)"
                strokeWidth="12"
                strokeDasharray={circumference}
                strokeDashoffset={strokeDashoffset}
                strokeLinecap="round"
                className="transition-all duration-300 ease-out"
                filter={isRunning ? 'url(#gaugeGlow)' : undefined}
              />
            </svg>

            {/* Gauge Internal Value Display */}
            <div className="absolute inset-0 flex flex-col items-center justify-center top-6">
              <div className="text-[11px] uppercase tracking-wider text-slate-400 font-semibold mb-1 flex items-center gap-1.5">
                {metrics.state === 'completed' && (
                  <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
                )}
                {metrics.state === 'error' && (
                  <AlertCircle className="w-3.5 h-3.5 text-rose-400" />
                )}
                {metrics.phase === 'download'
                  ? t('speedtest_download')
                  : metrics.phase === 'upload'
                  ? t('speedtest_upload')
                  : metrics.phase === 'complete'
                  ? t('speedtest_final')
                  : metrics.phase === 'error'
                  ? t('error')
                  : t('speedtest_testing')}
              </div>

              <div className="text-5xl font-black font-mono text-transparent bg-clip-text bg-gradient-to-r from-cyan-400 via-sky-300 to-blue-400 tracking-tight">
                {currentSpeed.toFixed(1)}
              </div>

              <div className="text-xs text-slate-400 font-mono font-bold mt-0.5">Mbps</div>
            </div>

            {/* Overall percentage indicator badge */}
            <div className="absolute bottom-0 text-[10px] font-mono text-slate-400 bg-slate-800/80 px-2 py-0.5 rounded-full border border-slate-700/50">
              {metrics.progress}%
            </div>
          </div>

          {/* Error Message Box */}
          {metrics.state === 'error' && (
            <div className="w-full bg-rose-500/10 border border-rose-500/30 rounded-xl p-3 text-rose-400 text-xs flex items-center gap-2 animate-shake">
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{metrics.error_msg || t('error')}</span>
            </div>
          )}

          {/* Detailed Metric Cards Grid */}
          <div className="grid grid-cols-4 gap-2.5 w-full">
            {/* Download Card */}
            <div
              className={`border rounded-xl p-2.5 text-center transition-all ${
                metrics.phase === 'download'
                  ? 'bg-emerald-500/15 border-emerald-500/50 ring-1 ring-emerald-500/30 shadow-lg shadow-emerald-500/10 scale-[1.02]'
                  : 'bg-slate-800/40 border-slate-700/50'
              }`}
            >
              <div className="text-[10px] text-slate-400 flex items-center justify-center gap-1">
                <ArrowDown className="w-3 h-3 text-emerald-400" />
                {t('speedtest_download')}
              </div>
              <div className="text-sm font-bold font-mono text-emerald-400 mt-1">
                {metrics.download_mbps} <span className="text-[9px] text-slate-500">Mbps</span>
              </div>
            </div>

            {/* Upload Card */}
            <div
              className={`border rounded-xl p-2.5 text-center transition-all ${
                metrics.phase === 'upload'
                  ? 'bg-blue-500/15 border-blue-500/50 ring-1 ring-blue-500/30 shadow-lg shadow-blue-500/10 scale-[1.02]'
                  : 'bg-slate-800/40 border-slate-700/50'
              }`}
            >
              <div className="text-[10px] text-slate-400 flex items-center justify-center gap-1">
                <ArrowUp className="w-3 h-3 text-blue-400" />
                {t('speedtest_upload')}
              </div>
              <div className="text-sm font-bold font-mono text-blue-400 mt-1">
                {metrics.upload_mbps} <span className="text-[9px] text-slate-500">Mbps</span>
              </div>
            </div>

            {/* Ping Card */}
            <div
              className={`border rounded-xl p-2.5 text-center transition-all ${
                metrics.phase === 'ping'
                  ? 'bg-amber-500/15 border-amber-500/50 ring-1 ring-amber-500/30 shadow-lg shadow-amber-500/10 scale-[1.02]'
                  : 'bg-slate-800/40 border-slate-700/50'
              }`}
            >
              <div className="text-[10px] text-slate-400 flex items-center justify-center gap-1">
                <Activity className="w-3 h-3 text-amber-400" />
                {t('speedtest_ping')}
              </div>
              <div className="text-sm font-bold font-mono text-amber-400 mt-1">
                {metrics.ping_ms} <span className="text-[9px] text-slate-500">ms</span>
              </div>
            </div>

            {/* Jitter Card */}
            <div className="bg-slate-800/40 border border-slate-700/50 rounded-xl p-2.5 text-center">
              <div className="text-[10px] text-slate-400 flex items-center justify-center gap-1">
                <Zap className="w-3 h-3 text-purple-400" />
                {t('speedtest_jitter')}
              </div>
              <div className="text-sm font-bold font-mono text-purple-400 mt-1">
                {metrics.jitter_ms} <span className="text-[9px] text-slate-500">ms</span>
              </div>
            </div>
          </div>

          {/* Action Control Button Area */}
          <div className="w-full flex items-center gap-2">
            {isRunning ? (
              <button
                onClick={stopTest}
                className="flex-1 flex items-center justify-center gap-2 bg-rose-600 hover:bg-rose-500 text-white font-bold py-3 rounded-xl transition shadow-lg shadow-rose-600/25 text-xs active:scale-[0.99]"
              >
                <Square className="w-4 h-4 fill-current" />
                {t('speedtest_stop')}
              </button>
            ) : metrics.state === 'completed' ? (
              <div className="flex-1 flex items-center gap-2">
                <button
                  onClick={reset}
                  className="p-3 bg-slate-800 hover:bg-slate-700 border border-slate-700 rounded-xl text-slate-300 transition"
                  title="Reset"
                >
                  <RotateCcw className="w-4 h-4" />
                </button>
                <button
                  onClick={startTest}
                  className="flex-1 flex items-center justify-center gap-2 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-white font-bold py-3 rounded-xl transition shadow-lg shadow-cyan-500/25 text-xs active:scale-[0.99]"
                >
                  <Play className="w-4 h-4 fill-current" />
                  {t('speedtest_start')}
                </button>
              </div>
            ) : (
              <button
                onClick={startTest}
                className="w-full flex items-center justify-center gap-2 bg-gradient-to-r from-cyan-500 via-blue-600 to-cyan-500 hover:from-cyan-400 hover:to-blue-500 text-white font-bold py-3 rounded-xl transition shadow-lg shadow-cyan-500/25 text-xs active:scale-[0.99]"
              >
                <Play className="w-4 h-4 fill-current" />
                {t('speedtest_start')}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
