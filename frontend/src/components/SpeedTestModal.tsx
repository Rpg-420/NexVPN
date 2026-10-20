import { X, Play, Square, Gauge, ArrowDown, ArrowUp, Activity, Zap, AlertCircle, CheckCircle2, RotateCcw } from 'lucide-react'
import { t } from '../lib/i18n'
import { useSpeedTest } from '../hooks/useSpeedTest'
import { ModalShell } from '../components/ui/modal-shell'

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
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-lg"
      className=""
    >
        
        {/* Modal Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-border bg-card">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl border border-border bg-muted flex items-center justify-center text-muted-foreground shadow-inner">
              <Gauge className={`w-5 h-5 ${isRunning ? 'animate-pulse' : ''}`} />
            </div>
            <div>
              <h3 className="text-base font-bold text-foreground flex items-center gap-2">
                {t('speedtest_title')}
              </h3>
              <p className="text-xs text-muted-foreground">{t('speedtest_subtitle')}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-muted-foreground hover:text-foreground p-1.5 rounded-lg hover:bg-muted transition"
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
                  <stop offset="0%" stopColor="hsl(var(--primary))" stopOpacity="0.55" />
                  <stop offset="50%" stopColor="hsl(var(--primary))" />
                  <stop offset="100%" stopColor="hsl(var(--primary))" />
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
                className="stroke-card"
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
              <div className="text-[11px] uppercase tracking-wider text-muted-foreground font-semibold mb-1 flex items-center gap-1.5">
                {metrics.state === 'completed' && (
                  <CheckCircle2 className="w-3.5 h-3.5 text-success" />
                )}
                {metrics.state === 'error' && (
                  <AlertCircle className="w-3.5 h-3.5 text-destructive" />
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

              <div className="bg-info text-5xl font-black font-mono text-transparent bg-clip-text tracking-tight">
                {currentSpeed.toFixed(1)}
              </div>

              <div className="text-xs text-muted-foreground font-mono font-bold mt-0.5">Mbps</div>
            </div>

            {/* Overall percentage indicator badge */}
            <div className="absolute bottom-0 text-[10px] font-mono text-muted-foreground bg-muted px-2 py-0.5 rounded-full border border-border">
              {metrics.progress}%
            </div>
          </div>

          {/* Error Message Box */}
          {metrics.state === 'error' && (
            <div className="w-full bg-destructive/10 border border-destructive/30 rounded-xl p-3 text-destructive text-xs flex items-center gap-2 animate-shake">
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
                  ? 'bg-success/15 border-success/50 ring-1 ring-success/30 shadow-sm scale-[1.02]'
                  : 'bg-muted border-border'
              }`}
            >
              <div className="text-[10px] text-muted-foreground flex items-center justify-center gap-1">
                <ArrowDown className="w-3 h-3 text-success" />
                {t('speedtest_download')}
              </div>
              <div className="text-sm font-bold font-mono text-success mt-1">
                {metrics.download_mbps} <span className="text-[9px] text-muted-foreground">Mbps</span>
              </div>
            </div>

            {/* Upload Card */}
            <div
              className={`border rounded-xl p-2.5 text-center transition-all ${
                metrics.phase === 'upload'
                  ? 'bg-info/15 border-primary/50 ring-1 ring-primary/30 shadow-sm scale-[1.02]'
                  : 'bg-muted border-border'
              }`}
            >
              <div className="text-[10px] text-muted-foreground flex items-center justify-center gap-1">
                <ArrowUp className="w-3 h-3 text-primary" />
                {t('speedtest_upload')}
              </div>
              <div className="text-sm font-bold font-mono text-primary mt-1">
                {metrics.upload_mbps} <span className="text-[9px] text-muted-foreground">Mbps</span>
              </div>
            </div>

            {/* Ping Card */}
            <div
              className={`border rounded-xl p-2.5 text-center transition-all ${
                metrics.phase === 'ping'
                  ? 'bg-warning/15 border-warning/50 ring-1 ring-warning/30 shadow-sm scale-[1.02]'
                  : 'bg-muted border-border'
              }`}
            >
              <div className="text-[10px] text-muted-foreground flex items-center justify-center gap-1">
                <Activity className="w-3 h-3 text-warning" />
                {t('speedtest_ping')}
              </div>
              <div className="text-sm font-bold font-mono text-warning mt-1">
                {metrics.ping_ms} <span className="text-[9px] text-muted-foreground">ms</span>
              </div>
            </div>

            {/* Jitter Card */}
            <div className="bg-muted border border-border rounded-xl p-2.5 text-center">
              <div className="text-[10px] text-muted-foreground flex items-center justify-center gap-1">
                <Zap className="w-3 h-3 text-primary" />
                {t('speedtest_jitter')}
              </div>
              <div className="text-sm font-bold font-mono text-primary mt-1">
                {metrics.jitter_ms} <span className="text-[9px] text-muted-foreground">ms</span>
              </div>
            </div>
          </div>

          {/* Action Control Button Area */}
          <div className="w-full flex items-center gap-2">
            {isRunning ? (
              <button
                onClick={stopTest}
                className="flex-1 flex items-center justify-center gap-2 bg-destructive hover:bg-destructive/90 text-destructive-foreground font-bold py-3 rounded-xl transition shadow-sm text-xs active:scale-[0.99]"
              >
                <Square className="w-4 h-4 fill-current" />
                {t('speedtest_stop')}
              </button>
            ) : metrics.state === 'completed' ? (
              <div className="flex-1 flex items-center gap-2">
                <button
                  onClick={reset}
                  className="p-3 bg-muted hover:bg-muted/90 border border-border rounded-xl text-muted-foreground transition"
                  title="Reset"
                >
                  <RotateCcw className="w-4 h-4" />
                </button>
                <button
                  onClick={startTest}
                  className="bg-primary flex-1 flex items-center justify-center gap-2 text-primary-foreground font-bold py-3 rounded-xl transition shadow-sm text-xs active:scale-[0.99]"
                >
                  <Play className="w-4 h-4 fill-current" />
                  {t('speedtest_start')}
                </button>
              </div>
            ) : (
              <button
                onClick={startTest}
                className="bg-primary w-full flex items-center justify-center gap-2 text-primary-foreground font-bold py-3 rounded-xl transition shadow-sm text-xs active:scale-[0.99]"
              >
                <Play className="w-4 h-4 fill-current" />
                {t('speedtest_start')}
              </button>
            )}
          </div>
        </div>
      </ModalShell>
  )
}
